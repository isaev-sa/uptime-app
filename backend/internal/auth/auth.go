package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"example.com/uptime-app/backend/internal/storage"
)

var ErrInvalid = errors.New("invalid input")
var ErrCredentials = errors.New("invalid credentials")
var ErrRefresh = errors.New("invalid refresh token")
var ErrDuplicate = errors.New("email already registered")

type Service struct {
	DB                    *gorm.DB
	Secret                []byte
	AccessTTL, RefreshTTL time.Duration
}
type Tokens struct {
	Access           string
	Refresh          string
	AccessExpiresAt  time.Time
	RefreshExpiresAt time.Time
	Email            string
	Name             string
	AvatarURL        string
}

func normalize(email, password string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	a, err := mail.ParseAddress(email)
	if err != nil || a.Address != email || len(email) > 254 || len(password) < 8 || len(password) > 72 || !utf8.ValidString(password) {
		return "", ErrInvalid
	}
	return email, nil
}
func (s *Service) issue(userID, email, name, avatarURL string) (Tokens, storage.RefreshSession, error) {
	now := time.Now().UTC()
	expires := now.Add(s.AccessTTL)
	refreshExpires := now.Add(s.RefreshTTL)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{Subject: userID, IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(expires)})
	access, err := token.SignedString(s.Secret)
	if err != nil {
		return Tokens{}, storage.RefreshSession{}, err
	}
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return Tokens{}, storage.RefreshSession{}, err
	}
	refresh := base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256([]byte(refresh))
	return Tokens{Access: access, Refresh: refresh, AccessExpiresAt: expires, RefreshExpiresAt: refreshExpires, Email: email, Name: name, AvatarURL: avatarURL}, storage.RefreshSession{ID: uuid.NewString(), UserID: userID, TokenHash: hash[:], ExpiresAt: refreshExpires}, nil
}
func (s *Service) VerifyAccess(raw string) (string, error) {
	claims := &jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) { return s.Secret, nil }, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())
	if err != nil || !token.Valid || claims.Subject == "" {
		return "", ErrCredentials
	}
	return claims.Subject, nil
}
func validateName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if !utf8.ValidString(name) || utf8.RuneCountInString(name) == 0 || utf8.RuneCountInString(name) > 100 {
		return "", ErrInvalid
	}
	return name, nil
}

func (s *Service) Profile(ctx context.Context, id string) (storage.User, error) {
	var user storage.User
	err := s.DB.WithContext(ctx).First(&user, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return user, ErrCredentials
	}
	return user, err
}

func (s *Service) UpdateProfile(ctx context.Context, id, name string) (storage.User, error) {
	name, err := validateName(name)
	if err != nil {
		return storage.User{}, err
	}
	result := s.DB.WithContext(ctx).Model(&storage.User{}).Where("id = ?", id).Update("name", name)
	if result.Error != nil {
		return storage.User{}, result.Error
	}
	if result.RowsAffected == 0 {
		return storage.User{}, ErrCredentials
	}
	return s.Profile(ctx, id)
}

func (s *Service) Register(ctx context.Context, name, email, password string) (Tokens, error) {
	name, err := validateName(name)
	if err != nil {
		return Tokens{}, ErrInvalid
	}
	email, err = normalize(email, password)
	if err != nil {
		return Tokens{}, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return Tokens{}, err
	}
	user := storage.User{ID: uuid.NewString(), Email: email, Name: name, PasswordHash: string(hash)}
	tokens, session, err := s.issue(user.ID, user.Email, user.Name, user.AvatarURL)
	if err != nil {
		return Tokens{}, err
	}
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		return tx.Create(&session).Error
	})
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return Tokens{}, ErrDuplicate
	}
	return tokens, err
}
func (s *Service) Login(ctx context.Context, email, password string) (Tokens, error) {
	email, err := normalize(email, password)
	if err != nil {
		return Tokens{}, ErrCredentials
	}
	var user storage.User
	err = s.DB.WithContext(ctx).Where("email = ?", email).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Tokens{}, ErrCredentials
	}
	if err != nil {
		return Tokens{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return Tokens{}, ErrCredentials
	}
	tokens, session, err := s.issue(user.ID, user.Email, user.Name, user.AvatarURL)
	if err != nil {
		return Tokens{}, err
	}
	if err = s.DB.WithContext(ctx).Create(&session).Error; err != nil {
		return Tokens{}, err
	}
	return tokens, nil
}
func (s *Service) Refresh(ctx context.Context, raw string) (Tokens, error) {
	if len(raw) != 43 {
		return Tokens{}, ErrRefresh
	}
	hash := sha256.Sum256([]byte(raw))
	var tokens Tokens
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var old storage.RefreshSession
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("token_hash = ? AND expires_at > ?", hash[:], time.Now().UTC()).First(&old).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrRefresh
		}
		if err != nil {
			return err
		}
		var user storage.User
		if err = tx.First(&user, "id = ?", old.UserID).Error; err != nil {
			return err
		}
		var next storage.RefreshSession
		tokens, next, err = s.issue(old.UserID, user.Email, user.Name, user.AvatarURL)
		if err != nil {
			return err
		}
		if err = tx.Delete(&old).Error; err != nil {
			return err
		}
		return tx.Create(&next).Error
	})
	if err != nil {
		return Tokens{}, err
	}
	return tokens, nil
}

// Logout revokes the current refresh session. Missing or invalid tokens are already logged out.
func (s *Service) Logout(ctx context.Context, raw string) error {
	if len(raw) != 43 {
		return nil
	}
	hash := sha256.Sum256([]byte(raw))
	return s.DB.WithContext(ctx).Where("token_hash = ?", hash[:]).Delete(&storage.RefreshSession{}).Error
}
