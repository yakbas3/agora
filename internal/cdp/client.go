package cdp

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	cdpAPIURL       = "https://api.cdp.coinbase.com/platform/v2/data/query/run"
	usdcBaseAddress = "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913"
	queryLimit      = 10000
)

type Client struct {
	keyID      string
	privateKey *ecdsa.PrivateKey
	httpClient *http.Client
}

func NewClient(keyID, keySecret string) (*Client, error) {
	privKey, err := parseES256Key(keySecret)
	if err != nil {
		return nil, fmt.Errorf("parse CDP key: %w", err)
	}
	return &Client{
		keyID:      keyID,
		privateKey: privKey,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}, nil
}

// QueryTransfers fetches USDC Transfer events for a facilitator in a time range.
func (c *Client) QueryTransfers(facilitatorAddr string, since, until time.Time) ([]Transfer, error) {
	var all []Transfer
	offset := 0

	for {
		sql := buildTransferQuery(facilitatorAddr, since, until, queryLimit, offset)
		rows, err := c.executeQuery(sql)
		if err != nil {
			return nil, err
		}

		transfers, err := parseTransferRows(rows)
		if err != nil {
			return nil, err
		}

		all = append(all, transfers...)

		if len(transfers) < queryLimit {
			break // no more pages
		}
		offset += queryLimit
	}

	return all, nil
}

func buildTransferQuery(facilitator string, since, until time.Time, limit, offset int) string {
	return fmt.Sprintf(`SELECT
  address AS contract_address,
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
		facilitator,
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
		"iat": now.Unix(),
		"exp": now.Add(120 * time.Second).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	return token.SignedString(c.privateKey)
}

func (c *Client) executeQuery(sql string) (*QueryResult, error) {
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
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("CDP API error %d: %s", resp.StatusCode, string(respBody))
	}

	var qr QueryResponse
	if err := json.Unmarshal(respBody, &qr); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &qr.Result, nil
}

func parseTransferRows(result *QueryResult) ([]Transfer, error) {
	if result == nil || len(result.Rows) == 0 {
		return nil, nil
	}

	var transfers []Transfer
	for _, row := range result.Rows {
		if len(row) < 9 {
			continue
		}
		t := Transfer{
			ContractAddress: toString(row[0]),
			Sender:          toString(row[1]),
			TransactionFrom: toString(row[2]),
			ToAddress:       toString(row[3]),
			TransactionHash: toString(row[4]),
			Amount:          toString(row[6]),
		}

		// Parse block_timestamp
		if ts, ok := row[5].(string); ok {
			parsed, err := time.Parse("2006-01-02T15:04:05", ts)
			if err != nil {
				parsed, err = time.Parse("2006-01-02 15:04:05", ts)
			}
			if err == nil {
				t.BlockTimestamp = parsed
			}
		}

		// Parse log_index
		if v, ok := row[7].(float64); ok {
			t.LogIndex = int(v)
		}

		// Parse block_number
		if v, ok := row[8].(float64); ok {
			t.BlockNumber = int64(v)
		}

		transfers = append(transfers, t)
	}
	return transfers, nil
}

// parseES256Key parses a base64 or PEM-encoded ES256 private key.
func parseES256Key(secret string) (*ecdsa.PrivateKey, error) {
	// Try PEM first
	if strings.Contains(secret, "-----BEGIN") {
		block, _ := pem.Decode([]byte(secret))
		if block == nil {
			return nil, fmt.Errorf("failed to decode PEM block")
		}
		key, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			// Try PKCS8
			pk, err2 := x509.ParsePKCS8PrivateKey(block.Bytes)
			if err2 != nil {
				return nil, fmt.Errorf("parse EC key: %w; PKCS8: %w", err, err2)
			}
			ecKey, ok := pk.(*ecdsa.PrivateKey)
			if !ok {
				return nil, fmt.Errorf("PKCS8 key is not ECDSA")
			}
			return ecKey, nil
		}
		return key, nil
	}

	// Try raw base64 (Coinbase gives base64-encoded DER)
	derBytes, err := base64.StdEncoding.DecodeString(secret)
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}

	key, err := x509.ParseECPrivateKey(derBytes)
	if err != nil {
		pk, err2 := x509.ParsePKCS8PrivateKey(derBytes)
		if err2 != nil {
			return nil, fmt.Errorf("parse EC key from base64: %w; PKCS8: %w", err, err2)
		}
		ecKey, ok := pk.(*ecdsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("PKCS8 key is not ECDSA")
		}
		return ecKey, nil
	}
	return key, nil
}

// jsonString marshals a string to a JSON string (handles escaping).
func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func toString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}
