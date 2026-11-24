package auth

import (
	"bytes"
	"encoding/json"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"example.com/uptime-app/backend/internal/storage"
	"example.com/uptime-app/backend/internal/uploads"
	"github.com/google/uuid"
)

const maxAvatarSize = 5 << 20

// @Summary Upload a profile avatar
// @Description JPEG or PNG up to 5 MiB and 4096 by 4096 pixels.
// @Tags profile
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param avatar formData file true "Avatar image"
// @Success 200 {object} avatarResponse
// @Failure 400 {string} string "Invalid image or multipart request"
// @Failure 401 {string} string "Invalid credentials"
// @Failure 413 {string} string "File too large"
// @Failure 500 {string} string "Internal server error"
// @Router /profile/avatar [post]
func (a *Controller) uploadAvatar(w http.ResponseWriter, r *http.Request) {
	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		respondError(w, ErrCredentials)
		return
	}
	id, err := a.Service.VerifyAccess(parts[1])
	if err != nil {
		respondError(w, err)
		return
	}
	data, err := uploads.ReadFile(w, r, "avatar", maxAvatarSize)
	if err != nil {
		uploadError(w, err)
		return
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "jpeg" && format != "png") || config.Width <= 0 || config.Height <= 0 || config.Width > 4096 || config.Height > 4096 {
		http.Error(w, "invalid image", http.StatusBadRequest)
		return
	}
	if _, _, err = image.Decode(bytes.NewReader(data)); err != nil {
		http.Error(w, "invalid image", http.StatusBadRequest)
		return
	}
	ext := ".png"
	if format == "jpeg" {
		ext = ".jpg"
	}
	name, err := uploads.Save(a.AvatarDir, ext, data)
	if err != nil {
		respondError(w, err)
		return
	}
	path := filepath.Join(a.AvatarDir, name)
	avatarURL := "/avatars/" + name
	result := a.Service.DB.WithContext(r.Context()).Model(&storage.User{}).Where("id = ?", id).Update("avatar_url", avatarURL)
	if result.Error != nil || result.RowsAffected == 0 {
		_ = os.Remove(path)
		if result.Error != nil {
			respondError(w, result.Error)
		} else {
			respondError(w, ErrCredentials)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(struct {
		AvatarURL string `json:"avatar_url"`
	}{avatarURL})
}

// @Summary Download an avatar
// @Tags profile
// @Produce image/png,image/jpeg
// @Param name path string true "Avatar filename"
// @Success 200 {file} file
// @Failure 404 {string} string "Avatar not found"
// @Router /avatars/{name} [get]
func (a *Controller) serveAvatar(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	ext := filepath.Ext(name)
	if (ext != ".png" && ext != ".jpg") || uuid.Validate(strings.TrimSuffix(name, ext)) != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, filepath.Join(a.AvatarDir, name))
}
