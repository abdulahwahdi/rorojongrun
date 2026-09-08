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
├── sdk/                    # candi-generated client SDKs, one per service, for other services to import
├── globalshared/           # candi-generated shared Go code used across ALL services in the monorepo
├── go.mod, go.sum          # ONE Go module for the entire monorepo (module name: "monorepo")
├── Dockerfile              # single, parameterized by --build-arg SERVICE_NAME=<name>
├── Makefile                # monorepo-level make targets (init/add-module/proto/migration/build/run/docker/mocks/test), all take service=<name>
├── README.md               # candi's own generated usage cheatsheet
└── CLAUDE.md
```

`sdk/`, `globalshared/`, `go.mod`, `Makefile`, `README.md`, and `Dockerfile` at the
root are all generated verbatim by `candi -init-monorepo` (already run once for
this repo) — don't rename or relocate them, other services' imports and the
CLI's own codegen assume these exact paths.

**Important:** this is a **single Go module** for the whole monorepo — the root
`go.mod` declares `module monorepo` (candi hardcodes this literal name; it does
not follow `-monorepo-name`). Services do **not** get their own `go.mod` or
`go.sum`. Every service imports shared code as `monorepo/globalshared/...` and
`monorepo/sdk/<service>/...`, and a service's own code is imported (e.g. from
`main.go`) as `monorepo/services/<name>/internal`. Never pass `-packageprefix`
when generating a new service/module unless you are prepared to keep it
consistent on *every* future `-init`/`-add-module` call forever — leaving it
unset is what keeps every service's import paths consistent automatically.

This layout matches what `candi -init-monorepo` generates. Do not hand-roll a
different tree — always generate through the CLI so generated code stays
consistent with what `-add-module` / `-add-handler` expect later.

## Candi CLI — the only way to scaffold code

Install once:

```bash
go install github.com/golangid/candi/cmd/candi@latest
```

The CLI is interactive; `-scope` picks the mode (1–5), then it prompts for the
rest (service name, module name(s), delivery handlers, database, etc). Prefer
the explicit boolean flags below over `-scope=N` — they're self-documenting
and match what candi's own generated `README.md`/`Makefile` use.

| Scope | Flag | Purpose |
|-------|------|---------|
| 1 | `-init` | Initialize a new service inside the monorepo |
| 2 | `-add-module` | Add a module to an existing service |
| 3 | `-add-handler` | Add a delivery/worker handler to an existing module |
| 4 | `-init-monorepo` | Initialize the monorepo skeleton (once, already done for this repo) |
| 5 | `-run` | Run one or more services locally (`-service=user,order`) |

`-init`/`-add-module`/`-add-handler` auto-detect that the current directory is
a monorepo (it checks for `sdk/` and `services/` folders) — **always `cd` to
the repo root first**, don't pass `-output`. `-add-module`/`-add-handler`
accept `-service=<name>` to pick the target service without an extra prompt.

### Creating each of the 7 services

Run from the **repo root**, once per service — the CLI is interactive, so
answer its prompts (service name, module name(s), server handlers, worker
handlers, dependencies/database, license) based on the responsibility table
and the handler guidance below:

```bash
candi -init      # prompts: service name -> module name(s) -> server handlers
                 # (REST/GRPC/GraphQL) -> [REST library if REST chosen] ->
                 # worker handlers -> dependencies (Redis/SQL/Mongo/Arango) ->
                 # [SQL driver + GORM y/n if SQL chosen] -> use license? (y/n)
```

Run it 7 times (once per service). It always creates `services/<service name
you typed>/` — there's no separate output flag to set.

### Adding a module or handler later

```bash
candi -add-module -service=order     # add another module inside the order service
candi -add-handler -service=order    # add a new delivery/worker handler to an existing module in order
```

**Never** hand-write the `domain/usecase/repository/delivery` skeleton for a new
module — always run `-add-module`/`-add-handler` so generated boilerplate
(interfaces, mocks, DI wiring, module registration) stays in sync with the rest
of the codebase.

## Generated service structure (per service)

```
services/<name>/
├── main.go                        # entrypoint, wires ServiceFactory (NOT under cmd/<name>/)
├── candi.json                     # generator metadata (enabled handlers, deps, package prefix, module list) — regenerated by the CLI, don't hand-edit
├── configs/                       # app_factory.go (DI wiring) + configs.go (env loading, DB/broker/tracer setup)
├── cmd/migration/                 # DB migration runner (relevant only if SQL/Mongo deps enabled)
├── internal/
│   ├── service.go                 # implements factory.ServiceFactory, registers all modules
│   └── modules/<module>/
│       ├── domain/                # const.go, payload.go, filter.go, request.go, response.go
│       ├── repository/            # interface + impl + mocks
│       ├── usecase/                # one file per usecase (e.g. get_all_<x>.go, create_<x>.go) + _test.go per file, mocks
│       ├── delivery/
│       │   ├── resthandler/       # only if REST enabled
│       │   ├── grpchandler/       # only if gRPC enabled
│       │   ├── graphqlhandler/    # only if GraphQL enabled (root/query/mutation/subscription resolvers)
│       │   └── workerhandler/     # only if a worker handler enabled (kafka/cron/redissubscriber/taskqueue/postgres/rabbitmq)
│       └── module.go              # implements factory.ModuleFactory, wires domain/usecase/repo/delivery together
├── pkg/
│   ├── shared/
│   │   ├── domain/                # entities/types shared by 2+ modules in this service
│   │   ├── repository/            # shared repository (e.g. shared DB/cache connection wrapper)
│   │   ├── usecase/                # + usecase/common/ for shared usecase-level helper logic
│   │   ├── middleware_impl_example.go
│   │   └── environment.go
│   └── helper/                    # generic, stateless helper functions for this service
├── api/
│   ├── api.go
│   ├── graphql/*.graphql          # only if GraphQL enabled
│   ├── proto/<name>/*.proto       # only if gRPC enabled
│   └── jsonschema/<name>/*.json
├── deployments/k8s/<name>.yaml
├── docs/
├── .env / .env.sample
├── .gitignore
├── Makefile                       # per-service targets: build/run/test/docker/migration/mocks/proto (each already scoped to this service)
└── README.md
```

No per-service `go.mod`, `go.sum`, or `Dockerfile` — those live once at the
monorepo root (see "Monorepo layout" above).

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

## Shared code & helpers

Candi already scaffolds dedicated places for reusable code — don't duplicate a
function in two modules/services when one of these fits. Pick the narrowest
scope that's true today; widen later if a second consumer actually shows up:

| Reusable across...                          | Put it in                                   |
|----------------------------------------------|----------------------------------------------|
| Two or more modules in the **same** service   | `services/<name>/pkg/shared/` (`domain/`, `repository/`, `usecase/common/`) |
| One-off stateless helper used in one service | `services/<name>/pkg/helper/` |
| Multiple **services** (e.g. money formatting, common domain enums, shared middleware) | `globalshared/` (monorepo root) |
| Another service needs to call this service directly (gRPC/REST client wrapper) | `sdk/` (monorepo root) — generate/expose the client here so callers never hand-roll their own client |

Rules:
- `pkg/shared` and `globalshared` are generated targets, not free-form dumping
  grounds — only move code there once it's actually needed by a second
  consumer (module/service); resist promoting something "just in case".
- A service's `pkg/` and `globalshared` code must stay framework/business-logic
  free where possible — pure helpers, shared types, and cross-cutting
  concerns (tracing, middleware) only. Domain-specific business rules stay in
  that module's own `usecase`.
- If `order` needs data from `payment` synchronously, generate/use `payment`'s
  client in `sdk/payment` rather than importing `payment`'s internal packages
  directly — services must never import another service's `internal/`.
- When you add something to `globalshared` or `sdk`, every service that could
  reasonably reuse it should switch to the shared version in the same PR (or a
  fast follow) — don't leave stale duplicates behind.

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
- Keep event payload contracts (proto/JSON schema) in `globalshared/` if/when
  duplication across services becomes a problem — don't create that
  abstraction preemptively before at least two services need it.

## Running services locally

```bash
candi -run -service=user,order,payment      # run a subset
candi -run                                   # run all services in the monorepo
```

Or use the root `Makefile` for a single service: `make run service=order`
(from the repo root — per-service Makefiles under `services/<name>/` also work
for that service alone since their targets aren't parameterized).

Local infra (Kafka, Redis, Postgres/Mongo, Jaeger) needs its own
`docker-compose.yml` (not generated by candi) — add one at the repo root
before relying on `make run`/`candi -run` against real brokers/databases.

## Conventions

- Follow candi's clean-architecture separation strictly: `delivery` layer only
  parses input/calls usecase; business logic lives in `usecase`; data access
  lives in `repository`; `domain` has no framework/behavior — data shapes only.
- Every usecase/repository interface gets a generated mock (via candi's
  mockgen setup) — regenerate mocks after changing an interface, don't hand-edit them.
- Config comes from environment variables loaded in `configs/` (via `.env` /
  `.env.sample`), never hardcoded.
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
- Before writing a helper function, check `pkg/helper` and `pkg/shared` in the
  current service, then `globalshared` at the repo root — don't paste a
  copy of a function that already exists in one of those. See "Shared code &
  helpers" below for where new reusable code belongs.

## Commit/PR scope

Each PR should generally touch one service (or the shared `globalshared/`/`sdk/`/
root tooling). Avoid cross-cutting PRs that touch multiple services' business
logic at once unless the change is mechanical (e.g. a shared contract update).
