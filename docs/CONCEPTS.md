# ClaudeCloud — Implemented Concepts

Educational documentation of the patterns and design decisions implemented in each phase.

---

## Phase 1: Foundations & Infrastructure

### Provider Abstraction (Strategy Pattern)

**Pattern**: Define a `Provisioner` interface so the compute backend is swappable via config.

The core insight is that local Docker and production Hetzner have identical lifecycle semantics (create, pause, wake, destroy) but completely different implementations. The `Provisioner` interface in `internal/provider/provider.go` defines five methods. A factory function in `internal/provider/factory/` reads `PROVIDER` from config and returns the appropriate implementation.

This means every handler, service, and test is provider-agnostic. The same `POST /instances` creates a Docker container locally or a Hetzner VPS in production. Tests use a `MockProvisioner` (in-memory map) that exercises the full service layer without any infrastructure.

The factory lives in its own sub-package (`provider/factory`) to avoid an import cycle — both Docker and Hetzner providers import the parent `provider` package for the interface and error types, so the parent can't also import them.

### Ent ORM (Schema-as-Code)

**Pattern**: Define database schemas as Go structs; code generation produces type-safe queries.

Ent generates a full query builder from schema definitions in `internal/ent/schema/`. The `User` schema has email and api_key fields with unique constraints; the `Instance` schema has provider, provider_id, status, and an owner edge back to User.

On startup, `db.Schema.Create()` auto-migrates the database. This is safe for development and early production — no separate migration files needed. The generated code provides type-safe builders like `client.Instance.Create().SetProvider("docker").SetOwnerID(1).Save(ctx)`.

### Service Layer (Business Logic Bridge)

**Pattern**: A service struct sits between HTTP handlers and infrastructure (provider + DB).

`InstanceService` in `internal/service/instance.go` encapsulates all business rules: checking for duplicate instances, calling the provider, persisting to the database, and mapping provider errors to API responses. Handlers only parse HTTP requests and call service methods.

State transition validation happens here — you can only pause a running instance, only wake a stopped one. If the provider call succeeds but the DB write fails, the service does best-effort cleanup. This keeps handlers thin and testable.

### Docker Provider (Container Lifecycle)

**Pattern**: Map user lifecycle operations to Docker container operations via the Docker SDK.

The Docker provider uses deterministic naming: `claude-{userID}` for containers, `claude-data-{userID}` for volumes. Create ensures the `claude-net` bridge network exists, creates a named volume, then starts a container from the `claude-instance` image. Pause maps to `docker stop`, Wake to `docker start`, Destroy removes the container but keeps the volume.

The `claude-instance` image is Ubuntu 24.04 with Node.js, Claude Code, and Zellij pre-installed. The entrypoint starts a detached Zellij session. Claude Code is configured with `dangerouslyApproveAll: true` for autonomous operation.

### Hetzner Provider (Terraform-exec)

**Pattern**: Use terraform-exec to manage per-user Terraform workspaces for isolated infrastructure state.

Each user gets a directory under `terraform/workspaces/user-{id}/` with their own Terraform state file. Create writes a `main.tf` referencing the shared `modules/user_instance` module, runs `tf.Init` + `tf.Apply`, and reads outputs (server ID, IP, volume ID).

The user_instance module provisions a Hetzner server with cloud-init (Node.js, Claude Code, Zellij), a persistent 20GB volume, and dynamic subnet assignment (`10.100.{userID % 250 + 1}.10`). Pause destroys the server (volume persists via Terraform lifecycle rules); Wake re-applies to recreate it.

### API Key Auth Middleware

**Pattern**: Simple static API key validation via `X-API-Key` header for Phase 1.

The `APIKeyAuth` middleware in `internal/api/middleware/auth.go` compares the header value against the configured key. Missing key returns 401 "missing API key"; wrong key returns 401 "invalid API key". This is intentionally simple — Clerk/OAuth is added in Phase 3 when the dashboard ships.

The health endpoint (`/healthz`) is excluded from auth so monitoring tools can check liveness without credentials.

---

## Phase 2: Core Provisioning & Zero-Trust

### Netbird HTTP Client (Thin REST Wrapper)

**Pattern**: Wrap an external REST API in a typed Go client with a single `do()` helper.

The `internal/netbird/` package provides a thin client for the Netbird Management API. A single `do()` method handles JSON marshaling/unmarshaling, authorization headers, and error response parsing. Each resource (groups, setup keys, routes, policies) gets its own file with CRUD methods that delegate to `do()`.

The `APIError` type implements the `error` interface and carries the HTTP status code, so callers can distinguish between 403 (auth issue) and 404 (not found) without parsing strings. All methods accept a `context.Context` for cancellation and timeouts.

### Two-Phase Netbird Provisioning

**Pattern**: Split network setup into "before server" and "after server" phases because cloud-init needs the setup key at boot time.

The `NetbirdService` in `internal/service/netbird.go` orchestrates zero-trust networking in two phases:

1. **PrepareNetbirdAccess**: Creates a peer group and one-off setup key *before* the server boots. The setup key is passed to cloud-init via Terraform variables, so the instance auto-enrolls into Netbird on first boot.

2. **FinalizeNetbirdAccess**: After the server is up, creates a route (so the user's Netbird peer can reach the instance subnet) and a policy (allowing bidirectional traffic within the user's group).

Both phases include rollback logic — if setup key creation fails, the already-created group is deleted. If policy creation fails, the route is cleaned up. `TeardownUser` deletes resources in reverse order (policy, route, group).

### CreateOptions and Provider Interface Extension

**Pattern**: Add optional parameters to the `Provisioner.Create` signature via an options struct.

The `CreateOptions` struct carries provider-specific parameters (like `NetbirdSetupKey`) without polluting the interface with provider-specific arguments. Docker ignores the options; Hetzner passes the setup key through Terraform variables to cloud-init. This keeps the interface clean while allowing provider-specific behavior.

The `Activity` method was also added to the interface for idle detection. Docker checks process count via `ContainerTop`; Hetzner checks server running status. The mock provider returns configurable results for testing.

### Shared Bootstrap Scripts

**Pattern**: Single-source setup scripts used by both Docker images and cloud-init templates.

The `scripts/instance/` directory contains idempotent scripts that work on both Docker (via `COPY` + `RUN`) and Hetzner (via cloud-init `write_files` + `runcmd`). The `setup.sh` script checks for each tool before installing it, so it's safe to run multiple times. The Zellij layout (`claude-layout.kdl`) defines a 70/30 split between a Claude pane and a shell pane.

The Dockerfile uses the repo root as build context (instead of `docker/`) so it can `COPY` scripts from `scripts/instance/`. Cloud-init embeds the same scripts inline via `write_files` blocks.

### Connect Script Endpoint

**Pattern**: Serve a provider-aware shell script that users can `curl | bash` to connect to their instance.

`GET /connect.sh?user_id={id}` returns a shell script customized for the user's provider. Docker mode generates `docker exec -it claude-{userID} zellij attach claude`. Hetzner mode generates a script that installs Netbird, connects to the mesh, and runs `mosh claude@{IP} -- zellij attach claude`.

Error cases return a valid shell script that echoes the error and exits 1, so `curl | bash` always gives user-visible feedback rather than silent failure.

### Activity Polling and Auto-Pause

**Pattern**: Background service polls running instances and auto-pauses idle ones to save resources.

The `ActivityService` runs on a configurable interval (default 5 minutes). For each running instance, it calls `provider.Activity()` to check if there's real user activity. If active, it updates `last_activity_at`. If inactive and the idle duration exceeds the threshold (default 2 hours), it auto-pauses the instance.

Docker determines activity by process count — more than 3 processes (entrypoint + tail + zellij) means a user is connected. Hetzner currently uses a simple running/not-running check. The `CronService` handles a separate concern: cleaning up expired Netbird setup keys every 30 minutes.

---

## Phase 3: MVP Dashboard & Billing

### JWT Authentication (Zero-Cost Auth)

**Pattern**: Issue JWTs from the Go backend directly — no external auth provider needed.

The `internal/auth/jwt.go` package provides `GenerateToken` and `ValidateToken` using HMAC-SHA256 signing. Tokens carry three custom claims: `user_id`, `email`, and `purpose` (either "session" or "magic_link"). This purpose field prevents token misuse — a magic link token can't be used as a session token and vice versa.

Magic link flow: `POST /auth/login` finds or creates a user by email, generates a short-lived magic link JWT (15 min), and sends it via email (or logs it in dev mode). `GET /auth/verify?token=` validates the magic link JWT, issues a 24-hour session JWT, and sets an HttpOnly cookie with `SameSite=Lax`. The cookie approach means the browser automatically includes auth on subsequent requests without client-side token management.

### Dual-Mode Auth Middleware

**Pattern**: A single middleware that supports both JWT (dashboard users) and X-API-Key (admin/backwards compat).

The `UserAuth` middleware in `internal/api/middleware/userauth.go` checks three sources in order: `Authorization: Bearer` header, `session` cookie, and `X-API-Key` header. JWT-authenticated requests get `user_id` and `email` in the context. API key requests get `is_admin` in the context. Context helpers (`UserIDFromContext`, `IsAdminContext`) let handlers branch on auth mode.

This is critical for the instance creation endpoint: JWT-authenticated users create instances for themselves (user_id from token). Admin API key users can specify any user_id in the request body, preserving backward compatibility with Phase 1-2 scripts.

### Services Struct (Router Refactor)

**Pattern**: Bundle service dependencies in a single struct instead of a growing parameter list.

The `Services` struct in `internal/api/server.go` holds `InstanceService`, `AuthService`, and `BillingService` (nil when Stripe isn't configured). The router function takes `(cfg, svcs)` instead of individual services. This keeps the function signature stable as new services are added.

Nil-safe service handling: billing routes are only registered when `svcs.Billing != nil`. This means the API works without Stripe configuration — billing endpoints simply don't exist in dev mode.

### Stripe Billing Integration

**Pattern**: Flat billing fields on User instead of a separate Subscription entity — MVP simplicity.

The `BillingService` in `internal/service/billing.go` handles the full Stripe lifecycle:

1. **Checkout**: Creates a Stripe customer (if needed), builds a Checkout Session with plan metadata (starter/pro), and returns the Stripe-hosted payment URL.

2. **Webhooks**: `POST /billing/webhook` verifies the Stripe signature and dispatches events:
   - `checkout.session.completed`: Activates subscription, auto-provisions instance
   - `customer.subscription.updated`: Syncs status changes
   - `customer.subscription.deleted`: Marks canceled, pauses running instance
   - `invoice.payment_failed`: Marks past_due

3. **Portal**: Creates a Stripe Billing Portal session for self-service management (cancel, update payment method, view invoices).

User schema carries `stripe_customer_id`, `stripe_subscription_id`, `subscription_status`, `plan`, and `usage_hours` directly — no joins needed for billing checks.

### Usage Metering

**Pattern**: Piggyback on the existing activity polling loop to track usage hours.

The `UsageTracker` in `internal/service/usage.go` hooks into the `ActivityService` via a callback (`SetOnActive`). Every time the activity checker detects an active instance, it calls `RecordActive`, which adds the polling interval (in hours) to the user's `usage_hours` field. If the activity interval is 5 minutes and an instance is active for an hour, 12 callbacks fire and add `12 * (5/60) = 1.0` hour.

This is intentionally approximate — it's good enough for billing and avoids the complexity of precise session tracking. The usage field accumulates monotonically; billing period resets would be handled by a future cron job.

### Next.js Dashboard

**Pattern**: Minimal Next.js 14 with App Router — server-side rendering not needed for an authenticated SPA.

The dashboard at `web/` is a client-rendered React app that talks to the Go API via `credentials: "include"` fetch calls (so the HttpOnly session cookie flows automatically). Key pages:

- **Landing** (`/`): Hero + two-tier pricing cards
- **Login** (`/auth/login`): Email form → "check your email" confirmation
- **Verify** (`/auth/verify`): Token validation → redirect to dashboard
- **Dashboard** (`/dashboard`): Plan status, usage hours, instance card with wake/pause buttons, connect command

The dashboard layout includes an auth guard that redirects to login if `/auth/me` fails. The API client (`lib/api.ts`) is a typed wrapper around fetch with error extraction from JSON responses.

### CLI Connect Tool

**Pattern**: Bash CLI that stores a session JWT and uses it for authenticated API calls.

The `scripts/claude-cloud` script provides `login`, `verify`, `connect`, `status`, and `logout` commands. After magic link verification, the session JWT is stored in `~/.claude-cloud/token`. The `connect` command sends the stored token as a `Bearer` header to `/connect.sh`, which returns a provider-specific connection script.

The connect endpoint (`/connect.sh`) was upgraded to support three auth methods: Bearer JWT, session cookie, and `?user_id` parameter (legacy). This means the CLI, the browser dashboard, and the original curl-based flow all work with the same endpoint.

---

## Phase 4: UI Layer — Web Terminal, Chat UI, Projects

### Instance Agent (Node.js Sidecar)

**Pattern**: Run a lightweight Node.js server on each instance to expose chat, file, and project APIs.

Each instance runs an Express + WebSocket server on port 3001 (`scripts/instance/agent/`). It provides:
- `WS /chat` — Claude Code SDK streaming via `@anthropic-ai/claude-code`
- `GET /files` — Directory listing within `/claude-data/`
- `GET /files/read` — File content with 1MB truncation
- `GET /projects` — Scans for `.git` directories up to 3 levels deep
- `POST /projects/clone` — Runs `git clone` in `/claude-data/`

Auth uses a per-instance random secret (`AGENT_SECRET` env var). The Go API stores this secret in the database and includes it when proxying requests. All file operations are restricted to `/claude-data/` via path validation (`safePath` function that resolves and checks the prefix).

### ttyd Web Terminal

**Pattern**: Use ttyd (a proven terminal-over-WebSocket tool) for browser-based shell access.

ttyd runs on each instance at port 7681, wrapping `zellij attach claude --create`. It uses a binary WebSocket protocol where the first byte indicates message type: 0 = terminal output/input, 1 = resize, 2 = preferences. The Go API proxies the WebSocket connection to ttyd, and the frontend uses xterm.js to render the terminal.

The xterm.js component (`WebTerminal.tsx`) handles the ttyd protocol encoding/decoding, auto-fits to the container size, and provides connection status display with a reconnect button.

### WebSocket Proxy (Go API)

**Pattern**: Centralized JWT auth for all instance access — the Go API proxies WebSocket and HTTP connections.

The `ProxyHandler` in `internal/api/handler/proxy.go` handles six proxy routes, all under `/instances/{id}/`. For WebSocket endpoints (terminal, chat), it upgrades the client connection, verifies JWT auth and instance ownership, then opens a backend WebSocket and bidirectionally copies messages. For HTTP endpoints (files, projects), it uses `httputil.ReverseProxy`.

JWT auth for WebSocket connections uses a `?token=` query parameter fallback since browsers can't set custom headers on WebSocket upgrade requests. The `extractUserID` method checks both the context (from middleware) and the query parameter.

Instance ownership verification happens in `GetInstanceHost`, which queries for an instance matching both the ID and the requesting user's ID. This prevents users from accessing other users' instances.

### Agent SDK Chat Integration

**Pattern**: Wrap the Claude Code Agent SDK in a streaming WebSocket interface.

The `chat.js` module uses `@anthropic-ai/claude-code`'s `query()` function as an async generator. Each response event (text, tool_use, tool_result, done) is mapped to a JSON message and sent over the WebSocket. The client accumulates text chunks into a single assistant message and tracks tool events in a separate array.

The chat page (`chat/page.tsx`) manages WebSocket state, message history, and streaming state. It uses refs for accumulating in-flight text and tool events to avoid stale closure issues in the WebSocket message handler. A `Suspense` boundary wraps the page because `useSearchParams()` requires it in Next.js App Router.

### Dashboard Navigation (Tab-Based Layout)

**Pattern**: Add navigation tabs to the dashboard layout for multi-page navigation.

The dashboard layout (`app/dashboard/layout.tsx`) renders four tabs: Overview, Terminal, Chat, and Projects. Active tab detection uses `usePathname()` — the Overview tab is exact-match, others use `startsWith`. Each tab links to a sub-route under `/dashboard/`.

### Projects as Filesystem (No Separate Entity)

**Pattern**: The instance filesystem is the source of truth for projects — no database entity needed.

The instance agent scans `/claude-data/` for `.git` directories (up to 3 levels deep) and reads the git remote URL from `.git/config`. Each project is returned as `{ name, path, remoteUrl }`. The "Open in Chat" button navigates to `/dashboard/chat?cwd={path}`, which sets the Claude Code working directory.

This approach avoids syncing project metadata between the database and filesystem. The instance always reports its current state, and clone operations are idempotent (git clone will fail if the directory already exists, giving the user a clear error).

---

## Phase 5: Reliability, Security & Launch

### Structured Logging (slog Migration)

**Pattern**: Replace stdlib `log.Logger` with Go 1.21's `log/slog` for structured, leveled logging.

Every service that previously accepted `*log.Logger` was migrated to `*slog.Logger`. In production (`ENVIRONMENT=production`), the handler is `slog.NewJSONHandler` for machine-parsable output. In development, `slog.NewTextHandler` produces human-readable key=value lines.

Structured fields are passed as typed key-value pairs: `logger.Info("instance created", "instance_id", id, "user_id", uid)`. This eliminates string formatting and makes log aggregation trivial — you can filter by `instance_id` or `user_id` without regex. The logger propagates from `main.go` through service constructors, so every component shares the same handler configuration.

### Security Headers Middleware

**Pattern**: Set defensive HTTP headers on every response via a global middleware.

The `Security` middleware in `internal/api/middleware/security.go` adds six headers to every response: `X-Content-Type-Options: nosniff` (prevents MIME sniffing), `X-Frame-Options: DENY` (prevents clickjacking), `Referrer-Policy: strict-origin-when-cross-origin`, `X-XSS-Protection: 0` (modern browsers handle this natively), and `Permissions-Policy: camera=(), microphone=()`. When the `BASE_URL` starts with `https`, it also sets `Strict-Transport-Security: max-age=31536000; includeSubDomains` to enforce HTTPS. The HSTS check is conditional because adding it in development (HTTP) would cause browsers to refuse non-HTTPS connections.

### Rate Limiting (Token Bucket)

**Pattern**: Per-IP token bucket rate limiting using `golang.org/x/time/rate` — no Redis needed at this scale.

The `RateLimit` middleware creates a per-IP `*rate.Limiter` stored in a `sync.Map`. Each limiter allows a configurable rate (tokens per second) with a burst capacity. Two tiers are applied: auth endpoints (`/auth/login`) get 5 requests/minute (prevents brute force), while authenticated API routes get 60 requests/minute. A background goroutine cleans up stale entries every 3 minutes (any IP not seen for 5 minutes is evicted). When a request exceeds the rate, a `429 Too Many Requests` response is returned with a `Retry-After` header.

This approach is appropriate for the target of 50 users. A distributed rate limiter (Redis-backed) would be needed at higher scale, but the in-memory approach avoids an infrastructure dependency.

### Config Validation (Fail-Fast)

**Pattern**: Validate critical configuration at startup and refuse to start if production config is incomplete.

The `Validate()` method on `Config` checks invariants only in production mode (`ENVIRONMENT=production`). It verifies that `JWT_SECRET` is not the default insecure value, `DATABASE_URL` is set, and `STRIPE_SECRET_KEY` is present (required for billing). Errors are collected and returned as a single multi-line error. In development mode, `Validate()` returns nil — insecure defaults are fine for local testing.

This prevents deploying a production instance with forgotten configuration. The check runs immediately after `config.Load()` and before any network listeners start, so the process exits cleanly with a descriptive error message.

### Process Supervision (Instance Reliability)

**Pattern**: Simple bash process supervisor with exponential backoff replaces raw backgrounding.

The `docker/supervisor.sh` script manages the three long-running processes inside each instance container: ttyd (web terminal), the Node.js agent, and Zellij. It starts each process, records its PID, then monitors every 5 seconds. If a process exits, it restarts with exponential backoff (1s, 2s, 4s, ... up to 30s), resetting the backoff on successful restart. This is simpler and lighter than supervisord, keeping the container image small.

The Docker instance image also gained a `HEALTHCHECK` directive that probes both ttyd (port 7681) and the agent (port 3001). The Docker provider's `Activity()` method now inspects the container health status — if the container reports "unhealthy", it's treated as inactive regardless of process count.

### Instance Health Monitoring

**Pattern**: Track consecutive health check failures to detect persistently degraded instances.

The `ActivityService` was extended to check agent reachability alongside its existing activity polling. A `sync.Map` tracks consecutive health failures per instance. After 3 consecutive failures, a warning is logged. The `ActivityInfo` struct gained an `IsHealthy` field that providers set based on container health status (Docker) or server running state (Hetzner).

### OpenTelemetry Tracing + Metrics

**Pattern**: Single OTEL SDK for both distributed tracing and Prometheus metrics.

The `internal/telemetry` package bootstraps both a `TracerProvider` and a `MeterProvider` from a single `Init()` call. The trace provider uses an OTLP HTTP exporter when `OTEL_EXPORTER_OTLP_ENDPOINT` is set; otherwise traces are discarded (dev mode). The meter provider uses a Prometheus exporter, exposing metrics at `/metrics` in Prometheus text format.

Three layers of instrumentation work together:

1. **Auto-instrumentation**: The `otelhttp` middleware wraps the Chi router, creating a span for every HTTP request with method, route, and status code attributes.

2. **Manual spans**: Key business operations — instance create/delete/pause/wake, billing checkout/webhook, proxy terminal/chat/files — create child spans with relevant attributes (user_id, instance_id, plan). Errors are recorded on the span via `span.RecordError()` and `span.SetStatus(codes.Error)`.

3. **Log correlation**: The `tracelog` middleware extracts `trace_id` and `span_id` from the OTEL span context and injects them into `slog` via `slog.With()`. All subsequent log lines within a request carry these fields, enabling log-to-trace correlation in observability tools.

Custom meters in the activity service track `cloudcode.instances.total` (UpDownCounter by status) and `cloudcode.instances.active` (UpDownCounter), updated each poll cycle. The W3C TraceContext propagator enables distributed tracing across service boundaries.
