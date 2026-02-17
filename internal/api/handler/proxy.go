package handler

import (
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/logan/cloudcode/internal/api/middleware"
	"github.com/logan/cloudcode/internal/api/response"
	"github.com/logan/cloudcode/internal/auth"
	"github.com/logan/cloudcode/internal/service"
)

var proxyTracer = otel.Tracer("cloudcode/handler/proxy")

// ProxyHandler proxies requests to instance ttyd and agent services.
type ProxyHandler struct {
	svc       *service.InstanceService
	jwtSecret string
}

// NewProxyHandler creates a new ProxyHandler.
func NewProxyHandler(svc *service.InstanceService, jwtSecret string) *ProxyHandler {
	return &ProxyHandler{svc: svc, jwtSecret: jwtSecret}
}

var upgrader = websocket.Upgrader{
	CheckOrigin:  func(r *http.Request) bool { return true },
	Subprotocols: []string{"tty"},
}

// extractUserID gets the user ID from context (middleware) or ?token= query param (WebSocket fallback).
func (h *ProxyHandler) extractUserID(r *http.Request) int {
	if uid := middleware.UserIDFromContext(r.Context()); uid != 0 {
		return uid
	}
	// WebSocket fallback: ?token=JWT
	if tok := r.URL.Query().Get("token"); tok != "" {
		claims, err := auth.ValidateToken(h.jwtSecret, tok)
		if err == nil && claims.Purpose == "session" {
			return claims.UserID
		}
	}
	return 0
}

// resolveInstance extracts the instance ID and verifies ownership, returning the host and agent secret.
func (h *ProxyHandler) resolveInstance(w http.ResponseWriter, r *http.Request) (host, agentSecret string, ok bool) {
	userID := h.extractUserID(r)
	if userID == 0 {
		response.Error(w, http.StatusUnauthorized, "authentication required")
		return "", "", false
	}

	id, err := service.ParseID(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid instance ID")
		return "", "", false
	}

	host, agentSecret, err = h.svc.GetInstanceHost(r.Context(), id, userID)
	if err != nil {
		handleServiceError(w, err)
		return "", "", false
	}
	return host, agentSecret, true
}

// Terminal proxies WebSocket connections to ttyd (port 7681).
func (h *ProxyHandler) Terminal(w http.ResponseWriter, r *http.Request) {
	_, span := proxyTracer.Start(r.Context(), "proxy.terminal")
	defer span.End()

	host, _, ok := h.resolveInstance(w, r)
	if !ok {
		return
	}
	span.SetAttributes(attribute.String("host", host))

	// Upgrade client connection
	clientConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer clientConn.Close()

	// Connect to ttyd — must negotiate the "tty" subprotocol
	targetURL := "ws://" + host + ":7681/ws"
	ttydDialer := websocket.Dialer{
		Subprotocols: []string{"tty"},
	}
	backendConn, _, err := ttydDialer.Dial(targetURL, nil)
	if err != nil {
		slog.Error("terminal proxy: backend dial failed", "host", host, "error", err)
		clientConn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "backend unavailable"))
		return
	}
	defer backendConn.Close()

	// Bidirectional proxy
	done := make(chan struct{}, 2)
	go func() {
		defer func() { done <- struct{}{} }()
		for {
			msgType, msg, err := backendConn.ReadMessage()
			if err != nil {
				slog.Debug("terminal proxy: backend→client read error", "host", host, "error", err)
				return
			}
			if err := clientConn.WriteMessage(msgType, msg); err != nil {
				slog.Debug("terminal proxy: backend→client write error", "host", host, "error", err)
				return
			}
		}
	}()
	go func() {
		defer func() { done <- struct{}{} }()
		for {
			msgType, msg, err := clientConn.ReadMessage()
			if err != nil {
				slog.Debug("terminal proxy: client→backend read error", "host", host, "error", err)
				return
			}
			if err := backendConn.WriteMessage(msgType, msg); err != nil {
				slog.Debug("terminal proxy: client→backend write error", "host", host, "error", err)
				return
			}
		}
	}()
	<-done
}

// Chat proxies WebSocket connections to the instance agent chat (port 3001).
func (h *ProxyHandler) Chat(w http.ResponseWriter, r *http.Request) {
	_, span := proxyTracer.Start(r.Context(), "proxy.chat")
	defer span.End()

	host, agentSecret, ok := h.resolveInstance(w, r)
	if !ok {
		return
	}
	span.SetAttributes(attribute.String("host", host))

	clientConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer clientConn.Close()

	targetURL := "ws://" + host + ":3001/chat?secret=" + agentSecret
	backendHeader := http.Header{}
	backendHeader.Set("Authorization", "Bearer "+agentSecret)
	backendConn, _, err := websocket.DefaultDialer.Dial(targetURL, backendHeader)
	if err != nil {
		slog.Error("chat proxy: backend dial failed", "host", host, "error", err)
		clientConn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "agent unavailable"))
		return
	}
	defer backendConn.Close()

	done := make(chan struct{}, 2)
	go func() {
		defer func() { done <- struct{}{} }()
		for {
			msgType, msg, err := backendConn.ReadMessage()
			if err != nil {
				return
			}
			if err := clientConn.WriteMessage(msgType, msg); err != nil {
				return
			}
		}
	}()
	go func() {
		defer func() { done <- struct{}{} }()
		for {
			msgType, msg, err := clientConn.ReadMessage()
			if err != nil {
				return
			}
			if err := backendConn.WriteMessage(msgType, msg); err != nil {
				return
			}
		}
	}()
	<-done
}

// Files proxies GET /instances/{id}/files to the agent.
func (h *ProxyHandler) Files(w http.ResponseWriter, r *http.Request) {
	h.proxyHTTP(w, r, "/files")
}

// FilesRead proxies GET /instances/{id}/files/read to the agent.
func (h *ProxyHandler) FilesRead(w http.ResponseWriter, r *http.Request) {
	h.proxyHTTP(w, r, "/files/read")
}

// Projects proxies GET /instances/{id}/projects to the agent.
func (h *ProxyHandler) Projects(w http.ResponseWriter, r *http.Request) {
	h.proxyHTTP(w, r, "/projects")
}

// ProjectsClone proxies POST /instances/{id}/projects/clone to the agent.
func (h *ProxyHandler) ProjectsClone(w http.ResponseWriter, r *http.Request) {
	h.proxyHTTP(w, r, "/projects/clone")
}

// Tabs proxies POST /instances/{id}/tabs to the agent.
func (h *ProxyHandler) Tabs(w http.ResponseWriter, r *http.Request) {
	h.proxyHTTP(w, r, "/tabs")
}

// Sessions proxies GET /instances/{id}/sessions to the agent.
func (h *ProxyHandler) Sessions(w http.ResponseWriter, r *http.Request) {
	h.proxyHTTP(w, r, "/sessions")
}

// SessionConversations proxies GET /instances/{id}/sessions/{project}/conversations to the agent.
func (h *ProxyHandler) SessionConversations(w http.ResponseWriter, r *http.Request) {
	project := chi.URLParam(r, "project")
	h.proxyHTTP(w, r, "/sessions/"+project+"/conversations")
}

// DeleteTab proxies DELETE /instances/{id}/tabs/{name} to the agent.
func (h *ProxyHandler) DeleteTab(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	h.proxyHTTP(w, r, "/tabs/"+name)
}

// AuthStatus proxies GET /instances/{id}/auth/status to the agent.
func (h *ProxyHandler) AuthStatus(w http.ResponseWriter, r *http.Request) {
	h.proxyHTTP(w, r, "/auth/status")
}

// proxyHTTP is a helper that reverse-proxies an HTTP request to the instance agent.
func (h *ProxyHandler) proxyHTTP(w http.ResponseWriter, r *http.Request, agentPath string) {
	_, span := proxyTracer.Start(r.Context(), "proxy.files")
	defer span.End()

	host, agentSecret, ok := h.resolveInstance(w, r)
	if !ok {
		return
	}
	span.SetAttributes(attribute.String("host", host), attribute.String("path", agentPath))

	target, _ := url.Parse("http://" + host + ":3001")
	proxy := httputil.NewSingleHostReverseProxy(target)

	// Rewrite the request
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.URL.Path = agentPath
		req.URL.RawQuery = r.URL.RawQuery
		req.Header.Set("Authorization", "Bearer "+agentSecret)
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		slog.Error("http proxy: backend error", "host", host, "path", agentPath, "error", err)
		response.Error(w, http.StatusBadGateway, "instance agent unavailable")
	}

	// Prevent response body from being closed prematurely with chunked encoding
	proxy.ModifyResponse = func(resp *http.Response) error {
		return nil
	}

	proxy.ServeHTTP(w, r)
}

// GetMine handles GET /instances/mine — returns the calling user's active instance.
func GetMine(svc *service.InstanceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.UserIDFromContext(r.Context())
		if userID == 0 {
			response.Error(w, http.StatusUnauthorized, "authentication required")
			return
		}

		inst, err := svc.GetByUserID(r.Context(), userID)
		if err != nil {
			handleServiceError(w, err)
			return
		}

		response.JSON(w, http.StatusOK, inst)
	}
}

