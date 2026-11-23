package auth

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

type monitor struct {
	ID              string    `json:"id" gorm:"type:uuid;primaryKey"`
	UserID          string    `json:"-"`
	URL             string    `json:"url"`
	IntervalSeconds int       `json:"interval_seconds"`
	CreatedAt       time.Time `json:"created_at"`
}

func validMonitor(raw string, interval int) bool {
	if len(raw) == 0 || len(raw) > 2048 || strings.TrimSpace(raw) != raw || interval < 1 || interval > 2147483647 {
		return false
	}
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Hostname() != "" && u.User == nil && u.Fragment == "" && u.Port() != "0"
}

func (a *Controller) monitors(w http.ResponseWriter, r *http.Request) {
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
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	if r.Method == http.MethodGet {
		items := []monitor{}
		if err := a.Service.DB.WithContext(r.Context()).Where("user_id = ?", id).Order("created_at DESC, id DESC").Find(&items).Error; err != nil {
			respondError(w, err)
			return
		}
		_ = json.NewEncoder(w).Encode(items)
		return
	}
	var input struct {
		URL             string `json:"url"`
		IntervalSeconds int    `json:"interval_seconds"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || decoder.Decode(&struct{}{}) != io.EOF || !validMonitor(input.URL, input.IntervalSeconds) {
		http.Error(w, "invalid monitor", http.StatusBadRequest)
		return
	}
	item := monitor{ID: uuid.NewString(), UserID: id, URL: input.URL, IntervalSeconds: input.IntervalSeconds}
	if err := a.Service.DB.WithContext(r.Context()).Create(&item).Error; err != nil {
		respondError(w, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(item)
}
