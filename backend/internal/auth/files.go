package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strings"

	"example.com/uptime-app/backend/internal/uploads"
	"github.com/google/uuid"
)

const maxFileSize = 20 << 20

func uploadError(w http.ResponseWriter, err error) {
	if errors.Is(err, uploads.ErrTooLarge) {
		http.Error(w, "file too large", http.StatusRequestEntityTooLarge)
		return
	}
	http.Error(w, "invalid multipart request", http.StatusBadRequest)
}

// @Summary Upload a file
// @Description Files up to 20 MiB are publicly downloadable via the returned URL.
// @Tags files
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param file formData file true "File to upload"
// @Success 201 {object} fileResponse
// @Failure 400 {string} string "Invalid multipart request"
// @Failure 401 {string} string "Invalid credentials"
// @Failure 413 {string} string "File too large"
// @Failure 500 {string} string "Internal server error"
// @Router /files [post]
func (a *Controller) uploadFile(w http.ResponseWriter, r *http.Request) {
	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		respondError(w, ErrCredentials)
		return
	}
	if _, err := a.Service.VerifyAccess(parts[1]); err != nil {
		respondError(w, err)
		return
	}
	data, err := uploads.ReadFile(w, r, "file", maxFileSize)
	if err != nil {
		uploadError(w, err)
		return
	}
	name, err := uploads.Save(a.UploadDir, "", data)
	if err != nil {
		respondError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(struct {
		URL string `json:"url"`
	}{"/files/" + name})
}

// @Summary Download a file
// @Tags files
// @Produce application/octet-stream
// @Param name path string true "File UUID"
// @Success 200 {file} file
// @Failure 404 {string} string "File not found"
// @Router /files/{name} [get]
func (a *Controller) serveFile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if uuid.Validate(name) != nil {
		http.NotFound(w, r)
		return
	}
	// Generic uploads are downloads, so active content cannot execute in the API origin.
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, filepath.Join(a.UploadDir, name))
}
