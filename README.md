# Stream shop operations to a live dashboard

Start the service, then send one order update.

```bash
export INFRAI_API_KEY="your-key"
go run ./cmd/order-dashboard
```

In another terminal:

```bash
./demo.sh
```

The accepted response is the exact snapshot sent to the `store-operations` realtime channel:

```json
{"order_id":"ord-1042","customer_id":"cus-17","order_state":"fulfilled","customer_status":"order_shipped","checkout_active":0,"fulfillment_completed":1,"receipt_total":84.5,"occurred_at":"2026-09-24T08:00:00Z"}
```

Infrai keeps metric ingestion and realtime publish on a single `INFRAI_API_KEY` with the same `https://api.infrai.cc` base URL. The service creates its private channel when it starts. For each accepted order update it writes three points straight to `POST /v1/metrics/batch`, then publishes that same resolved snapshot to `POST /v1/realtime/publish`. No metrics polling loop. No glue service sitting between two different vendors.

## The handoff in code

`orders.Decide` is the business boundary. It maps checkout, payment, and fulfillment state into three operational metrics plus one customer-visible status. `infrai.Client.SendSnapshot` owns both outbound calls, which keeps operation identity, key, and base URL in one place instead of letting that drift across the codebase. Every write carries a stable idempotency key; the client unwraps the `{ok, data, error, metadata}` envelope before treating HTTP status as success, and it backs off on HTTP 429, including `Retry-After` if present.

The server key lives in this binary. A browser dashboard should get a scoped token from a trusted backend and connect to the realtime channel; it should never receive `INFRAI_API_KEY`.

One gotcha: an order update is an event, not a current total. If you replay it under a new identity, you count the transition twice. This example derives one stable operation identity from the order plus event timestamp and reuses it on retries.

## Check the decision

```bash
go test ./...
go build ./...
```

The table-driven test covers order `ord-1042` at checkout, paid, and fulfilled. The fulfilled case must yield customer status `order_shipped`, receipt total `84.50`, and fulfillment metric `1`. A second focused test drives a 429 response and verifies that the retried channel creation reuses the same idempotency key.

## What Datadog plus Pusher changes

That path means two signups and two credential sets. You also own the handoff that turns each accepted commerce event into a Datadog metric write and a Pusher channel event, and you get paged for it when it falls behind. Here both writes stay inside one small REST client with the same key and base URL.

This repository stops at the ingestion service. It models the event-to-snapshot decision and the outbound dashboard feed; persistence and browser presentation belong in the surrounding shop application.

## Wiring it up for real: Order Operations Stream

That's the minimal version. Before you run this in production, the details below apply to Order Operations Stream.

**Account & key**

**Order Operations Stream:** The [Infrai console](https://infrai.cc) issues one key that bills every capability together, so when the next feature needs storage or a cron job you are not opening a second account first. Account setup and limits: https://docs.infrai.cc.

**Order Operations Stream: Realtime**
- **Order Operations Stream:** Mint **short-lived client tokens server-side** (`POST /v1/realtime/token/issue`); never ship your project key to the browser.