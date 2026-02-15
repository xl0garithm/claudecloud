package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/logan/cloudcode/internal/auth"
	"github.com/logan/cloudcode/internal/service"
)

// ConnectHandler serves the connect script endpoint.
type ConnectHandler struct {
	svc       *service.InstanceService
	jwtSecret string
}

// NewConnectHandler creates a new ConnectHandler.
func NewConnectHandler(svc *service.InstanceService, jwtSecret string) *ConnectHandler {
	return &ConnectHandler{svc: svc, jwtSecret: jwtSecret}
}

// ServeScript handles GET /connect.sh.
// Supports Bearer JWT, session cookie, or ?user_id parameter.
// Returns a shell script that connects to the user's running instance.
func (h *ConnectHandler) ServeScript(w http.ResponseWriter, r *http.Request) {
	var userID int

	// Try Bearer JWT auth
	if authHeader := r.Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		if claims, err := auth.ValidateToken(h.jwtSecret, tokenStr); err == nil && claims.Purpose == "session" {
			userID = claims.UserID
		}
	}

	// Try session cookie
	if userID == 0 {
		if cookie, err := r.Cookie("session"); err == nil {
			if claims, err := auth.ValidateToken(h.jwtSecret, cookie.Value); err == nil && claims.Purpose == "session" {
				userID = claims.UserID
			}
		}
	}

	// Fall back to ?user_id parameter
	if userID == 0 {
		userIDStr := r.URL.Query().Get("user_id")
		if userIDStr == "" {
			writeErrorScript(w, http.StatusBadRequest, "missing authentication or user_id parameter")
			return
		}
		var err error
		userID, err = strconv.Atoi(userIDStr)
		if err != nil || userID <= 0 {
			writeErrorScript(w, http.StatusBadRequest, "invalid user_id parameter")
			return
		}
	}

	info, err := h.svc.GetConnectInfo(r.Context(), userID)
	if err != nil {
		writeErrorScript(w, http.StatusNotFound, "no running instance found for this user")
		return
	}

	var script string
	switch info.Provider {
	case "docker", "mock":
		script = dockerConnectScript(info.UserID)
	case "hetzner":
		script = hetznerConnectScript(info)
	default:
		writeErrorScript(w, http.StatusInternalServerError, "unknown provider")
		return
	}

	w.Header().Set("Content-Type", "text/x-shellscript")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, script)
}

func dockerConnectScript(userID int) string {
	return fmt.Sprintf(`#!/bin/bash
set -e

echo "Connecting to Claude instance (Docker)..."
exec docker exec -it claude-%d zellij attach claude
`, userID)
}

func hetznerConnectScript(info *service.ConnectInfo) string {
	return fmt.Sprintf(`#!/bin/bash
set -e

INSTANCE_IP="%s"

echo "Connecting to Claude instance (Hetzner)..."

# Check if Netbird is installed
if ! command -v netbird &>/dev/null; then
    echo "Installing Netbird client..."
    curl -fsSL https://pkgs.netbird.io/install.sh | bash
fi

# Ensure Netbird is connected
if ! netbird status 2>/dev/null | grep -q "Connected"; then
    echo "Starting Netbird..."
    sudo netbird up
    sleep 2
fi

# Check if mosh is installed
if ! command -v mosh &>/dev/null; then
    echo "Installing mosh..."
    if command -v apt-get &>/dev/null; then
        sudo apt-get update && sudo apt-get install -y mosh
    elif command -v brew &>/dev/null; then
        brew install mosh
    else
        echo "Error: please install mosh manually"
        exit 1
    fi
fi

echo "Connecting via mosh to $INSTANCE_IP..."
exec mosh claude@"$INSTANCE_IP" -- zellij attach claude
`, info.Host)
}

func writeErrorScript(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "text/x-shellscript")
	w.WriteHeader(status)
	fmt.Fprintf(w, `#!/bin/bash
echo "Error: %s"
exit 1
`, msg)
}
