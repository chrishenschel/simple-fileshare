package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSessionRoundTrip(t *testing.T) {
	m := NewManager("0123456789abcdef", 7*24*time.Hour)
	c, err := m.CreateCookie(false)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(c)
	if !m.Valid(r) {
		t.Fatal("expected valid session")
	}
}

func TestTamperedCookieRejected(t *testing.T) {
	m := NewManager("0123456789abcdef", time.Hour)
	c, err := m.CreateCookie(false)
	if err != nil {
		t.Fatal(err)
	}
	c.Value = c.Value + "x"
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(c)
	if m.Valid(r) {
		t.Fatal("tampered cookie should be invalid")
	}
}
