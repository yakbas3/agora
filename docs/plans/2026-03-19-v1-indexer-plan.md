# V1 Transaction Indexer Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Index real x402 V1 payment transactions from Base chain via CDP SQL API, surface them through new API endpoints, and display them on the frontend.

**Architecture:** New `internal/cdp/` package handles JWT auth + CDP SQL queries. New `internal/sync/` package orchestrates per-facilitator polling. Reuses existing `transactions` table and `InsertTransactions` repository method. Frontend gets real facilitator data and a new transactions page.

**Tech Stack:** Go 1.24, ES256 JWT (golang-jwt + crypto/ecdsa), CDP SQL API, PostgreSQL, Next.js/React

---

### Task 1: Add CDP config fields

**Files:**
- Modify: `internal/config/config.go:9-21`
- Modify: `.env.example`

**Step 1: Add CDP fields to Config struct**

In `internal/config/config.go`, add two fields to the `Config` struct after line 20 (`APIPort`):

```go
CDPAPIKeyID     string `envconfig:"CDP_API_KEY_ID"`
CDPAPIKeySecret string `envconfig:"CDP_API_KEY_SECRET"`
```

**Step 2: Verify config loads**

Run: `go build ./cmd/agora`
Expected: Builds successfully

**Step 3: Commit**

```bash
git add internal/config/config.go
git commit -m "feat: add CDP SQL API config fields"
```

---

### Task 2: Create facilitators migration

**Files:**
- Create: `migrations/000009_create_facilitators.up.sql`
- Create: `migrations/000009_create_facilitators.down.sql`

**Step 1: Write up migration**

Create `migrations/000009_create_facilitators.up.sql`:

```sql
CREATE TABLE facilitators (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name            TEXT NOT NULL,
    chain           TEXT NOT NULL,
    address         TEXT NOT NULL,
    last_synced_at  TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_facilitators_chain_address ON facilitators (chain, lower(address));
```

**Step 2: Write down migration**

Create `migrations/000009_create_facilitators.down.sql`:

```sql
DROP TABLE IF EXISTS facilitators;
```

**Step 3: Run migration**

Run: `go build -o agora.exe ./cmd/agora && ./agora.exe migrate`
Expected: "Migrations complete."

**Step 4: Commit**

```bash
git add migrations/000009_*
git commit -m "feat: add facilitators table migration"
```

---

### Task 3: Seed facilitators into database

**Files:**
- Create: `migrations/000010_seed_facilitators.up.sql`
- Create: `migrations/000010_seed_facilitators.down.sql`

**Step 1: Write seed migration**

Create `migrations/000010_seed_facilitators.up.sql` with INSERT statements for all facilitators from `facilitators.md`. Only include Base chain addresses (102 total) since CDP SQL API is Base-only. Include all 26 facilitator names.

The format for each row:

```sql
INSERT INTO facilitators (name, chain, address) VALUES
-- 402104
('402104', 'base', '0x73b2b8df52fbe7c40fe78db52e3dffdd5db5ad07'),
-- AnySpend
('AnySpend', 'base', '0x179761d9eed0f0d1599330cc94b0926e68ae87f1'),
-- AurraCloud
('AurraCloud', 'base', '0x222c4367a2950f3b53af260e111fc3060b0983ff'),
('AurraCloud', 'base', '0xb70c4fe126de09bd292fe3d1e40c6d264ca6a52a'),
('AurraCloud', 'base', '0xd348e724e0ef36291a28dfeccf692399b0e179f8'),
-- CodeNut
('CodeNut', 'base', '0x8d8fa42584a727488eeb0e29405ad794a105bb9b'),
('CodeNut', 'base', '0x87af99356d774312b73018b3b6562e1ae0e018c9'),
('CodeNut', 'base', '0x65058cf664d0d07f68b663b0d4b4f12a5e331a38'),
('CodeNut', 'base', '0x88e13d4c764a6c840ce722a0a3765f55a85b327e'),
-- Coinbase
('Coinbase', 'base', '0xdbdf3d8ed80f84c35d01c6c9f9271761bad90ba6'),
('Coinbase', 'base', '0x9aae2b0d1b9dc55ac9bab9556f9a26cb64995fb9'),
('Coinbase', 'base', '0x3a70788150c7645a21b95b7062ab1784d3cc2104'),
('Coinbase', 'base', '0x708e57b6650a9a741ab39cae1969ea1d2d10eca1'),
('Coinbase', 'base', '0xce82eeec8e98e443ec34fda3c3e999cbe4cb6ac2'),
('Coinbase', 'base', '0x7f6d822467df2a85f792d4508c5722ade96be056'),
('Coinbase', 'base', '0x001ddabba5782ee48842318bd9ff4008647c8d9c'),
('Coinbase', 'base', '0x9c09faa49c4235a09677159ff14f17498ac48738'),
('Coinbase', 'base', '0xcbb10c30a9a72fae9232f41cbbd566a097b4e03a'),
('Coinbase', 'base', '0x9fb2714af0a84816f5c6322884f2907e33946b88'),
('Coinbase', 'base', '0x47d8b3c9717e976f31025089384f23900750a5f4'),
('Coinbase', 'base', '0x94701e1df9ae06642bf6027589b8e05dc7004813'),
('Coinbase', 'base', '0x552300992857834c0ad41c8e1a6934a5e4a2e4ca'),
('Coinbase', 'base', '0xd7469bf02d221968ab9f0c8b9351f55f8668ac4f'),
('Coinbase', 'base', '0x88800e08e20b45c9b1f0480cf759b5bf2f05180c'),
('Coinbase', 'base', '0x6831508455a716f987782a1ab41e204856055cc2'),
('Coinbase', 'base', '0xdc8fbad54bf5151405de488f45acd555517e0958'),
('Coinbase', 'base', '0x91d313853ad458addda56b35a7686e2f38ff3952'),
('Coinbase', 'base', '0xadd5585c776b9b0ea77e9309c1299a40442d820f'),
('Coinbase', 'base', '0x4ffeffa616a1460570d1eb0390e264d45a199e91'),
('Coinbase', 'base', '0x8f5cb67b49555e614892b7233cfddebfb746e531'),
('Coinbase', 'base', '0x67b9ce703d9ce658d7c4ac3c289cea112fe662af'),
('Coinbase', 'base', '0x68a96f41ff1e9f2e7b591a931a4ad224e7c07863'),
('Coinbase', 'base', '0x97acce27d5069544480bde0f04d9f47d7422a016'),
('Coinbase', 'base', '0xa32ccda98ba7529705a059bd2d213da8de10d101'),
-- Corbits
('Corbits', 'base', '0x06f0bfd2c8f36674df5cde852c1eed8025c268c9'),
-- Daydreams
('Daydreams', 'base', '0x279e08f711182c79ba6d09669127a426228a4653'),
-- Dexter
('Dexter', 'base', '0x40272e2eac848ea70db07fd657d799bd309329c4'),
-- Heurist
('Heurist', 'base', '0xb578b7db22581507d62bdbeb85e06acd1be09e11'),
('Heurist', 'base', '0x021cc47adeca6673def958e324ca38023b80a5be'),
('Heurist', 'base', '0x3f61093f61817b29d9556d3b092e67746af8cdfd'),
('Heurist', 'base', '0x290d8b8edcafb25042725cb9e78bcac36b8865f8'),
('Heurist', 'base', '0x612d72dc8402bba997c61aa82ce718ea23b2df5d'),
('Heurist', 'base', '0x1fc230ee3c13d0d520d49360a967dbd1555c8326'),
('Heurist', 'base', '0x48ab4b0af4ddc2f666a3fcc43666c793889787a3'),
('Heurist', 'base', '0xd97c12726dcf994797c981d31cfb243d231189fb'),
('Heurist', 'base', '0x90d5e567017f6c696f1916f4365dd79985fce50f'),
-- Meridian
('Meridian', 'base', '0x8e7769d440b3460b92159dd9c6d17302b036e2d6'),
('Meridian', 'base', '0x3210d7b21bfe1083c9dddbe17e8f947c9029a584'),
-- Mogami
('Mogami', 'base', '0xfe0920a0a7f0f8a1ec689146c30c3bbef439bf8a'),
-- OpenFacilitator
('OpenFacilitator', 'base', '0x7c766f5fd9ab3dc09acad5ecfacc99c4781efe29'),
-- Openmid
('Openmid', 'base', '0x16e47d275198ed65916a560bab4af6330c36ae09'),
-- OpenX402
('OpenX402', 'base', '0x97316fa4730bc7d3b295234f8e4d04a0a4c093e8'),
('OpenX402', 'base', '0x97db9b5291a218fc77198c285cefdc943ef74917'),
-- PayAI
('PayAI', 'base', '0xc6699d2aada6c36dfea5c248dd70f9cb0235cb63'),
('PayAI', 'base', '0xb2bd29925cbbcea7628279c91945ca5b98bf371b'),
('PayAI', 'base', '0x25659315106580ce2a787ceec5efb2d347b539c9'),
('PayAI', 'base', '0xb8f41cb13b1f213da1e94e1b742ec1323235c48f'),
('PayAI', 'base', '0xe575fa51af90957d66fab6d63355f1ed021b887b'),
('PayAI', 'base', '0x03a3f7ce8e21e6f8d9fa14c67d8876b2470dc2f1'),
('PayAI', 'base', '0x675707bc7d03089f820c1b7d49f7480083e8f4df'),
('PayAI', 'base', '0xf46833d4ac4f0f1405cc05c30edfd86770f721c9'),
('PayAI', 'base', '0x2daaef6f941de214bf7d6daf322bc6bc7406accb'),
('PayAI', 'base', '0x2fae4026a31f19183947f0a6909ef975ebfa9ca8'),
('PayAI', 'base', '0xe299c486066739c4a31609e1268d93229632dd47'),
('PayAI', 'base', '0x6ccf245c883f9f3c6caee0687aa61daf7bc96e32'),
('PayAI', 'base', '0xaf990eef9846b63d896056050fdc0b28bca9c24b'),
('PayAI', 'base', '0x489c40fc3c2a19ad8cb275b7dd6aa194e9219c4f'),
('PayAI', 'base', '0x9df61a719ddae27c20a63a417271cc2c704654bd'),
-- Polymer
('Polymer', 'base', '0x66c40946b0dffd04be467e18309857307ecd37cb'),
-- Primer
('Primer', 'base', '0x37dfb4033d5dd98fd335f24d0d42e8fe68d587d6'),
-- Questflow
('Questflow', 'base', '0x724efafb051f17ae824afcdf3c0368ae312da264'),
('Questflow', 'base', '0xa9a54ef09fc8b86bc747cec6ef8d6e81c38c6180'),
('Questflow', 'base', '0x4638bc811c93bf5e60deed32325e93505f681576'),
('Questflow', 'base', '0xd7d91a42dfadd906c5b9ccde7226d28251e4cd0f'),
('Questflow', 'base', '0x4544b535938b67d2a410a98a7e3b0f8f68921ca7'),
('Questflow', 'base', '0x59e8014a3b884392fbb679fe461da07b18c1ff81'),
('Questflow', 'base', '0xe6123e6b389751c5f7e9349f3d626b105c1fe618'),
('Questflow', 'base', '0xf70e7cb30b132fab2a0a5e80d41861aa133ea21b'),
('Questflow', 'base', '0x90da501fdbec74bb0549100967eb221fed79c99b'),
('Questflow', 'base', '0xce7819f0b0b871733c933d1f486533bab95ec47b'),
-- RelAI
('RelAI', 'base', '0x1892f72fdb3a966b2ad8595aa5f7741ef72d6085'),
-- Thirdweb
('Thirdweb', 'base', '0x80c08de1a05df2bd633cf520754e40fde3c794d3'),
('Thirdweb', 'base', '0xaaca1ba9d2627cbc0739ba69890c30f95de046e4'),
('Thirdweb', 'base', '0xa1822b21202a24669eaf9277723d180cd6dae874'),
('Thirdweb', 'base', '0xec10243b54df1a71254f58873b389b7ecece89c2'),
('Thirdweb', 'base', '0x052aaae3cad5c095850246f8ffb228354c56752a'),
('Thirdweb', 'base', '0x91ddea05f741b34b63a7548338c90fc152c8631f'),
('Thirdweb', 'base', '0xea52f2c6f6287f554f9b54c5417e1e431fe5710e'),
('Thirdweb', 'base', '0x3a5ca1c6aa6576ae9c1c0e7fa2b4883346bc5aa0'),
('Thirdweb', 'base', '0x7e20b62bf36554b704774afb0fcc0ae8f899213b'),
('Thirdweb', 'base', '0xd88a9a58806b895ff06744082c6a20b9d7184b0f'),
-- Treasure
('Treasure', 'base', '0xe07e9cbf9a55d02e3ac356ed4706353d98c5a618'),
-- Ultravioleta DAO
('Ultravioleta DAO', 'base', '0x103040545ac5031a11e8c03dd11324c7333a13c7'),
-- Virtuals Protocol
('Virtuals Protocol', 'base', '0x80735b3f7808e2e229ace880dbe85e80115631ca'),
-- x402 Jobs
('x402 Jobs', 'base', '0x51fec16843e49b99aaf9814e525aee1756e66a62'),
-- X402rs
('X402rs', 'base', '0xd8dfc729cbd05381647eb5540d756f4f8ad63eec'),
('X402rs', 'base', '0x76eee8f0acabd6b49f1cc4e9656a0c8892f3332e'),
('X402rs', 'base', '0x97d38aa5de015245dcca76305b53abe6da25f6a5'),
('X402rs', 'base', '0x0168f80e035ea68b191faf9bfc12778c87d92008'),
('X402rs', 'base', '0x5e437bee4321db862ac57085ea5eb97199c0ccc5'),
('X402rs', 'base', '0xc19829b32324f116ee7f80d193f99e445968499a'),
-- xEcho
('xEcho', 'base', '0x3be45f576696a2fd5a93c1330cd19f1607ab311d')
;
```

**Step 2: Write down migration**

Create `migrations/000010_seed_facilitators.down.sql`:

```sql
DELETE FROM facilitators;
```

**Step 3: Run migration**

Run: `./agora.exe migrate`
Expected: "Migrations complete."

**Step 4: Verify seed data**

Run: `docker exec $(docker ps -q --filter ancestor=pgvector/pgvector:pg16) psql -U agora -d agora -c "SELECT name, count(*) FROM facilitators GROUP BY name ORDER BY count(*) DESC;"`
Expected: 26 facilitator names, 102 total rows

**Step 5: Commit**

```bash
git add migrations/000010_*
git commit -m "feat: seed 102 Base facilitator addresses from x402scan registry"
```

---

### Task 4: Create Facilitator model

**Files:**
- Create: `internal/models/facilitator.go`

**Step 1: Write model**

```go
package models

import (
	"time"

	"github.com/google/uuid"
)

type Facilitator struct {
	ID           uuid.UUID  `db:"id"            json:"id"`
	Name         string     `db:"name"          json:"name"`
	Chain        string     `db:"chain"         json:"chain"`
	Address      string     `db:"address"       json:"address"`
	LastSyncedAt *time.Time `db:"last_synced_at" json:"last_synced_at"`
	CreatedAt    time.Time  `db:"created_at"    json:"created_at"`
}
```

**Step 2: Verify build**

Run: `go build ./...`
Expected: Builds successfully

**Step 3: Commit**

```bash
git add internal/models/facilitator.go
git commit -m "feat: add Facilitator model"
```

---

### Task 5: Add facilitator repository methods

**Files:**
- Modify: `internal/database/repository.go`

**Step 1: Add GetBaseFacilitators method**

Append to `repository.go`:

```go
// GetBaseFacilitators returns all facilitators on the Base chain.
func (r *Repository) GetBaseFacilitators(ctx context.Context) ([]models.Facilitator, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, chain, address, last_synced_at, created_at
		 FROM facilitators WHERE chain = 'base'
		 ORDER BY name, address`)
	if err != nil {
		return nil, fmt.Errorf("get base facilitators: %w", err)
	}
	defer rows.Close()

	var out []models.Facilitator
	for rows.Next() {
		var f models.Facilitator
		if err := rows.Scan(&f.ID, &f.Name, &f.Chain, &f.Address, &f.LastSyncedAt, &f.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan facilitator: %w", err)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// UpdateFacilitatorSyncTime sets last_synced_at for a facilitator.
func (r *Repository) UpdateFacilitatorSyncTime(ctx context.Context, id uuid.UUID, syncedAt time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE facilitators SET last_synced_at = $2 WHERE id = $1`,
		id, syncedAt)
	if err != nil {
		return fmt.Errorf("update facilitator sync time: %w", err)
	}
	return nil
}
```

**Step 2: Verify build**

Run: `go build ./...`
Expected: Builds successfully

**Step 3: Commit**

```bash
git add internal/database/repository.go
git commit -m "feat: add facilitator repository methods"
```

---

### Task 6: Build CDP SQL API client

**Files:**
- Create: `internal/cdp/client.go`
- Create: `internal/cdp/types.go`

**Step 1: Install golang-jwt dependency**

Run: `go get github.com/golang-jwt/jwt/v5`

**Step 2: Create types.go**

```go
package cdp

import "time"

// Transfer represents a single USDC transfer from the CDP SQL API.
type Transfer struct {
	ContractAddress string    `json:"contract_address"`
	Sender          string    `json:"sender"`
	TransactionFrom string   `json:"transaction_from"`
	ToAddress       string    `json:"to_address"`
	TransactionHash string   `json:"transaction_hash"`
	BlockTimestamp  time.Time `json:"block_timestamp"`
	Amount          string    `json:"amount"`
	LogIndex        int       `json:"log_index"`
	BlockNumber     int64     `json:"block_number"`
}

// QueryResponse is the CDP SQL API response envelope.
type QueryResponse struct {
	Result QueryResult `json:"result"`
}

type QueryResult struct {
	Columns []Column        `json:"columns"`
	Rows    [][]interface{} `json:"rows"`
	Status  string          `json:"status"`
}

type Column struct {
	Name string `json:"name"`
	Type string `json:"type"`
}
```

**Step 3: Create client.go**

```go
package cdp

import (
	"crypto/ecdsa"
	"crypto/x509"
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
	import "encoding/base64"
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
```

Note: The `import "encoding/base64"` in `parseES256Key` needs to be moved to the top-level import block. The implementation agent should fix this.

**Step 4: Verify build**

Run: `go build ./...`
Expected: Builds successfully

**Step 5: Commit**

```bash
git add internal/cdp/
git commit -m "feat: add CDP SQL API client with JWT auth"
```

---

### Task 7: Build sync runner

**Files:**
- Create: `internal/sync/runner.go`

**Step 1: Write runner**

```go
package sync

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/yamanakbas/agora/internal/cdp"
	"github.com/yamanakbas/agora/internal/database"
	"github.com/yamanakbas/agora/internal/models"
)

const (
	// Default start date for facilitators that have never been synced.
	defaultStartDate = "2024-06-01"
	// USDC has 6 decimals.
	usdcDecimals = 6
	// USDC contract address on Base.
	usdcBaseAddress = "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913"
)

type Runner struct {
	cdpClient *cdp.Client
	repo      *database.Repository
}

func NewRunner(cdpClient *cdp.Client, repo *database.Repository) *Runner {
	return &Runner{cdpClient: cdpClient, repo: repo}
}

func (r *Runner) Run(ctx context.Context) error {
	facilitators, err := r.repo.GetBaseFacilitators(ctx)
	if err != nil {
		return fmt.Errorf("load facilitators: %w", err)
	}
	log.Printf("Syncing %d Base facilitators", len(facilitators))

	now := time.Now().UTC()
	totalInserted := 0

	for _, f := range facilitators {
		inserted, err := r.syncFacilitator(ctx, f, now)
		if err != nil {
			log.Printf("ERROR syncing %s (%s): %v", f.Name, f.Address, err)
			continue
		}
		totalInserted += inserted
		if inserted > 0 {
			log.Printf("  %s (%s): %d new transactions", f.Name, f.Address[:10]+"...", inserted)
		}
	}

	log.Printf("Sync complete: %d new transactions total", totalInserted)

	// Refresh materialized views
	if totalInserted > 0 {
		log.Println("Refreshing endpoint_scores...")
		if err := r.repo.RefreshEndpointScores(ctx); err != nil {
			log.Printf("WARNING: refresh endpoint_scores: %v", err)
		}
		log.Println("Refreshing discovered_sellers...")
		if err := r.repo.RefreshDiscoveredSellers(ctx); err != nil {
			log.Printf("WARNING: refresh discovered_sellers: %v", err)
		}
	}

	return nil
}

func (r *Runner) syncFacilitator(ctx context.Context, f models.Facilitator, now time.Time) (int, error) {
	since := defaultSinceTime(f.LastSyncedAt)

	transfers, err := r.cdpClient.QueryTransfers(f.Address, since, now)
	if err != nil {
		return 0, fmt.Errorf("query transfers: %w", err)
	}

	if len(transfers) == 0 {
		// Update sync time even if no transfers, so we don't re-query the same window
		if err := r.repo.UpdateFacilitatorSyncTime(ctx, f.ID, now); err != nil {
			return 0, fmt.Errorf("update sync time: %w", err)
		}
		return 0, nil
	}

	// Convert CDP transfers to model transactions
	txs := make([]models.Transaction, 0, len(transfers))
	for _, t := range transfers {
		txs = append(txs, models.Transaction{
			ID:                 uuid.New(),
			TxHash:             t.TransactionHash,
			BlockNumber:        t.BlockNumber,
			BlockTime:          t.BlockTimestamp,
			EventType:          "Transfer",
			ProxyContract:      "",
			FacilitatorAddress: f.Address,
			PayerAddress:       t.Sender,
			RecipientAddress:   t.ToAddress,
			AmountRaw:          t.Amount,
			AmountUSD:          usdcToFloat(t.Amount),
			AssetAddress:       usdcBaseAddress,
			IndexedAt:          now,
		})
	}

	inserted, err := r.repo.InsertTransactions(ctx, txs)
	if err != nil {
		return 0, fmt.Errorf("insert transactions: %w", err)
	}

	if err := r.repo.UpdateFacilitatorSyncTime(ctx, f.ID, now); err != nil {
		return inserted, fmt.Errorf("update sync time: %w", err)
	}

	return inserted, nil
}

func defaultSinceTime(lastSynced *time.Time) time.Time {
	if lastSynced != nil {
		return *lastSynced
	}
	t, _ := time.Parse("2006-01-02", defaultStartDate)
	return t
}

// usdcToFloat converts a raw USDC amount string (6 decimals) to a float64 USD value.
func usdcToFloat(raw string) float64 {
	amount, ok := new(big.Int).SetString(raw, 10)
	if !ok {
		return 0
	}
	f := new(big.Float).SetInt(amount)
	divisor := new(big.Float).SetFloat64(1e6)
	result, _ := new(big.Float).Quo(f, divisor).Float64()
	return result
}
```

**Step 2: Verify build**

Run: `go build ./...`
Expected: Builds successfully

**Step 3: Commit**

```bash
git add internal/sync/
git commit -m "feat: add sync runner for V1 transaction indexing"
```

---

### Task 8: Add `sync` CLI command

**Files:**
- Modify: `cmd/agora/main.go`

**Step 1: Add sync case to main switch and import**

Add `"github.com/yamanakbas/agora/internal/cdp"` and `"github.com/yamanakbas/agora/internal/sync"` to imports.

Add case to the switch statement (after `"index"` case):

```go
case "sync":
	runSync(cfg)
```

Add to `printUsage()`:

```go
fmt.Fprintln(os.Stderr, "  sync      Sync V1 transactions from CDP SQL API")
```

**Step 2: Add runSync function**

```go
func runSync(cfg *config.Config) {
	if cfg.CDPAPIKeyID == "" || cfg.CDPAPIKeySecret == "" {
		log.Fatal("CDP_API_KEY_ID and CDP_API_KEY_SECRET are required for syncing. Set them in .env or environment.")
	}

	ctx := context.Background()

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	cdpClient, err := cdp.NewClient(cfg.CDPAPIKeyID, cfg.CDPAPIKeySecret)
	if err != nil {
		log.Fatalf("Failed to create CDP client: %v", err)
	}

	repo := database.NewRepository(pool)
	runner := sync.NewRunner(cdpClient, repo)

	log.Println("Starting V1 transaction sync...")
	if err := runner.Run(ctx); err != nil {
		log.Fatalf("Sync failed: %v", err)
	}
	log.Println("Done.")
}
```

**Step 3: Verify build**

Run: `go build -o agora.exe ./cmd/agora`
Expected: Builds successfully

**Step 4: Commit**

```bash
git add cmd/agora/main.go
git commit -m "feat: add sync CLI command for V1 transaction indexing"
```

---

### Task 9: Test the sync end-to-end

**Step 1: Ensure database is running**

Run: `docker compose up -d`

**Step 2: Run migrations**

Run: `./agora.exe migrate`

**Step 3: Run sync**

Run: `./agora.exe sync`
Expected: Logs showing facilitator-by-facilitator sync progress, some number of new transactions inserted. May take a few minutes depending on how many facilitators have activity.

**Step 4: Verify transactions in database**

Run: `docker exec $(docker ps -q --filter ancestor=pgvector/pgvector:pg16) psql -U agora -d agora -c "SELECT count(*) FROM transactions;"`
Expected: Non-zero count

Run: `docker exec $(docker ps -q --filter ancestor=pgvector/pgvector:pg16) psql -U agora -d agora -c "SELECT facilitator_address, count(*) FROM transactions GROUP BY facilitator_address ORDER BY count(*) DESC LIMIT 10;"`
Expected: Breakdown by facilitator

**Step 5: Commit any fixes needed, then tag**

```bash
git add -A
git commit -m "feat: V1 indexer working end-to-end"
```

---

### Task 10: Add facilitator and transaction API endpoints

**Files:**
- Modify: `internal/database/repository.go` (add query methods)
- Modify: `internal/api/handlers.go` (add handler functions)
- Modify: `internal/api/server.go` (register routes)

**Step 1: Add repository methods**

Append to `internal/database/repository.go`:

```go
// FacilitatorStats holds a facilitator with aggregated transaction stats.
type FacilitatorStats struct {
	models.Facilitator
	TxCount      int     `json:"tx_count"`
	TotalVolume  float64 `json:"total_volume_usd"`
	UniquePayers int     `json:"unique_payers"`
}

// GetFacilitatorStats returns all facilitators with their transaction stats.
func (r *Repository) GetFacilitatorStats(ctx context.Context) ([]FacilitatorStats, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT f.id, f.name, f.chain, f.address, f.last_synced_at, f.created_at,
		       COALESCE(s.tx_count, 0),
		       COALESCE(s.total_volume, 0),
		       COALESCE(s.unique_payers, 0)
		FROM facilitators f
		LEFT JOIN (
			SELECT facilitator_address,
			       COUNT(*)::int AS tx_count,
			       SUM(amount_usd) AS total_volume,
			       COUNT(DISTINCT payer_address)::int AS unique_payers
			FROM transactions
			GROUP BY facilitator_address
		) s ON lower(f.address) = lower(s.facilitator_address)
		WHERE f.chain = 'base'
		ORDER BY COALESCE(s.tx_count, 0) DESC, f.name`)
	if err != nil {
		return nil, fmt.Errorf("get facilitator stats: %w", err)
	}
	defer rows.Close()

	var out []FacilitatorStats
	for rows.Next() {
		var fs FacilitatorStats
		if err := rows.Scan(&fs.ID, &fs.Name, &fs.Chain, &fs.Address,
			&fs.LastSyncedAt, &fs.CreatedAt,
			&fs.TxCount, &fs.TotalVolume, &fs.UniquePayers); err != nil {
			return nil, fmt.Errorf("scan facilitator stats: %w", err)
		}
		out = append(out, fs)
	}
	return out, rows.Err()
}

// TransactionWithFacilitator holds a transaction with its facilitator name.
type TransactionWithFacilitator struct {
	models.Transaction
	FacilitatorName string `json:"facilitator_name"`
}

// GetTransactions returns a paginated list of transactions with facilitator names.
func (r *Repository) GetTransactions(ctx context.Context, limit, offset int, facilitatorFilter string) ([]TransactionWithFacilitator, int, error) {
	// Count total
	countQuery := `SELECT COUNT(*) FROM transactions t`
	countArgs := []any{}
	if facilitatorFilter != "" {
		countQuery += ` JOIN facilitators f ON lower(t.facilitator_address) = lower(f.address) AND f.chain = 'base' WHERE lower(f.name) = lower($1)`
		countArgs = append(countArgs, facilitatorFilter)
	}
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count transactions: %w", err)
	}

	// Fetch page
	query := `
		SELECT t.id, t.tx_hash, t.block_number, t.block_time, t.event_type,
		       t.proxy_contract, t.facilitator_address, t.payer_address,
		       t.recipient_address, t.amount_raw, t.amount_usd, t.asset_address,
		       t.indexed_at, COALESCE(f.name, 'Unknown')
		FROM transactions t
		LEFT JOIN facilitators f ON lower(t.facilitator_address) = lower(f.address) AND f.chain = 'base'`
	args := []any{}
	argIdx := 1

	if facilitatorFilter != "" {
		query += fmt.Sprintf(` WHERE lower(f.name) = lower($%d)`, argIdx)
		args = append(args, facilitatorFilter)
		argIdx++
	}

	query += fmt.Sprintf(` ORDER BY t.block_time DESC LIMIT $%d OFFSET $%d`, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("get transactions: %w", err)
	}
	defer rows.Close()

	var out []TransactionWithFacilitator
	for rows.Next() {
		var tw TransactionWithFacilitator
		if err := rows.Scan(&tw.ID, &tw.TxHash, &tw.BlockNumber, &tw.BlockTime,
			&tw.EventType, &tw.ProxyContract, &tw.FacilitatorAddress,
			&tw.PayerAddress, &tw.RecipientAddress, &tw.AmountRaw,
			&tw.AmountUSD, &tw.AssetAddress, &tw.IndexedAt,
			&tw.FacilitatorName); err != nil {
			return nil, 0, fmt.Errorf("scan transaction: %w", err)
		}
		out = append(out, tw)
	}
	return out, total, rows.Err()
}
```

**Step 2: Add handler functions**

Append to `internal/api/handlers.go`:

```go
func (h *Handlers) handleFacilitators(w http.ResponseWriter, r *http.Request) {
	stats, err := h.repo.GetFacilitatorStats(r.Context())
	if err != nil {
		log.Printf("get facilitator stats error: %v", err)
		http.Error(w, "failed to get facilitator stats", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (h *Handlers) handleTransactions(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	facilitator := r.URL.Query().Get("facilitator")
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	txs, total, err := h.repo.GetTransactions(r.Context(), limit, offset, facilitator)
	if err != nil {
		log.Printf("get transactions error: %v", err)
		http.Error(w, "failed to get transactions", http.StatusInternalServerError)
		return
	}

	resp := map[string]any{
		"transactions": txs,
		"total":        total,
		"limit":        limit,
		"offset":       offset,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
```

**Step 3: Register routes**

In `internal/api/server.go`, add to the `Start()` method after the existing routes:

```go
mux.HandleFunc("GET /api/facilitators", s.handlers.handleFacilitators)
mux.HandleFunc("GET /api/transactions", s.handlers.handleTransactions)
```

**Step 4: Verify build**

Run: `go build ./...`
Expected: Builds successfully

**Step 5: Smoke test API**

Run: `./agora.exe serve &` (in background), then:
Run: `curl http://localhost:8080/api/facilitators | head -c 500`
Run: `curl "http://localhost:8080/api/transactions?limit=5" | head -c 500`
Expected: JSON responses with facilitator stats and transactions

**Step 6: Commit**

```bash
git add internal/api/ internal/database/repository.go
git commit -m "feat: add facilitators and transactions API endpoints"
```

---

### Task 11: Enhance stats endpoint with transaction data

**Files:**
- Modify: `internal/database/repository.go` (GetStats method)

**Step 1: Add transaction stats to StatsResult**

Add fields to `StatsResult` struct:

```go
TotalTransactions int       `json:"total_transactions"`
TotalVolumeUSD    float64   `json:"total_volume_usd"`
TransactionsOverTime []DateCount `json:"transactions_over_time"`
```

**Step 2: Add queries to GetStats method**

At the end of `GetStats` (before `return s, nil`), add:

```go
// Transaction totals
r.pool.QueryRow(ctx,
	`SELECT COALESCE(COUNT(*), 0), COALESCE(SUM(amount_usd), 0) FROM transactions`,
).Scan(&s.TotalTransactions, &s.TotalVolumeUSD)

// Transactions over time (daily, last 30 days)
rows6, err := r.pool.Query(ctx,
	`SELECT DATE(block_time)::text AS d, COUNT(*)
	 FROM transactions
	 WHERE block_time >= NOW() - INTERVAL '30 days'
	 GROUP BY d ORDER BY d`)
if err == nil {
	defer rows6.Close()
	for rows6.Next() {
		var dc DateCount
		if err := rows6.Scan(&dc.Date, &dc.Count); err == nil {
			s.TransactionsOverTime = append(s.TransactionsOverTime, dc)
		}
	}
}
```

**Step 3: Verify build and test**

Run: `go build ./...`
Run: `curl http://localhost:8080/api/stats | python -m json.tool | grep total_transactions`
Expected: Shows transaction count

**Step 4: Commit**

```bash
git add internal/database/repository.go
git commit -m "feat: add transaction stats to stats endpoint"
```

---

### Task 12: Add frontend API functions for facilitators and transactions

**Files:**
- Modify: `web/src/lib/api.ts`
- Modify: `web/src/lib/types.ts`

**Step 1: Add types**

In `web/src/lib/types.ts`, add:

```typescript
export interface FacilitatorStats {
  id: string;
  name: string;
  chain: string;
  address: string;
  last_synced_at: string | null;
  tx_count: number;
  total_volume_usd: number;
  unique_payers: number;
}

export interface Transaction {
  id: string;
  tx_hash: string;
  block_number: number;
  block_time: string;
  event_type: string;
  facilitator_address: string;
  payer_address: string;
  recipient_address: string;
  amount_raw: string;
  amount_usd: number;
  asset_address: string;
  indexed_at: string;
  facilitator_name: string;
}

export interface TransactionsResponse {
  transactions: Transaction[];
  total: number;
  limit: number;
  offset: number;
}
```

**Step 2: Add API functions**

In `web/src/lib/api.ts`, add:

```typescript
export async function fetchFacilitators(): Promise<FacilitatorStats[]> {
  const res = await fetch(`${API_URL}/api/facilitators`);
  if (!res.ok) throw new Error(`Failed to fetch facilitators: ${res.status}`);
  return res.json();
}

export async function fetchTransactions(
  limit = 50,
  offset = 0,
  facilitator?: string
): Promise<TransactionsResponse> {
  let url = `${API_URL}/api/transactions?limit=${limit}&offset=${offset}`;
  if (facilitator) url += `&facilitator=${encodeURIComponent(facilitator)}`;
  const res = await fetch(url);
  if (!res.ok) throw new Error(`Failed to fetch transactions: ${res.status}`);
  return res.json();
}
```

**Step 3: Commit**

```bash
git add web/src/lib/api.ts web/src/lib/types.ts
git commit -m "feat: add frontend API functions for facilitators and transactions"
```

---

### Task 13: Update facilitators page with real data

**Files:**
- Modify: `web/src/app/facilitators/page.tsx`
- Modify: `web/src/components/facilitator-card.tsx`

**Step 1: Rewrite facilitators page**

Replace `web/src/app/facilitators/page.tsx` to fetch from API instead of dummy data. Make it a client component that calls `fetchFacilitators()` and renders cards grouped by facilitator name (since one facilitator can have multiple addresses).

The page should:
- Group addresses by facilitator name
- Show total tx count, volume, and unique payers per facilitator
- Sort by total volume descending

**Step 2: Update FacilitatorCard component**

Update `web/src/components/facilitator-card.tsx` to accept the new `FacilitatorStats` shape instead of the old `Facilitator` type. Show: name, address count, tx count, total volume USD, unique payers.

**Step 3: Verify frontend**

Run: `cd web && npm run dev`
Navigate to http://localhost:3000/facilitators
Expected: Real facilitator data displayed

**Step 4: Commit**

```bash
git add web/src/app/facilitators/page.tsx web/src/components/facilitator-card.tsx
git commit -m "feat: replace dummy facilitator data with real API data"
```

---

### Task 14: Create transactions page

**Files:**
- Create: `web/src/app/transactions/page.tsx`

**Step 1: Create page**

Build a client component that:
- Fetches from `fetchTransactions()`
- Displays a table with columns: Tx Hash (truncated, linked to `https://basescan.org/tx/{hash}`), Facilitator, From (truncated), To (truncated), Amount USD, Time (relative)
- Facilitator filter dropdown (populated from `fetchFacilitators()`)
- Pagination controls (prev/next)

**Step 2: Verify frontend**

Navigate to http://localhost:3000/transactions
Expected: Transaction table with real data

**Step 3: Commit**

```bash
git add web/src/app/transactions/
git commit -m "feat: add transactions page with filtering and pagination"
```

---

### Task 15: Add Transactions link to nav

**Files:**
- Modify: `web/src/components/nav.tsx`

**Step 1: Add link**

Add to the `links` array in `nav.tsx`:

```typescript
{ href: "/transactions", label: "Transactions" },
```

Place it after Facilitators and before Network.

**Step 2: Verify nav**

Navigate to http://localhost:3000
Expected: "Transactions" link appears in nav, clicking it goes to /transactions

**Step 3: Commit**

```bash
git add web/src/components/nav.tsx
git commit -m "feat: add Transactions link to navigation"
```

---

### Task 16: Update seed data dump

After all sync data is in the database, export a new seed dump:

**Step 1: Export updated seed**

Run: `docker exec $(docker ps -q --filter ancestor=pgvector/pgvector:pg16) pg_dump -U agora -d agora --no-owner --no-privileges | gzip > data/seed.sql.gz`

**Step 2: Commit**

```bash
git add data/seed.sql.gz
git commit -m "data: update seed dump with facilitators and transactions"
```

---

### Task 17: Final verification

**Step 1: Run Go tests**

Run: `go test ./...`
Expected: All tests pass

**Step 2: Run full stack**

Run: `docker compose -f docker-compose.prod.yml up --build`
Expected: All 4 services start, frontend shows real facilitator and transaction data

**Step 3: Verify all pages**

- http://localhost:3000 — Endpoints page works
- http://localhost:3000/facilitators — Shows real facilitator stats
- http://localhost:3000/transactions — Shows real transaction data
- http://localhost:3000/network — Network stats include transaction totals

**Step 4: Final commit**

```bash
git add -A
git commit -m "feat: V1 transaction indexer complete with frontend integration"
```
