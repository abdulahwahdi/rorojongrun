<div align="center">

# 🧾 Order

**The sales ledger of RoRoJongRun** — every payment becomes a booked order, with invoices, cash shifts,
reports and exports on top.

![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)
![candi](https://img.shields.io/badge/candi-v1.20-6E4AFF)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-ledger-4169E1?logo=postgresql&logoColor=white)
![Kafka](https://img.shields.io/badge/Kafka-events%20in%20%26%20out-231F20?logo=apachekafka&logoColor=white)
![REST](https://img.shields.io/badge/API-REST-0A7E07)
![Tests](https://img.shields.io/badge/tests-unit%20%2B%20Postgres%20integration-brightgreen)

[How it works](#-how-it-works) · [Features](#-features) · [Quick start](#-quick-start) · [API](#-api-at-a-glance) · [Events](#-events) · [Full reference](docs/order-ledger.md)

</div>

---

## ✨ What it is

`order` never creates payments and never talks to a gateway. It **listens**: each `payment.*` event from the
[payment service](../payment/docs/payment-flow.md) creates or updates **one order per payment** — whatever order
the events arrive in, however often Kafka redelivers them. Built for any business; the prototype books a
point-of-sale.

## 🔄 How it works

```mermaid
flowchart LR
    POS([🛒 POS]) -->|1 · quote| ORD
    POS -->|2 · create payment, amount = quote total| PAY[💳 payment]
    PAY -->|payment.* events| K1{{Kafka}}
    K1 -->|3 · book| ORD[🧾 order]
    STAFF([👩‍🍳 staff]) -->|4 · preparing → ready → completed| ORD
    ORD -->|order.* events| K2{{Kafka}}
    K2 --> KIT[🍳 kitchen / shipment]
    K2 --> NOT[✉️ notification<br/>invoice emails]
    K2 --> ACT[📜 activity<br/>audit trail]
```

Two **independent** statuses: the payment runs until it is paid, the order's own steps start after that.

```mermaid
stateDiagram-v2
    direction LR
    [*] --> awaiting_payment: first payment event
    awaiting_payment --> confirmed: payment paid (auto) · invoice issued
    awaiting_payment --> cancelled: payment expired / cancelled (auto) · walk-out (staff)
    confirmed --> preparing
    preparing --> ready
    ready --> completed
    confirmed --> completed: steps can be skipped forward
    confirmed --> cancelled: credit note
    preparing --> cancelled
    ready --> cancelled
    completed --> refunded: credit note
    cancelled --> [*]
    refunded --> [*]
```

## 🧩 Features

| | |
|---|---|
| 📥 **Event-sourced ledger** | One order per payment, built from whichever event arrives first; redeliveries booked once; stale events recorded, never applied; every event of a payment serialised. |
| 🔀 **Two statuses** | `paymentStatus` from payment, `orderStatus` for the business steps; an admin can override the payment status after a lost callback. |
| 🧮 **Tax & rounding per merchant** | PPN inclusive / exclusive, rounding to Rp100 (nearest / up / down), a quote endpoint so the POS charges exactly what the ledger expects. |
| 🔢 **Gapless numbering** | `KOP-20260929-000123` orders, `INV/M1/202609/000042` invoices, prefixes per merchant, periods in the merchant's timezone. |
| 🧾 **Invoices & credit notes** | Issued in the same transaction as the confirmation, immutable, emailed, printable as A4 or 80 mm receipt. |
| 💵 **Cash shifts** | Opening float, cash sales and refunds per cashier, expected vs counted at closing. |
| 📊 **Reports** | Totals per status, paid money, tax, rounding, fees, refunds and net — by day, method, outlet or cashier. |
| 📤 **Async CSV exports** | Request → task queue → download; progress, cancel, retries, expiry, per-user visibility. |
| 📣 **Outbox events** | `order.*`, invoice emails and the audit trail leave through a transactional outbox: never lost, never phantom. |
| 📜 **Audit trail** | Every change — who, what, why — lands in the activity service. |

## 🚀 Quick start

```bash
# from the repo root
cp services/order/.env.sample services/order/.env     # Postgres, Kafka, user service hosts
make migration service=order                          # tables + the default merchant settings "*"
make run service=order                                # REST :8040 · task queue dashboard :8041
```

Then, with a token from the user service (`T`):

```bash
api() { curl -s "$@" -H "authorization: Bearer $T" -H 'content-type: application/json'; }

# a merchant: PPN 11% on top, totals rounded to Rp100, order numbers KOP-…
api -X PUT localhost:8040/v1/merchant-settings/M1 \
  -d '{"name":"Kopi Kita","orderPrefix":"KOP","taxRate":11,"taxMode":"exclusive","roundingMode":"nearest","roundingUnit":100}'

# price the basket before asking payment for a link
api -X POST localhost:8040/v1/orders/quote \
  -d '{"merchantId":"M1","items":[{"name":"Kopi","price":18000,"quantity":2},{"name":"Roti","price":12500,"quantity":1}]}'
# → subtotal 48500 · tax 5335 · rounding -35 · total 53800

# create the payment in the payment service with amount 53800, the same items and
# metadata {"merchantId":"M1","outletId":"O1","cashierId":"<user id>","channel":"pos"} — the order books itself
api "localhost:8040/v1/orders?merchantId=M1&dateFrom=$(date +%F)"
```

Requires `USE_KAFKA_CONSUMER`, `USE_CRON_SCHEDULER` and `USE_TASK_QUEUE_WORKER` (see
[configuration](docs/order-ledger.md#configuration)) and the permission codes of service `order` in your realm.

## 🗺️ API at a glance

| Area | Routes |
|---|---|
| **Orders** | `GET /v1/orders` · `GET /v1/orders/summary` · `POST /v1/orders/quote` · `GET /v1/orders/:id` · `PATCH /v1/orders/:id/status` · `PATCH /v1/orders/:id/payment-status` |
| **Invoices** | `GET /v1/invoices` · `GET /v1/invoices/lookup?number=` · `GET /v1/invoices/:id` · `GET /v1/invoices/:id/print` · `POST /v1/invoices/:id/resend` |
| **Merchants** | `GET /v1/merchant-settings[/:merchantId]` · `PUT` · `DELETE` |
| **Shifts** | `POST /v1/shifts/open` · `GET /v1/shifts/current` · `GET /v1/shifts[/:id]` · `POST /v1/shifts/:id/close` |
| **Exports** | `POST /v1/exports` · `GET /v1/exports[/:id]` · `POST /v1/exports/:id/cancel` · `GET /v1/exports/:id/download` |

Permissions, filters and payloads: [docs/order-ledger.md › API](docs/order-ledger.md#api).

## 📣 Events

| In (from payment) | Out (through the outbox) |
|---|---|
| `payment.created` · `payment.checkout_started` · `payment.completed` · `payment.expired` · `payment.cancelled` | `order.created` · `order.confirmed` · `order.status_updated` · `order.completed` · `order.cancelled` · `order.refunded` · `order.invoice_issued` · `order.shift_opened` · `order.shift_closed` · `notification.requested` · `activity.requested` |

## 🗂️ Inside

```
internal/modules/
├── order/      the ledger: payment-event consumer, lifecycle, reports, outbox, cron
├── invoice/    invoices, credit notes, printable HTML, emails
├── merchant/   tax, rounding, numbering and seller settings; the quote
├── shift/      cash shifts and reconciliation
└── export/     async CSV exports on candi's task queue
pkg/shared/     models, filters, pricing + numbering, outbox & sequence repositories, file store
test/           integration tests against a real Postgres
```

## 🧪 Tests

```bash
go test ./services/order/...                       # unit tests
ORDER_TEST_DSN='postgres://user:pass@localhost:55432/order_test?sslmode=disable' \
  go test -race ./services/order/...               # + integration tests (wipes that database)
```

## 🛠️ Everyday commands

| Task | Command (repo root) |
|---|---|
| Migrate | `make migration service=order` · new one: `make migration service=order create <name>` · rollback: `make migration service=order down` |
| Run | `make run service=order` |
| Mocks · tests | `make mocks service=order` · `make test service=order` |
| Docker image | `make docker service=order` |

---

<div align="center">

Part of the [RoRoJongRun monorepo](../../README.md) · built with [candi](https://github.com/golangid/candi) ·
full reference in **[docs/order-ledger.md](docs/order-ledger.md)**

</div>
