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
