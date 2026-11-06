package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

const CookieName = "fileshare_session"

var ErrInvalidSession = errors.New("invalid session")

type Session struct {
	ExpiresAt int64 `json:"exp"`
}

type Manager struct {
	secret []byte
	ttl    time.Duration
}

func NewManager(secret string, ttl time.Duration) *Manager {
	return &Manager{secret: []byte(secret), ttl: ttl}
}

func (m *Manager) CreateCookie(secure bool) (*http.Cookie, error) {
	s := Session{ExpiresAt: time.Now().Add(m.ttl).Unix()}
	raw, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	payload := base64.RawURLEncoding.EncodeToString(raw)
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	value := payload + "." + sig
	return &http.Cookie{
		Name:     CookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		Expires:  time.Unix(s.ExpiresAt, 0),
	}, nil
}

func (m *Manager) ClearCookie(secure bool) *http.Cookie {
	return &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	}
}

func (m *Manager) Valid(r *http.Request) bool {
	c, err := r.Cookie(CookieName)
	if err != nil || c.Value == "" {
		return false
	}
	_, err = m.parse(c.Value)
	return err == nil
}

func (m *Manager) parse(value string) (Session, error) {
	dot := -1
	for i := 0; i < len(value); i++ {
		if value[i] == '.' {
			dot = i
			break
		}
	}
	if dot <= 0 || dot >= len(value)-1 {
		return Session{}, ErrInvalidSession
	}
	payload, sig := value[:dot], value[dot+1:]
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(payload))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return Session{}, ErrInvalidSession
	}
	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return Session{}, ErrInvalidSession
	}
	var s Session
	if err := json.Unmarshal(raw, &s); err != nil {
		return Session{}, ErrInvalidSession
	}
	if time.Now().Unix() > s.ExpiresAt {
		return Session{}, ErrInvalidSession
	}
	return s, nil
}
