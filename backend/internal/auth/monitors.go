package auth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"example.com/uptime-app/backend/internal/storage"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const maxMonitors = 1000
const monitorPageSize = 50

var errMonitorLimit = errors.New("monitor limit reached")

type monitor struct {
	ID              string    `json:"id" gorm:"type:uuid;primaryKey"`
	UserID          string    `json:"-"`
	URL             string    `json:"url"`
	IntervalSeconds int       `json:"interval_seconds"`
	CreatedAt       time.Time `json:"created_at"`
}

type monitorPage struct {
	Items   []monitor `json:"items"`
	HasMore bool      `json:"has_more"`
}

func validMonitor(raw string, interval int) bool {
	if len(raw) == 0 || len(raw) > 2048 || strings.TrimSpace(raw) != raw || interval < 1 || interval > 2147483647 {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return false
	}
	if port := u.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return false
		}
	}
	return true
}

func (s *Service) listMonitors(ctx context.Context, userID string, before time.Time, beforeID string) (monitorPage, error) {
	items := []monitor{}
	query := s.DB.WithContext(ctx).Where("user_id = ?", userID)
	if beforeID != "" {
		query = query.Where("(created_at, id) < (?, ?)", before, beforeID)
	}
	err := query.Order("created_at DESC, id DESC").Limit(monitorPageSize + 1).Find(&items).Error
	page := monitorPage{Items: items, HasMore: len(items) > monitorPageSize}
	if page.HasMore {
		page.Items = items[:monitorPageSize]
	}
	return page, err
}

func (s *Service) createMonitor(ctx context.Context, userID, rawURL string, interval int) (monitor, error) {
	item := monitor{ID: uuid.NewString(), UserID: userID, URL: rawURL, IntervalSeconds: interval}
	if !validMonitor(rawURL, interval) {
		return monitor{}, ErrInvalid
	}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user storage.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, "id = ?", userID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrCredentials
			}
			return err
		}
		var count int64
		if err := tx.Model(&monitor{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
			return err
		}
		if count >= maxMonitors {
			return errMonitorLimit
		}
		return tx.Create(&item).Error
	})
	return item, err
}

func (a *Controller) monitors(w http.ResponseWriter, r *http.Request) {
	id, err := a.authenticate(r)
	if err != nil {
		respondError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodGet {
		query := r.URL.Query()
		beforeID := query.Get("before_id")
		var before time.Time
		if beforeID != "" || query.Get("before") != "" {
			before, err = time.Parse(time.RFC3339Nano, query.Get("before"))
			if err != nil {
				http.Error(w, "invalid cursor", http.StatusBadRequest)
				return
			}
			if _, err = uuid.Parse(beforeID); err != nil {
				http.Error(w, "invalid cursor", http.StatusBadRequest)
				return
			}
		}
		page, err := a.Service.listMonitors(r.Context(), id, before, beforeID)
		if err != nil {
			respondError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(page)
		return
	}
	var input struct {
		URL             string `json:"url"`
		IntervalSeconds int    `json:"interval_seconds"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		http.Error(w, "invalid monitor", http.StatusBadRequest)
		return
	}
	item, err := a.Service.createMonitor(r.Context(), id, input.URL, input.IntervalSeconds)
	if errors.Is(err, ErrInvalid) {
		http.Error(w, "invalid monitor", http.StatusBadRequest)
		return
	}
	if errors.Is(err, errMonitorLimit) {
		http.Error(w, "monitor limit reached", http.StatusConflict)
		return
	}
	if err != nil {
		respondError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(item)
}

func (a *Controller) monitorByID(w http.ResponseWriter, r *http.Request) {
	userID, err := a.authenticate(r)
	if err != nil {
		respondError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	id := r.PathValue("id")
	if _, err := uuid.Parse(id); err != nil {
		http.Error(w, "invalid monitor id", http.StatusBadRequest)
		return
	}
	if r.Method == http.MethodDelete {
		result := a.Service.DB.WithContext(r.Context()).Where("id = ? AND user_id = ?", id, userID).Delete(&monitor{})
		if result.Error != nil {
			respondError(w, result.Error)
			return
		}
		if result.RowsAffected == 0 {
			http.Error(w, "monitor not found", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var input monitorRequest
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || decoder.Decode(&struct{}{}) != io.EOF || !validMonitor(input.URL, input.IntervalSeconds) {
		http.Error(w, "invalid monitor", http.StatusBadRequest)
		return
	}
	var item monitor
	err = a.Service.DB.WithContext(r.Context()).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&monitor{}).Where("id = ? AND user_id = ?", id, userID).Updates(map[string]interface{}{"url": input.URL, "interval_seconds": input.IntervalSeconds})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return tx.Where("id = ? AND user_id = ?", id, userID).First(&item).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		http.Error(w, "monitor not found", http.StatusNotFound)
		return
	}
	if err != nil {
		respondError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(item)
}
