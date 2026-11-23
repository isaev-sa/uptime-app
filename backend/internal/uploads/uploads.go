// Package uploads provides bounded multipart parsing and local file storage.
package uploads

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

var ErrInvalid = errors.New("invalid multipart request")
var ErrTooLarge = errors.New("file too large")

// ReadFile accepts exactly one multipart file and bounds both its contents and request overhead.
func ReadFile(w http.ResponseWriter, r *http.Request, field string, limit int64) ([]byte, error) {
	r.Body = http.MaxBytesReader(w, r.Body, limit+(64<<10))
	reader, err := r.MultipartReader()
	if err != nil {
		return nil, ErrInvalid
	}
	part, err := reader.NextPart()
	if err != nil || part.FormName() != field || part.FileName() == "" {
		return nil, ErrInvalid
	}
	data, err := io.ReadAll(io.LimitReader(part, limit+1))
	if err != nil || int64(len(data)) > limit {
		return nil, ErrTooLarge
	}
	if next, err := reader.NextPart(); err != io.EOF || next != nil {
		return nil, ErrInvalid
	}
	return data, nil
}

// Save assigns a random name and never uses a client-supplied path.
func Save(dir, ext string, data []byte) (string, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	name := uuid.NewString() + ext
	file, err := os.OpenFile(filepath.Join(dir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return "", err
	}
	if _, err = file.Write(data); err != nil {
		file.Close()
		os.Remove(filepath.Join(dir, name))
		return "", err
	}
	if err = file.Close(); err != nil {
		os.Remove(filepath.Join(dir, name))
		return "", err
	}
	return name, nil
}
