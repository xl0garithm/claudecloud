package handler

import (
	"net/http"

	"github.com/logan/cloudcode/internal/api/response"
)

// InstanceHandler holds handlers for instance CRUD operations.
// Wired to the service layer in Batch 4.
type InstanceHandler struct{}

// NewInstanceHandler creates a new InstanceHandler.
func NewInstanceHandler() *InstanceHandler {
	return &InstanceHandler{}
}

// Create handles POST /instances.
func (h *InstanceHandler) Create(w http.ResponseWriter, r *http.Request) {
	response.Error(w, http.StatusNotImplemented, "not implemented")
}

// Get handles GET /instances/{id}.
func (h *InstanceHandler) Get(w http.ResponseWriter, r *http.Request) {
	response.Error(w, http.StatusNotImplemented, "not implemented")
}

// Delete handles DELETE /instances/{id}.
func (h *InstanceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	response.Error(w, http.StatusNotImplemented, "not implemented")
}

// Pause handles POST /instances/{id}/pause.
func (h *InstanceHandler) Pause(w http.ResponseWriter, r *http.Request) {
	response.Error(w, http.StatusNotImplemented, "not implemented")
}

// Wake handles POST /instances/{id}/wake.
func (h *InstanceHandler) Wake(w http.ResponseWriter, r *http.Request) {
	response.Error(w, http.StatusNotImplemented, "not implemented")
}
