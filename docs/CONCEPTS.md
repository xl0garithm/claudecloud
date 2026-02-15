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
