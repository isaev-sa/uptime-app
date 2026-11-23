package auth

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"example.com/uptime-app/backend/internal/server"
)

func TestFileUploadAndDownload(t *testing.T) {
	service := &Service{Secret: []byte("01234567890123456789012345678901"), AccessTTL: time.Minute, RefreshTTL: time.Hour}
	token, _, err := service.issue("user-id", "test@example.com", "Test", "")
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	(&Controller{Service: service, UploadDir: t.TempDir(), AvatarDir: t.TempDir()}).RegisterRoutes(mux)
	handler := server.Middleware(mux, "http://localhost:3000")
	upload := func(token, field, content string) *httptest.ResponseRecorder {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, err := writer.CreateFormFile(field, "document.html")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = part.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
		if err = writer.Close(); err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "/files", &body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w
	}
	if w := upload("invalid", "file", "x"); w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized: %d", w.Code)
	}
	if w := upload(token.Access, "other", "x"); w.Code != http.StatusBadRequest {
		t.Fatalf("field: %d", w.Code)
	}
	if w := upload(token.Access, "file", strings.Repeat("x", maxFileSize+1)); w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("size: %d", w.Code)
	}
	content := "<script>alert('test')</script>"
	w := upload(token.Access, "file", content)
	var result struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || w.Code != http.StatusCreated || !strings.HasPrefix(result.URL, "/files/") {
		t.Fatalf("upload: %d %s %v", w.Code, w.Body, err)
	}
	download := httptest.NewRecorder()
	handler.ServeHTTP(download, httptest.NewRequest(http.MethodGet, result.URL, nil))
	if download.Code != http.StatusOK || download.Body.String() != content || download.Header().Get("Content-Disposition") != "attachment" || download.Header().Get("Content-Type") != "application/octet-stream" {
		t.Fatalf("download: %d %v %s", download.Code, download.Header(), download.Body)
	}
}
