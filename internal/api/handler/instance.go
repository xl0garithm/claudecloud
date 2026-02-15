package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/logan/cloudcode/internal/api/response"
	"github.com/logan/cloudcode/internal/provider"
	"github.com/logan/cloudcode/internal/service"
)

// InstanceHandler holds handlers for instance CRUD operations.
type InstanceHandler struct {
	svc *service.InstanceService
}

// NewInstanceHandler creates a new InstanceHandler.
func NewInstanceHandler(svc *service.InstanceService) *InstanceHandler {
	return &InstanceHandler{svc: svc}
}

type createRequest struct {
	UserID int `json:"user_id"`
}

// Create handles POST /instances.
func (h *InstanceHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.UserID <= 0 {
		response.Error(w, http.StatusBadRequest, "user_id is required")
		return
	}

	inst, err := h.svc.Create(r.Context(), req.UserID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	response.JSON(w, http.StatusCreated, inst)
}

// Get handles GET /instances/{id}.
func (h *InstanceHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := service.ParseID(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid instance ID")
		return
	}

	inst, err := h.svc.Get(r.Context(), id)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, inst)
}

// Delete handles DELETE /instances/{id}.
func (h *InstanceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := service.ParseID(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid instance ID")
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		handleServiceError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"status": "destroyed"})
}

// Pause handles POST /instances/{id}/pause.
func (h *InstanceHandler) Pause(w http.ResponseWriter, r *http.Request) {
	id, err := service.ParseID(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid instance ID")
		return
	}

	if err := h.svc.Pause(r.Context(), id); err != nil {
		handleServiceError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

// Wake handles POST /instances/{id}/wake.
func (h *InstanceHandler) Wake(w http.ResponseWriter, r *http.Request) {
	id, err := service.ParseID(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid instance ID")
		return
	}

	if err := h.svc.Wake(r.Context(), id); err != nil {
		handleServiceError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"status": "running"})
}

func handleServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, provider.ErrNotFound):
		response.Error(w, http.StatusNotFound, "instance not found")
	case errors.Is(err, provider.ErrAlreadyExists):
		response.Error(w, http.StatusConflict, "instance already exists for user")
	case errors.Is(err, provider.ErrInvalidState):
		response.Error(w, http.StatusConflict, "invalid instance state for operation")
	default:
		response.Error(w, http.StatusInternalServerError, "internal error")
	}
}
