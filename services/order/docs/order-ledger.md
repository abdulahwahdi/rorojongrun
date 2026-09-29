# Order service — sales ledger, invoices, cash shifts, exports

`order` is the **book of record for every sale** of the business (the prototype books point-of-sale sales).
It never creates payments and never talks to a gateway: every `payment.*` event the payment service publishes
becomes, or updates, **one order per payment**. On top of the payment it adds what payment does not know about:
the order's own lifecycle after payment, invoices and credit notes, per-merchant tax / rounding / numbering,
cashier shifts with cash reconciliation, reports, asynchronous CSV exports, its own `order.*` events and an
audit trail in the activity service.

```
POS ──quote──▶ order (POST /v1/orders/quote)            price the basket with the merchant's tax + rounding
POS ──create payment (amount = quote total, items, metadata)──▶ payment
payment ──outbox──▶ Kafka payment.created / checkout_started / completed / expired / cancelled
                                   │
                                   ▼
                 order Kafka consumer (one order per paymentId, idempotent, out-of-order safe)
                                   │  paid → confirmed + invoice (+ cash shift)
                                   │  expired / cancelled → cancelled
                                   ▼
 staff ──PATCH /v1/orders/:id/status──▶ preparing → ready → completed → refunded (credit note)
                                   │
order ──outbox──▶ Kafka order.* (kitchen, shipment, …) · notification.requested (emails) · activity.requested (audit)
```

## The two statuses

An order has two **independent** statuses. The payment status follows the payment service until it is paid;
the order's own steps start after that.

| Status | Values | Set by |
|---|---|---|
| `paymentStatus` | `pending` `processing` `paid` `expired` `cancelled` | the payment events (Kafka), or an admin override |
| `orderStatus` | `awaiting_payment` `confirmed` `preparing` `ready` `completed` `cancelled` `refunded` | payment ending (automatic, out of `awaiting_payment` only), then staff |

```
awaiting_payment ──payment paid (auto)──▶ confirmed ──▶ preparing ──▶ ready ──▶ completed ──▶ refunded
       │                                      │            │           │         (staff; steps may be skipped forward, never back)
       └─payment expired/cancelled (auto)─▶ cancelled ◀────┴───────────┘ (staff)
```

* The payment only moves an order **out of `awaiting_payment`**: `paid` confirms it (invoice, cash shift,
  `order.confirmed`), `expired`/`cancelled` cancels it. Kafka never touches `orderStatus` afterwards, so a
  manual step is never undone by a payment event.
* Staff cannot move an order out of `awaiting_payment` except to cancel it (a walk-out). If the money arrives
  afterwards the payment status becomes `paid`, the order stays `cancelled` and the timeline entry is flagged
  **`needs_refund`**.
* Cancelling a **paid** order or refunding a completed one needs a note, issues a **credit note** and, for a
  cash sale, takes the refund out of the refunding cashier's open shift. Refunds of the money itself stay
  manual (the payment service does not automate refunds either).
* Every manual change carries the order's `version` (optimistic lock): a stale version answers `409`.
  `GET /v1/orders/:id` returns `allowedStatuses`, the statuses staff can pick right now.

### Admin override of the payment status

`PATCH /v1/orders/:id/payment-status {status, note, version, methodCode?}` fixes the payment side by hand
(a lost callback, cash taken offline): `pending`, `paid`, `expired` or `cancelled`. It sets
`paymentStatusSource = manual`; a manual `paid` confirms and invoices the order exactly like the event would.
A paid payment cannot be changed by hand (refund the order instead). **The payment service is not changed.**
Kafka still books later events, but a manually set *final* status is never overwritten by them (the timeline
shows them flagged `ignored_manual`); a manual non-final status is overwritten by the next payment event.

## Booking payment events

The consumer subscribes to `PAYMENT_EVENT_TOPICS` and dispatches on the payload's `event` field. Each payment
event is a **full snapshot** (items, customer, description, `occurredAt`; see
`services/payment/docs/payment-flow.md`), because the event types travel on different topics and may arrive in
any order.

* All events of one payment are booked one at a time (a Postgres advisory lock on the payment id), whatever
  topic or partition they came from.
* The **first** event seen creates the order: order number, items, priced breakdown with the merchant's
  settings of the day, `order.created`.
* A redelivered event is booked once (`order_events` unique on `order_id, event, transaction_id`).
* A final payment status is never left; an older non-final event is recorded with the flag `stale` and not
  applied.
* A payload that can never be booked (not JSON, unknown event, no payment id) is logged and skipped. A
  temporary error is retried 3 times in the handler; candi then commits the message anyway, so replaying a
  partition (resetting the consumer group's offsets) is the recovery path — booking is idempotent.

### POS metadata

The payment's `metadata` tells order where the sale belongs. All keys are optional.

| Key | Meaning | Default |
|---|---|---|
| `merchantId` | the business; selects the merchant settings | `*` (the default settings) |
| `outletId` | the store / branch | `""` |
| `cashierId` | the cashier; puts cash sales into their open shift | `""` (cash sale left unassigned) |
| `channel` | `pos`, `web`, `app`, … | `""` |

A payment without items is booked as one line of its amount.

## Merchant settings: tax, rounding, numbering

`merchant_settings` holds one row per merchant; the row `*` applies to merchants without their own
(numbered in their own sequences).

| Field | Example | Notes |
|---|---|---|
| `name`, `address`, `taxId` | `Kopi Kita` | seller data printed on invoices |
| `orderPrefix` | `KOP` | order numbers `KOP-20260929-000123`, per prefix per day |
| `invoicePrefix` / `creditNotePrefix` | `INV` / `CN` | `INV/M1/202609/000123`, per merchant per month (`INV/202609/…` for `*`) |
| `taxName`, `taxRate`, `taxMode` | `PPN`, `11`, `exclusive` | `none`, `inclusive` (extracted from the prices), `exclusive` (added on top) |
| `roundingMode`, `roundingUnit` | `nearest`, `100` | `none`, `nearest`, `up`, `down`; applied to the total |
| `timezone` | `Asia/Jakarta` | number periods, report days, CSV and invoice times |

Numbers come from `number_sequences` rows locked until the transaction ends: **no gaps**, a rolled back number is
handed out again.

**Pricing** (`POST /v1/orders/quote`, and the breakdown recorded on every order):

```
subtotal   = Σ price × quantity
exclusive: tax = round(subtotal × rate)          pre-round = subtotal + tax
inclusive: tax = subtotal − round(subtotal / (1 + rate))   pre-round = subtotal
total      = round(pre-round, roundingMode, roundingUnit)   roundingAdjustment = total − pre-round
```

The POS should quote first and create the payment with `amount = total`. Order does not reject a different
amount — the payment is the truth — but flags the order `amountMismatch` (filterable, exported).
Changing settings never rewrites history: orders and invoices keep a snapshot.

## Invoices and credit notes

* Issued **in the same transaction** that confirms the order: exactly one invoice per order, never changed
  after it is issued (only `status` and `emailedAt`).
* A refund (or cancelling a paid order) issues one **credit note** that references the invoice, which becomes
  `credited`.
* The customer gets an email (`notification.requested`, templates `order_invoice` / `order_credit_note`, seeded
  by `services/notification/cmd/migration/migrations/20260929130000_seed_order_email_templates.sql`) when the
  payment carried `customer.email`. The notification service needs an enabled `email` channel config.
* `GET /v1/invoices/:id/print?format=a4|receipt` renders printable HTML (80 mm receipt or A4).
* Invoice numbers contain `/`, so routes take the numeric id; `GET /v1/invoices/lookup?number=` finds one by
  number.

## Cash shifts

A cashier opens a shift at an outlet with an opening float and closes it with the cash counted in the drawer.
One open shift per cashier and outlet.

* A paid order whose method is `cash` lands in its cashier's open shift (`shiftId`); without one it stays
  unassigned (list `methodCode=cash` orders to find them).
* A cash refund is taken out of the refunding cashier's open shift (`refundShiftId`).
* `expected = openingFloat + cash sales − cash refunds`; closing freezes the numbers and records
  `countedCash` and `difference`. An open shift shows its live numbers.

## Exports (asynchronous CSV)

Request → queue → download. The `export_jobs` row is the source of truth; candi's task queue only carries the
job id.

1. `POST /v1/exports {type, filter}` — `orders` (one row per order), `order_lines` (one row per item) or
   `invoices`. `filter` takes the query parameters of `GET /v1/orders` / `GET /v1/invoices` as a JSON object.
   The rows are counted first: more than `ORDER_EXPORT_MAX_ROWS` answers `400`. Answers `202` with the job.
2. The task queue worker (`order-export`) claims the job (`queued → running`), writes the CSV in keyset
   batches of 1000 with times in the merchant's timezone, records progress, and stops at the next batch when a
   cancel was asked. A failure puts the job back to `queued`; after 3 attempts it is `failed`.
3. `GET /v1/exports/:id/download` streams the file once `completed` (`409` before, `410` once expired).

The cron sweeper queues again a job the queue lost (queued for more than a minute — e.g. requested while the
worker was starting, or on a replica without the task queue worker) and takes over a running job silent for
5 minutes; an hourly purge deletes files past `EXPORT_RETENTION` (`expired`).
Everyone sees their own exports; `manageExports` sees and acts on everybody's.
Files live in `EXPORT_STORAGE_DIR` (a volume shared by the replicas in k8s; `pkg/shared.FileStore` is the seam
for object storage).

## API

Routes are registered flat (no `Group`), same as the user service. Every route is `Bearer` + a permission code
of service `order` (the user service decides, `globalshared/auth`).

| Route | Permission | Notes |
|---|---|---|
| `GET /v1/orders` | `getAllOrders` | filters `paymentStatus` `orderStatus` `source` `merchantId` `outletId` `cashierId` `channel` `methodCode` `shiftId` `amountMismatch` `dateFrom` `dateTo` (YYYY-MM-DD in the merchant's timezone, or RFC3339) `search` |
| `GET /v1/orders/summary` | `getOrderSummary` | same filters + `groupBy=day\|method\|outlet\|cashier`; counts per status, money of paid orders, refunds, net |
| `POST /v1/orders/quote` | `quoteOrder` | `{merchantId, items}` → breakdown |
| `GET /v1/orders/:id` | `getOrder` | id or order number; items, timeline, invoices, `allowedStatuses` |
| `PATCH /v1/orders/:id/status` | `updateOrderStatus` | `{status, note?, version}` |
| `PATCH /v1/orders/:id/payment-status` | `overridePaymentStatus` | `{status, note, version, methodCode?}` |
| `GET /v1/invoices` | `getAllInvoices` | filters `type` `status` `merchantId` `outletId` `orderId` `dateFrom` `dateTo` `search` |
| `GET /v1/invoices/lookup?number=` | `getInvoice` | |
| `GET /v1/invoices/:id` | `getInvoice` | with items |
| `GET /v1/invoices/:id/print?format=a4\|receipt` | `getInvoice` | HTML |
| `POST /v1/invoices/:id/resend` | `resendInvoice` | emails it again |
| `GET /v1/merchant-settings[/:merchantId]` | `getMerchantSettings` | `*` is the default row |
| `PUT /v1/merchant-settings/:merchantId` | `manageMerchantSettings` | create or replace |
| `DELETE /v1/merchant-settings/:merchantId` | `manageMerchantSettings` | not `*` |
| `POST /v1/shifts/open` | `openShift` | `{merchantId?, outletId, cashierId?, openingFloat}`; the cashier defaults to the caller |
| `GET /v1/shifts/current?merchantId=&outletId=&cashierId=` | `getShifts` | live numbers |
| `GET /v1/shifts[/:id]` | `getShifts` | the detail lists the shift's cash orders |
| `POST /v1/shifts/:id/close` | `closeShift` | `{countedCash, note?}` |
| `POST /v1/exports` | `exportOrders` / `exportInvoices` | by type |
| `GET /v1/exports[/:id]` | `getExports` | own exports, all with `manageExports` |
| `POST /v1/exports/:id/cancel` | `cancelExport` | queued → cancelled at once, running → stops at the next batch |
| `GET /v1/exports/:id/download` | `downloadExport` | CSV |

Register these codes (service `order`) in each realm that uses them, see `services/user/docs/realms-and-rbac.md`.

## Kafka

**In** — `PAYMENT_EVENT_TOPICS` (default the five `payment.*` topics), consumer group `KAFKA_CONSUMER_GROUP`.

**Out** — through the transactional outbox `order_outbox`: written in the same DB transaction as the change,
published right after commit and by a cron every 10 s. An event exists if and only if its change committed; a
Kafka outage delays events but never loses them (order is kept). Topic = event name, key = order number.

| Topic | When | Payload |
|---|---|---|
| `order.created` | first event of a payment booked | `{event, occurredAt, order}` (order with items) |
| `order.confirmed` | payment paid (auto or override) | same — **the kitchen's cue** |
| `order.status_updated` | every manual order status change | + `actor`, `note`, `fromStatus`, `toStatus` |
| `order.completed` / `order.cancelled` / `order.refunded` | those steps | same |
| `order.invoice_issued` | an invoice or credit note | `{event, occurredAt, invoice}` (key = invoice number) |
| `order.shift_opened` / `order.shift_closed` | shifts | `{event, occurredAt, shift}` |
| `notification.requested` (`ORDER_NOTIFICATION_TOPIC`) | invoice / credit note emails | the notification service's send payload |
| `activity.requested` (`ORDER_ACTIVITY_TOPIC`) | **every change** | `{serviceName:"order", eventType, referenceId, actorId, message, metadata}` |

### Audit trail (activity service)

Every change is also written to the activity service's audit trail through `activity.requested`: orders placed,
every payment event booked (with its flag), automatic confirmations and cancellations, manual status changes and
payment overrides (with the actor and note), invoices and credit notes issued or resent, shifts opened and
closed, exports requested, cancelled and downloaded. `referenceId` is the order number (`shift-<id>` /
`export-<id>` for shifts and exports), so `GET /v1/activity?serviceName=order&referenceId=ORD-…` in the activity service is an
order's history. The activity service consumes the topic directly (no REST dependency).

## Configuration

| Env | Meaning |
|---|---|
| `USE_KAFKA_CONSUMER=true` | books payment events |
| `USE_CRON_SCHEDULER=true` | required: outbox flush (10 s), export sweeper (1 min), export purge (1 h) |
| `USE_TASK_QUEUE_WORKER=true` | required: generates exports. `taskqueueworker.AddJob` only works in the process running the worker, so run it with the REST API (the sweeper covers replicas without it) |
| `PAYMENT_EVENT_TOPICS` | payment topics, comma separated |
| `ORDER_NOTIFICATION_TOPIC` | default `notification.requested` |
| `ORDER_ACTIVITY_TOPIC` | default `activity.requested` |
| `EXPORT_STORAGE_DIR`, `EXPORT_RETENTION`, `ORDER_EXPORT_MAX_ROWS` | exports (defaults `./storage/exports`, `168h`, `100000`) |
| `ISSUER_BASE_URL`, `USER_HTTP_HOST`, `USER_GRPC_HOST`, `USER_BASIC_AUTH_KEY` | auth through the user service |

Run the migration first: `make migration service=order` (it seeds the `*` merchant settings).

## Tests

* Unit tests: `go test ./services/order/...` (pricing, rounding, numbering, transitions, event application,
  CSV rows, invoice HTML).
* Integration tests against a real Postgres — they drive the real usecases, SQL, locks and unique indexes
  (out-of-order and concurrent events, redeliveries, overrides, refunds and credit notes, cash shifts, summary,
  the whole export lifecycle). They are skipped unless `ORDER_TEST_DSN` points to a **throwaway** database,
  which they wipe:

```bash
docker run -d --name order-test-pg -e POSTGRES_USER=user -e POSTGRES_PASSWORD=pass -e POSTGRES_DB=order_test -p 55432:5432 postgres:16-alpine
ORDER_TEST_DSN='postgres://user:pass@localhost:55432/order_test?sslmode=disable' go test -race ./services/order/...
```

## Not covered

Automated refunds of the money (credit notes record them; paying back is manual), a merchant registry beyond
the settings rows, multi-currency (IDR only, like payment), and object storage for exports.
