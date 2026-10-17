#!/bin/sh
set -eu

curl --fail-with-body --request POST http://localhost:8080/orders \
  --header 'Content-Type: application/json' \
  --data '{"order_id":"ord-1042","customer_id":"cus-17","state":"fulfilled","total":84.50,"occurred_at":"2026-09-24T08:00:00Z"}'
