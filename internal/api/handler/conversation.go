package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/logan/cloudcode/internal/api/middleware"
	"github.com/logan/cloudcode/internal/api/response"
	"github.com/logan/cloudcode/internal/service"
)

// ConversationHandler holds handlers for chat conversation operations.
type ConversationHandler struct {
	svc *service.ConversationService
}

// NewConversationHandler creates a new ConversationHandler.
func NewConversationHandler(svc *service.ConversationService) *ConversationHandler {
	return &ConversationHandler{svc: svc}
}

// GetOrCreate handles GET /conversations?project=<path>
// Returns existing conversation for this project or creates a new one.
func (h *ConversationHandler) GetOrCreate(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == 0 {
		response.Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	projectPath := r.URL.Query().Get("project")

	conv, err := h.svc.GetOrCreateByProject(r.Context(), userID, projectPath)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, conv)
}

// List handles GET /conversations/list
// Returns all conversations for the current user.
func (h *ConversationHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == 0 {
		response.Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	convs, err := h.svc.ListByUser(r.Context(), userID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, convs)
}

// GetMessages handles GET /conversations/{id}/messages
func (h *ConversationHandler) GetMessages(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == 0 {
		response.Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	convID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid conversation id")
		return
	}

	msgs, err := h.svc.GetMessages(r.Context(), convID, userID)
	if err != nil {
		response.Error(w, http.StatusNotFound, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, msgs)
}

type addMessageRequest struct {
	Role       string  `json:"role"`
	Content    string  `json:"content"`
	ToolEvents *string `json:"tool_events,omitempty"`
}

// AddMessage handles POST /conversations/{id}/messages
func (h *ConversationHandler) AddMessage(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == 0 {
		response.Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	convID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid conversation id")
		return
	}

	var req addMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Role != "user" && req.Role != "assistant" {
		response.Error(w, http.StatusBadRequest, "role must be 'user' or 'assistant'")
		return
	}

	msg, err := h.svc.AddMessage(r.Context(), convID, userID, req.Role, req.Content, req.ToolEvents)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.JSON(w, http.StatusCreated, msg)
}

// Delete handles DELETE /conversations/{id}
func (h *ConversationHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == 0 {
		response.Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	convID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid conversation id")
		return
	}

	if err := h.svc.DeleteConversation(r.Context(), convID, userID); err != nil {
		response.Error(w, http.StatusNotFound, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
