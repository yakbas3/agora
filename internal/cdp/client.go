package cdp

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"math/big"
	mrand "math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	cdpAPIURL       = "https://api.cdp.coinbase.com/platform/v2/data/query/run"
	usdcBaseAddress = "0x833589fcd6edb6e08f4c7c32d4f71b54bda02913" // lowercase
	queryLimit      = 10000
)

type Client struct {
	keyID      string
	privateKey ed25519.PrivateKey
	httpClient *http.Client
}

func NewClient(keyID, keySecret string) (*Client, error) {
	keySecret = strings.TrimSpace(keySecret)
	derBytes, err := base64.StdEncoding.DecodeString(keySecret)
	if err != nil {
		return nil, fmt.Errorf("base64 decode CDP key: %w", err)
	}
	if len(derBytes) != 64 {
		return nil, fmt.Errorf("unexpected CDP key length: %d bytes (expected 64)", len(derBytes))
	}
	return &Client{
		keyID:      keyID,
		privateKey: ed25519.PrivateKey(derBytes),
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}, nil
}

// QueryTransfers fetches USDC Transfer events for a facilitator in a time range.
func (c *Client) QueryTransfers(facilitatorAddr string, since, until time.Time) ([]Transfer, error) {
	var all []Transfer
	offset := 0

	for {
		sql := buildTransferQuery(facilitatorAddr, since, until, queryLimit, offset)
		result, err := c.executeQuery(sql)
		if err != nil {
			return nil, err
		}

		transfers := parseTransferRows(result)
		all = append(all, transfers...)

		if len(transfers) < queryLimit {
			break // no more pages
		}
		offset += queryLimit
	}

	return all, nil
}

func buildTransferQuery(facilitator string, since, until time.Time, limit, offset int) string {
	// CDP SQL API stores all addresses in lowercase
	return fmt.Sprintf(`SELECT
  address,
  parameters['from']::String AS sender,
  transaction_from,
  parameters['to']::String AS to_address,
  transaction_hash,
  block_timestamp,
  parameters['value']::UInt256 AS amount,
  log_index,
  block_number
FROM base.events
WHERE event_signature = 'Transfer(address,address,uint256)'
  AND address = '%s'
  AND lower(transaction_from) = lower('%s')
  AND block_timestamp >= '%s'
  AND block_timestamp < '%s'
ORDER BY block_timestamp DESC
LIMIT %d
OFFSET %d`,
		usdcBaseAddress,
		strings.ToLower(facilitator),
		since.UTC().Format("2006-01-02 15:04:05"),
		until.UTC().Format("2006-01-02 15:04:05"),
		limit,
		offset,
	)
}

func (c *Client) generateJWT() (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub": c.keyID,
		"iss": "cdp",
		"aud": []string{"cdp_service"},
		"nbf": now.Unix(),
		"iat": now.Unix(),
		"exp": now.Add(60 * time.Second).Unix(),
		"uri": "POST api.cdp.coinbase.com/platform/v2/data/query/run",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)

	token.Header["kid"] = c.keyID
	nonceBig, _ := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	token.Header["nonce"] = nonceBig.String()

	return token.SignedString(c.privateKey)
}

func (c *Client) executeQuery(sql string) ([]map[string]interface{}, error) {
	const maxRetries = 8

	for attempt := 0; attempt <= maxRetries; attempt++ {
		jwtToken, err := c.generateJWT()
		if err != nil {
			return nil, fmt.Errorf("generate JWT: %w", err)
		}

		body := fmt.Sprintf(`{"sql": %s}`, jsonString(sql))
		req, err := http.NewRequest("POST", cdpAPIURL, strings.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+jwtToken)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("CDP API request: %w", err)
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("read response: %w", err)
		}

		if resp.StatusCode == 429 && attempt < maxRetries {
			// Exponential backoff with jitter, following x402scan's pattern:
			// base 500ms * 2^attempt + random 0-200ms. Capped at 30s so we
			// don't sleep forever on sustained pressure.
			backoffMs := 500<<attempt + mrand.Intn(200)
			if backoffMs > 30000 {
				backoffMs = 30000
			}
			log.Printf("      CDP 429, retry in %dms (attempt %d/%d)", backoffMs, attempt+1, maxRetries)
			time.Sleep(time.Duration(backoffMs) * time.Millisecond)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("CDP API error %d: %s", resp.StatusCode, string(respBody))
		}

		var qr QueryResponse
		if err := json.Unmarshal(respBody, &qr); err != nil {
			return nil, fmt.Errorf("decode response: %w", err)
		}
		return qr.Result, nil
	}

	return nil, fmt.Errorf("CDP API: max retries exceeded (429 rate limit)")
}

func parseTransferRows(rows []map[string]interface{}) []Transfer {
	if len(rows) == 0 {
		return nil
	}

	var transfers []Transfer
	for _, row := range rows {
		t := Transfer{
			ContractAddress: toString(row["address"]),
			Sender:          toString(row["sender"]),
			TransactionFrom: toString(row["transaction_from"]),
			ToAddress:       toString(row["to_address"]),
			TransactionHash: toString(row["transaction_hash"]),
			Amount:          toString(row["amount"]),
		}

		// Parse block_timestamp
		if ts := toString(row["block_timestamp"]); ts != "" {
			for _, layout := range []string{
				"2006-01-02T15:04:05.000Z",
				"2006-01-02T15:04:05Z",
				"2006-01-02T15:04:05",
				"2006-01-02 15:04:05",
			} {
				if parsed, err := time.Parse(layout, ts); err == nil {
					t.BlockTimestamp = parsed
					break
				}
			}
		}

		// Parse log_index
		if v, ok := row["log_index"].(float64); ok {
			t.LogIndex = int(v)
		} else if s, ok := row["log_index"].(string); ok {
			t.LogIndex, _ = strconv.Atoi(s)
		}

		// Parse block_number
		if v, ok := row["block_number"].(float64); ok {
			t.BlockNumber = int64(v)
		} else if s, ok := row["block_number"].(string); ok {
			t.BlockNumber, _ = strconv.ParseInt(s, 10, 64)
		}

		transfers = append(transfers, t)
	}
	return transfers
}

// jsonString marshals a string to a JSON string (handles escaping).
func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}
