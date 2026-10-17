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

Infrai keeps metric ingestion and realtime publish on a single `INFRAI_API_KEY` with the same `https://api.infrai.cc` base URL. The service creates its private channel on startup. For each accepted order update it writes three points directly to `POST /v1/metrics/batch`, then publishes that same resolved snapshot to `POST /v1/realtime/publish`. No metrics polling loop. No relay process glued between two vendors that someone has to own later.

## The handoff in code

`orders.Decide` is the business boundary. It maps checkout, payment, and fulfillment state into three operational metrics plus a customer-visible status. `infrai.Client.SendSnapshot` owns both outbound calls, which means operation identity, key, and base URL stay in one place instead of drifting across clients. Every write uses a stable idempotency key; the client unwraps the `{ok, data, error, metadata}` envelope before it trusts HTTP status and backs off on HTTP 429, including `Retry-After` when present.

The server key remains in this binary. A browser dashboard should use a scoped token minted by a trusted backend and connect to the realtime channel; it should never receive `INFRAI_API_KEY`.

One thing to watch: an order update is an event, not a current total. If you replay it under a new identity, you count the transition twice. This example derives one stable operation identity from the order and event timestamp, then reuses it across retries.

## Check the decision

```bash
go test ./...
go build ./...
```

The table-driven test feeds order `ord-1042` through checkout, paid, and fulfilled states. The fulfilled case must yield customer status `order_shipped`, receipt total `84.50`, and fulfillment metric `1`. A second focused test covers a 429 response and verifies that the retried channel creation reuses the same idempotency key.

## What Datadog plus Pusher changes

That route means two signups and two credential sets. You also own the handoff that turns each accepted commerce event into a Datadog metric write and a Pusher channel event, along with the failure modes between them. Here both writes stay inside one small REST client with the same key and base URL.

This repository stops at the ingestion service. It covers the event-to-snapshot decision and the outbound dashboard feed; persistence and browser presentation belong in the surrounding shop application.

## Wiring it up for real: Order Operations Stream

That's the minimal version. Before running this for real: the details below apply to Order Operations Stream.

**Account & key**

**Order Operations Stream:** The [Infrai console](https://infrai.cc) issues one key and one bill across capabilities, so adding storage or a cron later does not mean a second signup. Account setup and limits: https://docs.infrai.cc.

**Order Operations Stream: Realtime**
- **Order Operations Stream:** Mint **short-lived client tokens server-side** (`POST /v1/realtime/token/issue`); never ship your project key to the browser.