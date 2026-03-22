# x402 Protocol — How It Works

## What Is x402?

x402 is a protocol that lets APIs charge per-request using stablecoins (USDC). When an AI agent or user hits an x402-enabled API endpoint, they get back HTTP `402 Payment Required` with payment details. The client signs a USDC payment, attaches it as a header, and re-sends the request. A **facilitator** submits the payment on-chain and the API serves the response.

## Two Settlement Versions

### V1: EIP-3009 (transferWithAuthorization) — ~95%+ of all x402 payments

The facilitator wallet calls `transferWithAuthorization()` directly on the USDC contract. This emits two events:
- `AuthorizationUsed(address authorizer, bytes32 nonce)` on the USDC contract
- `Transfer(address from, address to, uint256 value)` on the USDC contract

**How to identify V1 x402 payments on-chain:** The `transaction_from` (gas payer / tx submitter) is a known facilitator wallet address, and the event is a USDC Transfer.

### V2: Permit2 Proxy Contracts — barely used as of Feb 2026

Uses dedicated proxy contracts that emit `Settled()` and `SettledWithPermit()` events:
- Exact proxy: `0x4020615294c913F045dc10f0a5cdEbd86c280001`
- Upto proxy: `0x4020633461b2895a48930Ff97eE8fCdE8E520002`
- PermitTransfer proxy: `0x4020CD856C882D5fb903D99CE35316A085Bb0001`

V2 has near-zero on-chain activity. Almost nobody uses it yet.

## Key Actors

- **Client/Agent:** The AI agent or user making API requests
- **Resource Server:** The API endpoint that charges for access
- **Facilitator:** A trusted intermediary that verifies and submits the payment on-chain. The facilitator wallet pays gas. This is the key identifier for V1 payments — if a known facilitator wallet submitted a USDC transfer, it's an x402 payment.

## References

- x402 spec: https://github.com/coinbase/x402
- EIP-3009 exact scheme spec: `github.com/coinbase/x402/blob/main/specs/schemes/exact/scheme_exact_evm.md`
- x402-base-pulse (Substreams indexer): tracks both V1 and V2
- x402.org ecosystem page: https://www.x402.org/ecosystem
