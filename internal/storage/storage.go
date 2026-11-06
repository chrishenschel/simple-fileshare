package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	MaxUploadBytes   = 200 * 1024 * 1024
	ExpirationsFile  = "expirations.json"
	InfoFile         = "info.json"
	shareIDPattern   = `^[A-Za-z0-9]{5}$`
)

var (
	ErrNotFound     = errors.New("share not found")
	ErrExpired      = errors.New("share expired")
	ErrBadPIN       = errors.New("incorrect pin")
	ErrTooLarge     = errors.New("file too large")
	ErrInvalidPIN   = errors.New("pin must be 4 digits")
	ErrInvalidExpiry = errors.New("invalid expiry")
	shareIDRE       = regexp.MustCompile(shareIDPattern)
	pinRE           = regexp.MustCompile(`^\d{4}$`)
)

type Info struct {
	PIN       string     `json:"pin"`
	Filename  string     `json:"filename"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at"`
}

type Store struct {
	root string
	mu   sync.Mutex
}

func New(root string) (*Store, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	s := &Store{root: root}
	if err := s.ensureExpirations(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Root() string { return s.root }

func (s *Store) ensureExpirations() error {
	path := filepath.Join(s.root, ExpirationsFile)
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return s.writeExpirations(map[string]time.Time{})
	}
	return err
}

func (s *Store) ValidShareID(id string) bool {
	return shareIDRE.MatchString(id)
}

func (s *Store) Exists(id string) bool {
	_, err := os.Stat(filepath.Join(s.root, id))
	return err == nil
}

func ParseExpiry(option string, now time.Time) (*time.Time, error) {
	var d time.Duration
	switch option {
	case "never":
		return nil, nil
	case "1h":
		d = time.Hour
	case "3h":
		d = 3 * time.Hour
	case "10h":
		d = 10 * time.Hour
	case "24h":
		d = 24 * time.Hour
	case "7d":
		d = 7 * 24 * time.Hour
	default:
		return nil, ErrInvalidExpiry
	}
	t := now.Add(d).UTC()
	return &t, nil
}

func SanitizeFilename(name string) string {
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, "..", "")
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == string(os.PathSeparator) {
		return "file"
	}
	return name
}

func (s *Store) Create(id, pin, originalName string, expiresAt *time.Time, r io.Reader, size int64) error {
	if !pinRE.MatchString(pin) {
		return ErrInvalidPIN
	}
	if size > MaxUploadBytes {
		return ErrTooLarge
	}
	if !s.ValidShareID(id) {
		return fmt.Errorf("invalid share id")
	}

	filename := SanitizeFilename(originalName)
	dir := filepath.Join(s.root, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	destPath := filepath.Join(dir, filename)
	f, err := os.OpenFile(destPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		_ = os.RemoveAll(dir)
		return err
	}
	limited := io.LimitReader(r, MaxUploadBytes+1)
	written, err := io.Copy(f, limited)
	closeErr := f.Close()
	if err != nil {
		_ = os.RemoveAll(dir)
		return err
	}
	if closeErr != nil {
		_ = os.RemoveAll(dir)
		return closeErr
	}
	if written > MaxUploadBytes {
		_ = os.RemoveAll(dir)
		return ErrTooLarge
	}

	info := Info{
		PIN:       pin,
		Filename:  filename,
		CreatedAt: time.Now().UTC(),
		ExpiresAt: expiresAt,
	}
	raw, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		_ = os.RemoveAll(dir)
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, InfoFile), raw, 0o644); err != nil {
		_ = os.RemoveAll(dir)
		return err
	}

	if expiresAt != nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		exp, err := s.readExpirations()
		if err != nil {
			_ = os.RemoveAll(dir)
			return err
		}
		exp[id] = expiresAt.UTC()
		if err := s.writeExpirations(exp); err != nil {
			_ = os.RemoveAll(dir)
			return err
		}
	}
	return nil
}

func (s *Store) GetInfo(id string) (Info, error) {
	if !s.ValidShareID(id) {
		return Info{}, ErrNotFound
	}
	raw, err := os.ReadFile(filepath.Join(s.root, id, InfoFile))
	if err != nil {
		if os.IsNotExist(err) {
			return Info{}, ErrNotFound
		}
		return Info{}, err
	}
	var info Info
	if err := json.Unmarshal(raw, &info); err != nil {
		return Info{}, err
	}
	if info.ExpiresAt != nil && !info.ExpiresAt.After(time.Now().UTC()) {
		return Info{}, ErrExpired
	}
	return info, nil
}

func (s *Store) OpenFile(id string, info Info) (*os.File, error) {
	path := filepath.Join(s.root, id, info.Filename)
	return os.Open(path)
}

func (s *Store) CheckPIN(id, pin string) (Info, error) {
	info, err := s.GetInfo(id)
	if err != nil {
		return Info{}, err
	}
	if info.PIN != pin {
		return Info{}, ErrBadPIN
	}
	return info, nil
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	exp, err := s.readExpirations()
	if err != nil {
		return err
	}
	delete(exp, id)
	if err := s.writeExpirations(exp); err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(s.root, id))
}

// DueIDs returns share ids whose expiry time is <= now (caller should hold no assumption of lock).
func (s *Store) DueIDs(now time.Time) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	exp, err := s.readExpirations()
	if err != nil {
		return nil, err
	}
	var due []string
	now = now.UTC()
	for id, at := range exp {
		if !at.After(now) {
			due = append(due, id)
		}
	}
	return due, nil
}

func (s *Store) readExpirations() (map[string]time.Time, error) {
	path := filepath.Join(s.root, ExpirationsFile)
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]time.Time{}, nil
		}
		return nil, err
	}
	if len(raw) == 0 {
		return map[string]time.Time{}, nil
	}
	var rawMap map[string]string
	if err := json.Unmarshal(raw, &rawMap); err != nil {
		return nil, err
	}
	out := make(map[string]time.Time, len(rawMap))
	for id, ts := range rawMap {
		t, err := time.Parse(time.RFC3339, ts)
		if err != nil {
			return nil, fmt.Errorf("expirations %s: %w", id, err)
		}
		out[id] = t.UTC()
	}
	return out, nil
}

func (s *Store) writeExpirations(exp map[string]time.Time) error {
	rawMap := make(map[string]string, len(exp))
	for id, t := range exp {
		rawMap[id] = t.UTC().Format(time.RFC3339)
	}
	raw, err := json.MarshalIndent(rawMap, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(s.root, ExpirationsFile)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
