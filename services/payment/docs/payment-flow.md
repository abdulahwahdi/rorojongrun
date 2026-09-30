# Payment service — gateways, checkout journey, callbacks

`payment` is the bridge between the other services and the payment gateways (Midtrans, Xendit) or cash.
A service asks for a **payment link**; every step after that (choose a method, pay, wait, done) happens
in this service. Gateway callbacks come in over **Kafka**, on topics that are configured in the database.

```
order ──POST /v1/payments──▶ payment ──▶ {paymentUrl}                 (caller side)
                                │
customer ──▶ checkout frontend ─┤ GET  /v1/checkout/{token}           (customer side, token = credential)
                                │ GET  /v1/checkout/{token}/methods
                                │ PUT  /v1/checkout/{token}/method
                                │ POST /v1/checkout/{token}/pay ──▶ gateway (charge)  or  cash code
                                │ GET  /v1/checkout/{token}/status   (poll)
                                │
gateway ─▶ webhook ingress ─▶ Kafka ─▶ payment callback consumer ─▶ verify, settle
                                │
payment ──outbox──▶ Kafka: payment.created / .checkout_started / .completed / .expired / .cancelled
                          notification.requested (customer emails)
```

## Model

| Table | Purpose |
|---|---|
| `payment_gateways` | one row per gateway (`midtrans`, `xendit`, `mock`): enabled flag, environment, non-secret `settings`, **encrypted** `credentials_enc` |
| `payment_methods` | what the customer can pick: type, gateway + channel, amount range, fee (flat + percent), sort order, enabled flag. `cash` has no gateway |
| `payment_kafka_topics` | Kafka topics: `consume` topics carry gateway callbacks, `publish` topics carry outbound events |
| `payments` | one payment link per request of another service, unique per open `(source, referenceId)` |
| `payment_transactions` | one row per checkout attempt; its id is the `order_id` / `external_id` sent to the gateway |
| `payment_callback_logs` | every callback consumed, with its outcome; failed ones can be replayed |
| `payment_outbox` | events written in the same DB transaction as the change that caused them |

## Payment states

```
pending ──pay──▶ processing ──callback paid / cash confirmed──▶ paid          (final)
   ▲                │  │
   └─ attempt failed┘  ├─ cancel ─▶ cancelled                                  (final)
      or method changed└─ expiry ─▶ expired                                    (final)
```

* A failed or expired **attempt** puts the payment back to `pending`: the customer may choose another method.
* Selecting another method while an attempt is open cancels that attempt (and the gateway charge, best effort).
* A payment is settled by *any* of its attempts that gets paid, even one the customer already replaced.
* Money received for a payment that is already `paid` or `cancelled` is recorded on the transaction and the
  callback is logged as `ignored` with `refund required`. Refunds are not automated (out of scope).
* Expiry is applied by a cron (every minute) and lazily whenever a payment is read or paid.

## API

Routes are registered flat (no `Group`), same as the user service.

**Caller side** — bearer token + permission code. The user service authorises (`globalshared/auth`).

| Route | Permission | Notes |
|---|---|---|
| `POST /v1/payments` | `createPayment` | idempotent per `(source, referenceId)` while open; a service token is pinned to its client id as `source`, an operator token may set `source` in the body |
| `GET /v1/payments`, `GET /v1/payments/:id` | `getAllPayments`, `getPayment` | includes the attempts |
| `POST /v1/payments/:id/cancel` | `cancelPayment` | twice is fine; a paid or expired payment answers 409 |

The same three operations exist over gRPC (`payment.PaymentHandler`, internal basic auth key) and as the
`sdk/payment` client (`NewPaymentServiceGRPC` / `NewPaymentServiceREST`).

**Checkout side** — public, the unguessable token in the path is the credential.

| Route | Notes |
|---|---|
| `GET /v1/checkout/:token` | summary, status, items, current attempt with its payment instruction |
| `GET /v1/checkout/:token/methods` | only methods that are enabled, in the amount range, allowed by the caller and whose gateway is enabled **and** has credentials; fee and total per method |
| `PUT /v1/checkout/:token/method` `{methodCode}` | selects a method, computes fee and total |
| `POST /v1/checkout/:token/pay` `{methodCode?}` | starts the attempt: gateway charge, or a cash code. Idempotent while the same attempt runs |
| `GET /v1/checkout/:token/status` | light poll |
| `POST /v1/checkout/:token/cancel` | the customer cancels |

The payment instruction (`transaction.instruction`) is gateway independent: `type` plus `vaNumber` /
`billKey` / `qrString` / `qrUrl` / `deeplinkUrl` / `url` (redirect) / `cashCode`.

**Cash** — staff token. `GET /v1/cash/:cashCode` looks up the amount due, `POST /v1/cash/:cashCode/confirm`
`{amountReceived}` settles the payment (must cover the total, the change is returned).
Permission `confirmCashPayment`.

**Admin** — permissions `manageGateways`, `manageMethods`, `manageTopics`, `getCallbackLogs`, `replayCallback`.

| Route | Notes |
|---|---|
| `GET/PUT /v1/gateways[/:code]`, `PATCH /v1/gateways/:code/status` | credentials are write-only and merged (an empty value deletes a key); reads show them masked. A gateway cannot be enabled without its required credentials |
| `/v1/payment-methods` | CRUD + `PATCH /:id/status` |
| `/v1/kafka-topics` | CRUD + `PATCH /:id/status` |
| `GET /v1/callback-logs[/:id]`, `POST /v1/callback-logs/:id/replay` | authenticating headers are never shown |
| `POST /v1/dev/mock-callback` | dev only, refused when `ENVIRONMENT=production` |

## Gateways

| Gateway | Credentials | Settings | How it charges | How a callback is verified |
|---|---|---|---|---|
| `midtrans` | `serverKey` | `baseUrl`, `snapBaseUrl` (optional) | Core API `/v2/charge` for VA (BCA/BNI/BRI/Permata/Mandiri bill), QRIS, GoPay, ShopeePay; Snap redirect for cards | `SHA512(order_id + status_code + gross_amount + serverKey)` |
| `xendit` | `secretKey`, `callbackToken` | `baseUrl` (optional) | Invoice API (hosted page); channel `invoice` = customer picks on Xendit's page | `x-callback-token` header |
| `mock` | `callbackToken` (optional) | — | no network; fake VA number | optional `x-mock-token` header. Never in production |

Credentials are stored AES-256-GCM encrypted with `GATEWAY_ENCRYPTION_SECRET` (`globalshared/crypto`).
**Changing that secret makes stored credentials unreadable** — enter them again afterwards. Gateways start
disabled; enabling needs the credentials. A gateway that is disabled later still settles callbacks for
charges it already issued.

```bash
curl -X PUT  $PAY/v1/gateways/midtrans -H "authorization: Bearer $T" -H 'content-type: application/json' \
  -d '{"environment":"sandbox","credentials":{"serverKey":"SB-Mid-server-..."}}'
curl -X PATCH $PAY/v1/gateways/midtrans/status -H "authorization: Bearer $T" -H 'content-type: application/json' \
  -d '{"isEnabled":true}'
```

## Kafka

### Callbacks in (configurable, hot-reloaded)

candi's own Kafka worker fixes its topics when the service starts. Payment therefore runs its own consumer
(`USE_CALLBACK_CONSUMER=true`, consumer group `CALLBACK_CONSUMER_GROUP`) that reads the enabled `consume`
rows of `payment_kafka_topics` (whose gateway is enabled) and **re-reads them every
`CALLBACK_TOPIC_RELOAD_INTERVAL`** (default 30s); a change made through this instance's admin API applies
at once. When the set changes it rejoins the group with the new topics — no restart. `USE_KAFKA_CONSUMER`
stays `false`, payment has no static topics.

Add a topic: `POST /v1/kafka-topics {"topic":"my.midtrans.callbacks","direction":"consume","gatewayCode":"midtrans"}`
(the topic must exist in Kafka). Disable it with `PATCH /v1/kafka-topics/:id/status`.

Message shape, published by whatever receives the gateway webhook (that ingress is outside this service):

```json
{"headers": {"x-callback-token": "..."}, "body": { ...the gateway's JSON... }}
```

`body` may also be a JSON *string* when the exact bytes matter for a signature. A message without `body` is
taken as the raw gateway payload, with the Kafka record headers standing in for the HTTP headers.

Processing:

* verified with the gateway's own scheme, amount must equal the transaction's amount; forged, unknown
  or mismatching callbacks are logged (`failed` / `ignored`) and **not** retried;
* transient errors (DB down) are retried 3 times, then logged as `failed` and the partition moves on —
  replay it with `POST /v1/callback-logs/:id/replay`;
* idempotent: a callback delivered twice is logged as `duplicate`.

### Events out (configurable topics, transactional outbox)

Events are inserted into `payment_outbox` in the **same DB transaction** as the state change, then
published right after commit and by a cron every 10s. So an event exists if and only if its change
committed, and a Kafka outage delays events but never loses them (order is kept).
The topic name of each event type is looked up in `payment_kafka_topics` (`publish` rows) at publish time;
an event type can fan out to several topics; if no topic is enabled the event is dropped (switched off on purpose).

| Event type → default topic | When |
|---|---|
| `payment.created` | a payment link was created |
| `payment.checkout_started` | **every checkout attempt** (each pay action, cash or gateway), queued before the gateway is called |
| `payment.completed` | the payment was paid |
| `payment.expired` / `payment.cancelled` | the payment ended that way |
| `notification_email` → `notification.requested` | a customer email for the notification service |

Payload of the `payment.*` events (key = payment id):

```json
{"event":"payment.checkout_started","paymentId":"…","transactionId":"…","source":"pos","referenceId":"ORD-1001",
 "description":"Table 4","status":"processing","amount":150000,"fee":0,"totalAmount":150000,"currency":"IDR",
 "methodCode":"mock_va","gatewayCode":"mock","paidAt":null,"expiresAt":"2026-09-26T12:07:12Z",
 "metadata":{"merchantId":"M1","outletId":"O1","cashierId":"u-1","channel":"pos"},
 "customer":{"name":"Budi","email":"budi@example.com"},"items":[{"name":"Kopi","price":75000,"quantity":2}],
 "createdAt":"2026-09-25T12:07:12Z","occurredAt":"2026-09-25T12:08:40Z"}
```

Every event is a **full snapshot** of the payment (description, customer, items included): the event types travel on
different topics, so a consumer may see `payment.completed` before `payment.created` and must be able to build its
record from whichever arrives first. `occurredAt` is when the change happened, for ordering non-final updates.

Callers filter on `source` and match on `referenceId` / `metadata`. The `order` service records **every** payment as an
order (see `services/order/docs/order-ledger.md`); the metadata keys `merchantId`, `outletId`, `cashierId` and `channel`
are what it books the sale under.

## Customer emails

Every checkout attempt sends **one email with the payment instructions** (template `payment_checkout`) and
a paid payment sends **one receipt** (`payment_paid`), through the notification service: a
`notification.requested` message (same shape as `POST /v1/notification/send`). Being in the outbox, the
receipt cannot be lost and a rolled-back checkout never mails. No `customer.email` → no email.

Prerequisites in the **notification** service: the two templates (seeded by its migration
`20260925130000_seed_payment_email_templates.sql`, editable through its template API) and an **enabled
`email` channel config** (`PUT /v1/notification/channel-configs/email {"isEnabled":true}`).

## Configuration

| Env | Meaning |
|---|---|
| `CHECKOUT_BASE_URL` | the payment link is `<base>/<token>`; the frontend calls the checkout API with that token |
| `GATEWAY_ENCRYPTION_SECRET` | encrypts gateway credentials (see above) |
| `DEFAULT_PAYMENT_EXPIRY` | lifetime of a link when the caller gives no `expiresInSec` (default 24h; allowed 60s–30d) |
| `USE_CALLBACK_CONSUMER`, `CALLBACK_CONSUMER_GROUP`, `CALLBACK_TOPIC_RELOAD_INTERVAL` | the callback consumer |
| `USE_CRON_SCHEDULER=true` | required: expiry and the outbox safety-net flush run in the cron |
| `ISSUER_BASE_URL`, `USER_HTTP_HOST`, `USER_GRPC_HOST`, `USER_BASIC_AUTH_KEY` | auth through the user service |
| `ENVIRONMENT=production` | disables the mock gateway's dev endpoint |

Register the permission codes above in each realm that uses them (see `services/user/docs/realms-and-rbac.md`,
service name `payment`). Run the migration first: `make migration service=payment`.

## Try it locally with the mock gateway

```bash
# operator token from the user service, then:
api() { curl -s "$@" -H "authorization: Bearer $T" -H 'content-type: application/json'; }
api -X PATCH $PAY/v1/gateways/mock/status -d '{"isEnabled":true}'
api -X POST  $PAY/v1/payments -d '{"source":"order","referenceId":"ORD-1","amount":150000,"customer":{"name":"Budi","email":"budi@example.com"}}'
# customer: (no auth)
curl -s -X POST $PAY/v1/checkout/$TOKEN/pay -H 'content-type: application/json' -d '{"methodCode":"mock_va"}'
# the "gateway" calls back: published to the mock consume topic and settled by the consumer
api -X POST $PAY/v1/dev/mock-callback -d '{"transactionId":"<transaction.id>","status":"paid"}'
curl -s $PAY/v1/checkout/$TOKEN/status
```

## Not covered

Refunds and partial payments (a payment that gets paid twice is flagged in the callback log, refunding is
manual), the webhook → Kafka ingress itself, and the checkout frontend.
