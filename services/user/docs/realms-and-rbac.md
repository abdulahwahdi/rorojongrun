# User service — realms, login and RBAC

The `user` service authenticates every principal and decides every permission. This page is the
operating manual; the source of truth for request/response shapes is the swagger comments on the REST
handlers (`internal/modules/*/delivery/resthandler`) and `api/graphql/*.graphql`.

## Model

```
realm ─┬─ realm_keys      RSA signing keys (private half AES-GCM encrypted with KEY_ENCRYPTION_SECRET)
       ├─ users           unique (username|email|phone) per realm, bcrypt password, lockout
       ├─ clients         public (frontend apps) | confidential (services, own secret + service account)
       ├─ roles ──< role_permissions >── permissions (service, code, type api|ui)
       │   └──< user_roles >── users
       ├─ menus           tree per client, each node optionally tied to one permission
       └─ sessions        one row per refresh token, rotated within a family (family id = JWT "sid")
```

Everything is scoped by realm; the same email may exist in two realms as two accounts. Deletes are soft
(`deleted_at`), except signing keys, role↔permission and user↔role links.

## First start (seeder)

On startup `internal/bootstrap` makes sure — idempotently — that there is:

| What | Detail |
|---|---|
| realm `master` | with an active signing key and a `realm-admin` role |
| permission `*:*` + role `superadmin` | wildcard: every service, every code |
| first superadmin | only when the master realm has **no user at all**; from `BOOTSTRAP_ADMIN_USERNAME` / `BOOTSTRAP_ADMIN_PASSWORD` (min 8 chars). Without a password nothing is created and a hint is logged. Restarts never touch an existing admin. |
| client `admin-cli` | public, grants `password`, `refresh_token` |

Run the migrations first (`make migration service=user`), then start the service. Lost the admin
password? Another superadmin resets it (`PUT /v1/realms/master/users/{id}/password`).

```bash
curl -s localhost:8000/v1/realms/master/auth/token -H 'content-type: application/json' \
  -d '{"grantType":"password","clientId":"admin-cli","username":"admin","password":"..."}'
```

## Who may administer what

* Every admin API is `Bearer` + a permission code (`HTTPPermissionACL("createUser")`, …) checked against the
  **caller's own realm**.
* The target realm in the path must be the caller's realm, or the caller must be in `master`.
* Creating / listing / deleting **realms** is master-only. The `master` realm cannot be deleted or disabled.
* Each new realm gets a `realm-admin` role (`user:*` = all APIs of this service inside that realm). Give it
  to the people who run that realm.

## Login

`POST /v1/realms/{realm}/auth/token`

| grantType | fields | notes |
|---|---|---|
| `password` | clientId, username (or email), password | wrong password counts towards the realm's lockout (`maxFailedAttempts` → locked `lockoutSec`, HTTP 423) |
| `refresh_token` | clientId, refreshToken | rotates; presenting an already used token revokes the whole login (theft detection) |
| `client_credentials` | clientId, clientSecret | confidential clients only; no refresh token, `typ=service` |
| `otp` | clientId, email, code | realm needs `otpLoginEnabled`; request a code with `POST .../auth/otp/request` (always 202); codes are issued and verified by the notification service |

Confidential clients must send `clientSecret` on every grant. A client only accepts the grants listed in
its `grantTypes`. `POST .../auth/logout` revokes the calling session; setting a password, disabling a user
or a client, and deleting a user/client/realm revoke sessions too.

## What a frontend needs

* `GET /v1/realms/{realm}/me` — profile and roles.
* `GET /v1/realms/{realm}/me/permissions?client=<clientId>` — all granted `(service, code, type)` plus the
  menu tree of that client, already filtered: nodes whose permission is not granted disappear with their
  subtree, and group nodes (no `path`) without visible children disappear.
* Menus are managed per client: `/v1/realms/{realm}/clients/{clientId}/menus` (`?tree=true` for the nested view).

## Enforcing permissions in another service

```go
// configs/configs.go of the service
sdk.SetGlobalSDK(sdk.SetUser(user.NewUserServiceGRPC(env.UserGRPCHost, env.UserBasicAuthKey)))

keys := auth.NewJWKSKeyProvider(env.UserHTTPHost)                  // http base url of the user service
validator := auth.NewTokenValidator(env.IssuerBaseURL, keys)       // same ISSUER_BASE_URL as the user service
acl := auth.NewACLChecker("notification", auth.NewSDKPermissionClient(sdk.GetSDK().User()), 30*time.Second)

deps.SetMiddleware(middleware.NewMiddlewareWithOption(
	middleware.SetTokenValidator(validator),
	middleware.SetACLPermissionChecker(acl),
	middleware.SetUserIDExtractor(func(c *candishared.TokenClaim) string { return c.Subject }),
))
```

Then register the service's permission codes in each realm that uses them (they are matched by
`(service, code)`, `*` is a wildcard):

```bash
curl -X POST .../v1/realms/backoffice/permissions/bulk -H "Authorization: Bearer $T" -d '{"permissions":[
  {"service":"notification","code":"sendNotification"},{"service":"notification","code":"getAllTemplates"}]}'
```

Revocation latency = the ACL cache TTL (5 s inside `user`, whatever you pass to `NewACLChecker` elsewhere).
Access tokens themselves live `accessTokenTtlSec` (default 15 min) and are only checked for signature/expiry
by other services; the session/permission state is checked on every (cached) permission decision.

## Configuration

| Env | Meaning |
|---|---|
| `ISSUER_BASE_URL` | public base url, part of every `iss`; must be identical wherever tokens are validated |
| `KEY_ENCRYPTION_SECRET` | encrypts signing keys at rest. **Changing it makes existing keys unreadable** — rotate keys after changing |
| `BOOTSTRAP_ADMIN_USERNAME` / `_PASSWORD` | first superadmin, see above |
| `NOTIFICATION_HOST` / `NOTIFICATION_AUTH_KEY` | OTP login; leave empty to disable |
| `BASIC_AUTH_USERNAME` / `BASIC_AUTH_PASS` | internal key of the gRPC authorization API (base64 of `user:pass` is what `NewUserServiceGRPC` takes) |

## Not covered yet

Rate limiting of the token endpoint (put it behind a gateway or add a limiter), self-registration, social
login / MFA beyond OTP, groups and composite roles, and moving the other services from
`shared.DefaultMiddleware` to `globalshared/auth` (one PR per service). While `notification` still uses its
default middleware, `NOTIFICATION_AUTH_KEY` can be any non-empty token; once it adopts `globalshared/auth`
the user service needs a service-account token for it.
