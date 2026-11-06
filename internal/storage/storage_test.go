package storage

import (
	"strings"
	"testing"
	"time"
)

func TestParseExpiry(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	if exp, err := ParseExpiry("never", now); err != nil || exp != nil {
		t.Fatalf("never: %v %v", exp, err)
	}
	exp, err := ParseExpiry("3h", now)
	if err != nil {
		t.Fatal(err)
	}
	if !exp.Equal(now.Add(3 * time.Hour)) {
		t.Fatalf("got %v", exp)
	}
}

func TestCreateAndPIN(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	exp := time.Now().UTC().Add(time.Hour)
	err = s.Create("AbcdE", "1234", "report.pdf", &exp, strings.NewReader("hello"), 5)
	if err != nil {
		t.Fatal(err)
	}
	info, err := s.CheckPIN("AbcdE", "1234")
	if err != nil {
		t.Fatal(err)
	}
	if info.Filename != "report.pdf" {
		t.Fatalf("filename %q", info.Filename)
	}
	if _, err := s.CheckPIN("AbcdE", "9999"); err != ErrBadPIN {
		t.Fatalf("want ErrBadPIN got %v", err)
	}
}

func TestSanitizeFilename(t *testing.T) {
	if SanitizeFilename("../../etc/passwd") != "passwd" {
		t.Fatal(SanitizeFilename("../../etc/passwd"))
	}
}
