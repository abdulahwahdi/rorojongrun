# RoRoJongRun — Microservices Monorepo (Candi Framework)

This repository hosts a set of Go microservices built on **[golangid/candi](https://github.com/golangid/candi)**,
generated and managed with the `candi` CLI in **monorepo mode**. All services
live in one repo/workspace but run and deploy independently.

## Services

| Service        | Folder                     | Responsibility (draft — refine as domain logic is built) |
|----------------|-----------------------------|------------------------------------------------------------|
| `user`         | `services/user`             | Account/profile management, auth, roles |
| `payment`      | `services/payment`          | Payment intents, transactions, payment gateway integration |
| `order`        | `services/order`            | Order lifecycle, cart-to-order, order status orchestration |
| `notification` | `services/notification`     | Push/email/SMS notifications, fan-out on domain events |
| `activity`     | `services/activity`         | Activity/audit log & history feed per user or order |
| `shipment`     | `services/shipment`         | Delivery/courier assignment, tracking, delivery status |
| `kitchen`      | `services/kitchen`          | Kitchen/merchant order queue, prep status |

> Note: folder is named `kitchen` (fixing the "kichen" typo from the request). Keep this
> spelling consistent everywhere (module name, package prefix, Kafka topics, etc).

Naming convention for anything folder/package/topic-related should always use the
english service name above (`user`, `payment`, `order`, `notification`, `activity`,
`shipment`, `kitchen`) — never abbreviate or re-spell it per-service.

## Why Candi

Candi is a Go microservice toolkit/CLI that scaffolds a clean-architecture service
(`domain` / `usecase` / `repository` / `delivery`) and gives every service the same
shape for servers (REST, GraphQL, gRPC) and workers (Kafka, RabbitMQ, Redis
subscriber, Postgres event listener, Cron, Task Queue), plus built-in Jaeger
tracing, graceful shutdown, and DI wiring. Using one framework across all 7
services keeps them consistent and lets any engineer move between services
without relearning structure.

## Monorepo layout

```
rorojongrun/
├── services/
│   ├── user/
│   ├── payment/
│   ├── order/
│   ├── notification/
│   ├── activity/
│   ├── shipment/
│   └── kitchen/
├── libraries/              # shared internal Go packages (if any), imported by services
├── deployments/            # per-service Dockerfiles / k8s manifests / docker-compose overlays
├── docker-compose.yml      # local infra: kafka, redis, postgres/mongo, jaeger, etc.
├── Makefile                # monorepo-level make targets (delegates to candi -run)
└── CLAUDE.md
```

This layout matches what `candi -init-monorepo` generates. Do not hand-roll a
different tree — always generate through the CLI so generated code stays
consistent with what `-add-module` / `-add-handler` expect later.

## Candi CLI — the only way to scaffold code

Install once:

```bash
go install github.com/golangid/candi/cmd/candi@latest
```

The CLI is interactive; `-scope` picks the mode (1–5), then it prompts for the
rest (service name, module name(s), delivery handlers, database, etc).

| Scope | Flag combo | Purpose |
|-------|------------|---------|
| 1 | `candi -scope=1` (or `-init-monorepo`) | Initialize the monorepo skeleton (once, already done for this repo) |
| 2 | `candi -scope=2` (or `-init`) | Initialize a new service inside the monorepo |
| 3 | `candi -scope=3` (or `-add-module`) | Add a module to an existing service |
| 4 | `candi -scope=4` (or `-add-handler`) | Add a delivery/worker handler to an existing module |
| 5 | `candi -scope=5` (or `-run`) | Run one or more services locally (`-service=user,order`) |

Useful flags: `-monorepo-name`, `-packageprefix`, `-output` (target directory),
`-withgomod` (default true — keep it true per-service), `-protooutputpkg` (gRPC
proto output path), `-service=<name1,name2>` for `-run`.

### Creating each of the 7 services

Run from the repo root, once per service, e.g.:

```bash
candi -scope=2 -output=services/user
candi -scope=2 -output=services/payment
candi -scope=2 -output=services/order
candi -scope=2 -output=services/notification
candi -scope=2 -output=services/activity
candi -scope=2 -output=services/shipment
candi -scope=2 -output=services/kitchen
```

The prompts will ask for: service name, initial module name(s), which
delivery/worker handlers to enable, and which database driver to use. Pick
per-service based on the responsibility table above (guidance below).

### Adding a module or handler later

```bash
candi -scope=3 -output=services/order      # add module inside order service
candi -scope=4 -output=services/order      # add a new handler to an existing module
```

**Never** hand-write the `domain/usecase/repository/delivery` skeleton for a new
module — always run `-add-module`/`-add-handler` so generated boilerplate
(interfaces, mocks, DI wiring, module registration) stays in sync with the rest
of the codebase.

## Generated service structure (per service)

```
services/<name>/
├── cmd/<name>/main.go            # entrypoint, wires ServiceFactory
├── config/                       # env loading, DB/broker/tracer setup
├── internal/modules/<module>/
│   ├── domain/                   # entities, request/response structs
│   ├── repository/                # interfaces + impl (mongo/sql) + mocks
│   ├── usecase/                   # business logic + mocks
│   ├── delivery/
│   │   ├── resthandler/
│   │   ├── graphqlhandler/
│   │   ├── grpchandler/
│   │   └── workerhandler/         # kafka/cron/redissubscriber/taskqueue/postgres/rabbitmq
│   └── module.go                  # implements factory.ModuleFactory
├── api/                           # proto files / graphql schema, if applicable
├── Dockerfile
├── Makefile
├── go.mod
└── docker-compose.yml (local infra override, optional)
```

## Suggested delivery/worker handlers per service

Use this as the default answer when the CLI prompts "select active handler(s)".
Adjust only with a clear reason (write it down here when you deviate).

- **user** — REST + GraphQL (client-facing), gRPC (server-to-server auth checks)
- **payment** — REST/gRPC for synchronous calls, Kafka consumer for async payment
  gateway webhooks/callbacks
- **order** — REST/GraphQL for client-facing order creation, Kafka producer/consumer
  to publish order-state events and consume payment/kitchen/shipment status
- **notification** — Kafka consumer only (fan-out on events from order/payment/
  shipment/kitchen), no public API needed unless there's a notification-preferences REST endpoint
- **activity** — Kafka consumer (subscribes to domain events from every other
  service for the audit/history feed) + REST for read/query
- **shipment** — REST/gRPC + Kafka consumer (order created) / producer (delivery
  status changes)
- **kitchen** — REST/gRPC (merchant app) + Kafka consumer (order created) /
  producer (order ready)

## Inter-service communication

- **Synchronous** (gRPC): use when a caller needs an immediate answer — e.g.
  `order` calling `payment` to charge, `order`/`shipment`/`kitchen` calling `user`
  to fetch profile/address.
- **Asynchronous** (Kafka): use for state-change fan-out — e.g. `order.created`,
  `payment.completed`, `kitchen.order_ready`, `shipment.delivered`. `notification`
  and `activity` should almost always be pure Kafka consumers, never a direct
  synchronous dependency of the producing service.
- Topic naming: `<service>.<event>` in past tense, e.g. `order.created`,
  `payment.completed`, `kitchen.order_ready`, `shipment.delivered`.
- Keep event payload contracts (proto/JSON schema) in `libraries/` (or a shared
  `contracts` module) if/when duplication across services becomes a problem —
  don't create that abstraction preemptively before at least two services need it.

## Running services locally

```bash
candi -scope=5 -service=user,order,payment      # run a subset
candi -scope=5                                   # run all services in the monorepo
```

Or use the generated per-service `Makefile` / `docker-compose.yml` for a single
service: `cd services/order && make run`.

Local infra (Kafka, Redis, Postgres/Mongo, Jaeger) is expected to run via the
root `docker-compose.yml` — bring it up before starting services.

## Conventions

- Follow candi's clean-architecture separation strictly: `delivery` layer only
  parses input/calls usecase; business logic lives in `usecase`; data access
  lives in `repository`; `domain` has no framework/behavior — data shapes only.
- Every usecase/repository interface gets a generated mock (via candi's
  mockgen setup) — regenerate mocks after changing an interface, don't hand-edit them.
- Config comes from environment variables loaded in `config/`, never hardcoded
  — mirror the `.env.example` per service pattern if candi generates one.
- Tracing: keep Jaeger enabled in every service; don't strip tracing spans that
  candi wires up automatically for handlers.
- One module per bounded concern inside a service (e.g. `order` service might
  eventually get an `order` module and a `cart` module) — don't cram unrelated
  entities into one module.
- Never modify files under a module's generated interface files
  (`repository.go`, `usecase.go`) by hand to add new methods — use
  `-add-handler`/CLI-driven regeneration where possible, or if hand-editing is
  unavoidable, update the interface, implementation, and mock together in the
  same change.

## Commit/PR scope

Each PR should generally touch one service (or the shared `libraries/`/root
tooling). Avoid cross-cutting PRs that touch multiple services' business logic
at once unless the change is mechanical (e.g. a shared contract update).
