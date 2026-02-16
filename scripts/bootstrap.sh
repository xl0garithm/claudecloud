#!/usr/bin/env bash
set -euo pipefail

# ── CloudCode Local Bootstrap ──
# One script to go from fresh clone to running stack.
# Usage: ./scripts/bootstrap.sh [--no-e2e] [--rebuild]

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$ROOT_DIR"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

SKIP_E2E=false
FORCE_REBUILD=false

for arg in "$@"; do
  case "$arg" in
    --no-e2e)   SKIP_E2E=true ;;
    --rebuild)  FORCE_REBUILD=true ;;
  esac
done

log()  { echo -e "${CYAN}[bootstrap]${NC} $*"; }
ok()   { echo -e "${GREEN}  ✓${NC} $*"; }
warn() { echo -e "${YELLOW}  !${NC} $*"; }
fail() { echo -e "${RED}  ✗${NC} $*"; exit 1; }

# ── 1. Check prerequisites ──────────────────────────────────────────
log "Checking prerequisites..."

command -v docker >/dev/null 2>&1  || fail "docker not found. Install: https://docs.docker.com/get-docker/"
command -v docker compose >/dev/null 2>&1 || command -v docker-compose >/dev/null 2>&1 || fail "docker compose not found"

# Determine compose command
if docker compose version >/dev/null 2>&1; then
  COMPOSE="docker compose"
else
  COMPOSE="docker-compose"
fi

command -v go >/dev/null 2>&1      || fail "go not found. Install: https://go.dev/dl/"
command -v node >/dev/null 2>&1    || fail "node not found. Install: https://nodejs.org/"
command -v curl >/dev/null 2>&1    || fail "curl not found"

GO_VER=$(go version | grep -oP 'go\K[0-9]+\.[0-9]+')
NODE_VER=$(node -v | grep -oP 'v\K[0-9]+')

ok "docker $(docker --version | grep -oP '[0-9]+\.[0-9]+\.[0-9]+')"
ok "go $GO_VER"
ok "node v$NODE_VER"

# ── 2. Environment file ─────────────────────────────────────────────
log "Setting up environment..."

if [ ! -f .env ]; then
  cp .env.example .env
  ok "Created .env from .env.example"
else
  ok ".env already exists"
fi

# ── 3. Docker network ───────────────────────────────────────────────
log "Ensuring claude-net Docker network exists..."

if docker network inspect claude-net >/dev/null 2>&1; then
  ok "claude-net network exists"
else
  docker network create claude-net
  ok "Created claude-net network"
fi

# ── 4. Go dependencies ──────────────────────────────────────────────
log "Downloading Go dependencies..."
go mod download
ok "Go modules ready"

# ── 5. Unit tests ───────────────────────────────────────────────────
log "Running unit tests..."
if go test ./... -short -count=1 > /tmp/cloudcode-test.log 2>&1; then
  ok "All unit tests passed"
else
  warn "Some tests failed — check /tmp/cloudcode-test.log"
fi

# ── 6. Build Go binary (validates compilation) ──────────────────────
log "Building Go binary..."
make build 2>&1 | tail -1
ok "bin/cloudcode built"

# ── 7. Build instance image ─────────────────────────────────────────
log "Building claude-instance Docker image..."

if [ "$FORCE_REBUILD" = true ]; then
  docker build --no-cache -t claude-instance -f docker/Dockerfile.instance . 2>&1 | tail -5
elif docker image inspect claude-instance >/dev/null 2>&1 && [ "$FORCE_REBUILD" = false ]; then
  ok "claude-instance image already exists (use --rebuild to force)"
else
  docker build -t claude-instance -f docker/Dockerfile.instance . 2>&1 | tail -5
fi
ok "claude-instance image ready"

# ── 8. Install frontend dependencies ────────────────────────────────
log "Installing frontend dependencies..."
if [ ! -d web/node_modules ]; then
  (cd web && npm ci --silent)
  ok "npm packages installed"
else
  ok "node_modules already present"
fi

# ── 9. Start the stack ───────────────────────────────────────────────
log "Starting docker-compose stack..."

# Bring down any existing containers cleanly
$COMPOSE down --remove-orphans 2>/dev/null || true

$COMPOSE up --build -d 2>&1 | tail -5
ok "Containers starting"

# ── 10. Wait for services ───────────────────────────────────────────
log "Waiting for services to be ready..."

wait_for() {
  local name="$1" url="$2" max="$3"
  local i=0
  while [ $i -lt "$max" ]; do
    if curl -sf "$url" >/dev/null 2>&1; then
      ok "$name is up"
      return 0
    fi
    i=$((i + 1))
    sleep 2
  done
  fail "$name did not become ready after $((max * 2))s"
}

wait_for "API (healthz)" "http://localhost:8080/healthz" 30
wait_for "Frontend"      "http://localhost:3000"         20

# ── 11. Smoke tests ─────────────────────────────────────────────────
log "Running smoke tests..."

# Health check response
HEALTH=$(curl -sf http://localhost:8080/healthz)
echo "  healthz: $HEALTH"

# Security headers
HEADERS=$(curl -sI http://localhost:8080/healthz 2>&1)
if echo "$HEADERS" | grep -qi "x-content-type-options"; then
  ok "Security headers present"
else
  warn "Security headers missing"
fi

# Metrics endpoint
if curl -sf http://localhost:8080/metrics | head -1 | grep -q "^#"; then
  ok "/metrics returns Prometheus format"
else
  warn "/metrics not responding"
fi

# Rate limiting
echo -n "  Rate limit test: "
CODES=""
for i in $(seq 8); do
  CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST http://localhost:8080/auth/login \
    -H "Content-Type: application/json" -d '{"email":"test@test.com"}')
  CODES="$CODES $CODE"
done
if echo "$CODES" | grep -q "429"; then
  ok "Rate limiting working (saw 429)"
else
  warn "No 429 seen in: $CODES"
fi

# ── 12. E2E tests ───────────────────────────────────────────────────
if [ "$SKIP_E2E" = false ]; then
  log "Running E2E tests..."
  if bash scripts/e2e-test.sh; then
    ok "E2E tests passed"
  else
    warn "E2E tests failed (instance may need more startup time)"
  fi
else
  log "Skipping E2E tests (--no-e2e)"
fi

# ── Done ─────────────────────────────────────────────────────────────
echo ""
echo -e "${GREEN}══════════════════════════════════════════════════${NC}"
echo -e "${GREEN}  CloudCode is running!${NC}"
echo -e "${GREEN}══════════════════════════════════════════════════${NC}"
echo ""
echo -e "  Landing page:  ${CYAN}http://localhost:3000${NC}"
echo -e "  API:           ${CYAN}http://localhost:8080${NC}"
echo -e "  Health check:  ${CYAN}http://localhost:8080/healthz${NC}"
echo -e "  Metrics:       ${CYAN}http://localhost:8080/metrics${NC}"
echo ""
echo -e "  Logs:          ${YELLOW}$COMPOSE logs -f${NC}"
echo -e "  Stop:          ${YELLOW}$COMPOSE down${NC}"
echo -e "  Rebuild:       ${YELLOW}./scripts/bootstrap.sh --rebuild${NC}"
echo ""
