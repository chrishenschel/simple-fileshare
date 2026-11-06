package web

import (
	"crypto/subtle"
	"embed"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/chrishenschel/simple-fileshare/internal/auth"
	"github.com/chrishenschel/simple-fileshare/internal/config"
	"github.com/chrishenschel/simple-fileshare/internal/idgen"
	"github.com/chrishenschel/simple-fileshare/internal/ratelimit"
	"github.com/chrishenschel/simple-fileshare/internal/storage"
)

//go:embed templates/*.html
var templateFS embed.FS

type Server struct {
	cfg        config.Config
	store      *storage.Store
	sessions   *auth.Manager
	loginLimit *ratelimit.Limiter
	pinLimit   *ratelimit.Limiter
	pages      map[string]*template.Template
}

func New(cfg config.Config, store *storage.Store, sessions *auth.Manager) (*Server, error) {
	pages, err := loadPages()
	if err != nil {
		return nil, err
	}
	return &Server{
		cfg:        cfg,
		store:      store,
		sessions:   sessions,
		loginLimit: ratelimit.New(5, 15*time.Minute),
		pinLimit:   ratelimit.New(5, 15*time.Minute),
		pages:      pages,
	}, nil
}

func loadPages() (map[string]*template.Template, error) {
	layout, err := fs.ReadFile(templateFS, "templates/layout.html")
	if err != nil {
		return nil, err
	}
	entries, err := fs.ReadDir(templateFS, "templates")
	if err != nil {
		return nil, err
	}
	pages := make(map[string]*template.Template)
	for _, e := range entries {
		name := e.Name()
		if name == "layout.html" || !strings.HasSuffix(name, ".html") {
			continue
		}
		body, err := fs.ReadFile(templateFS, "templates/"+name)
		if err != nil {
			return nil, err
		}
		tpl, err := template.New(name).Parse(string(layout))
		if err != nil {
			return nil, err
		}
		if _, err := tpl.Parse(string(body)); err != nil {
			return nil, err
		}
		key := strings.TrimSuffix(name, ".html")
		pages[key] = tpl
	}
	return pages, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /login", s.handleLoginGet)
	mux.HandleFunc("POST /login", s.handleLoginPost)
	mux.HandleFunc("POST /logout", s.handleLogout)
	mux.HandleFunc("GET /{$}", s.handleUploadGet)
	mux.HandleFunc("POST /{$}", s.handleUploadPost)
	mux.HandleFunc("GET /f/{id}", s.handlePinGet)
	mux.HandleFunc("POST /f/{id}", s.handlePinPost)
	return mux
}

type pageData struct {
	Title     string
	Error     string
	Message   string
	Link      string
	LinkLabel string
	PIN       string
	ID        string
}

func (s *Server) render(w http.ResponseWriter, name string, status int, data pageData) {
	if data.Title == "" {
		data.Title = name
	}
	tpl := s.pages[name]
	if tpl == nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := tpl.ExecuteTemplate(w, "layout", data); err != nil {
		log.Printf("render %s: %v", name, err)
	}
}

func (s *Server) clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (s *Server) isSecure(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func (s *Server) requireAuth(w http.ResponseWriter, r *http.Request) bool {
	if s.sessions.Valid(r) {
		return true
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
	return false
}

func (s *Server) handleLoginGet(w http.ResponseWriter, r *http.Request) {
	if s.sessions.Valid(r) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	s.render(w, "login", http.StatusOK, pageData{Title: "Sign in"})
}

func (s *Server) handleLoginPost(w http.ResponseWriter, r *http.Request) {
	ip := s.clientIP(r)
	if !s.loginLimit.Allow(ip) {
		s.render(w, "login", http.StatusTooManyRequests, pageData{
			Title: "Sign in",
			Error: "Too many attempts, try later.",
		})
		return
	}
	if err := r.ParseForm(); err != nil {
		s.render(w, "login", http.StatusBadRequest, pageData{Title: "Sign in", Error: "Invalid form."})
		return
	}
	pw := r.FormValue("password")
	if subtle.ConstantTimeCompare([]byte(pw), []byte(s.cfg.MasterPassword)) != 1 {
		s.render(w, "login", http.StatusUnauthorized, pageData{
			Title: "Sign in",
			Error: "Incorrect password.",
		})
		return
	}
	cookie, err := s.sessions.CreateCookie(s.isSecure(r))
	if err != nil {
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, cookie)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, s.sessions.ClearCookie(s.isSecure(r)))
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *Server) handleUploadGet(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	s.render(w, "upload", http.StatusOK, pageData{Title: "Upload"})
}

func (s *Server) handleUploadPost(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, storage.MaxUploadBytes+1024*1024)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		s.render(w, "upload", http.StatusBadRequest, pageData{
			Title: "Upload",
			Error: "Upload too large or invalid (max 200 MB).",
		})
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		s.render(w, "upload", http.StatusBadRequest, pageData{Title: "Upload", Error: "File is required."})
		return
	}
	defer file.Close()

	pin := strings.TrimSpace(r.FormValue("pin"))
	expiryOpt := r.FormValue("expiry")
	expiresAt, err := storage.ParseExpiry(expiryOpt, time.Now().UTC())
	if err != nil {
		s.render(w, "upload", http.StatusBadRequest, pageData{Title: "Upload", Error: "Invalid expiry option."})
		return
	}

	var id string
	for i := 0; i < 8; i++ {
		candidate, err := idgen.New(idgen.DefaultLength)
		if err != nil {
			http.Error(w, "id error", http.StatusInternalServerError)
			return
		}
		if !s.store.Exists(candidate) {
			id = candidate
			break
		}
	}
	if id == "" {
		http.Error(w, "could not allocate id", http.StatusInternalServerError)
		return
	}

	size := hdr.Size
	if size < 0 {
		size = 0
	}
	err = s.store.Create(id, pin, hdr.Filename, expiresAt, file, size)
	if err == storage.ErrInvalidPIN {
		s.render(w, "upload", http.StatusBadRequest, pageData{Title: "Upload", Error: "PIN must be exactly 4 digits."})
		return
	}
	if err == storage.ErrTooLarge {
		s.render(w, "upload", http.StatusBadRequest, pageData{Title: "Upload", Error: "File too large (max 200 MB)."})
		return
	}
	if err != nil {
		log.Printf("upload: %v", err)
		s.render(w, "upload", http.StatusInternalServerError, pageData{Title: "Upload", Error: "Upload failed."})
		return
	}

	link := fmt.Sprintf("%s/f/%s", s.cfg.Domain, id)
	s.render(w, "upload", http.StatusOK, pageData{
		Title: "Upload",
		Link:  link,
		PIN:   pin,
	})
}

func (s *Server) handlePinGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.store.GetInfo(id); err != nil {
		s.render(w, "message", http.StatusNotFound, pageData{
			Title:   "Not found",
			Message: "File not found or expired.",
		})
		return
	}
	s.render(w, "pin", http.StatusOK, pageData{Title: "Enter PIN", ID: id})
}

func (s *Server) handlePinPost(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ip := s.clientIP(r)
	key := ip + "|" + id
	if !s.pinLimit.Allow(key) {
		s.render(w, "pin", http.StatusTooManyRequests, pageData{
			Title: "Enter PIN",
			ID:    id,
			Error: "Too many attempts, try later.",
		})
		return
	}
	if err := r.ParseForm(); err != nil {
		s.render(w, "pin", http.StatusBadRequest, pageData{Title: "Enter PIN", ID: id, Error: "Invalid form."})
		return
	}
	pin := strings.TrimSpace(r.FormValue("pin"))
	info, err := s.store.CheckPIN(id, pin)
	if err == storage.ErrNotFound || err == storage.ErrExpired {
		s.render(w, "message", http.StatusNotFound, pageData{
			Title:   "Not found",
			Message: "File not found or expired.",
		})
		return
	}
	if err == storage.ErrBadPIN {
		s.render(w, "pin", http.StatusForbidden, pageData{
			Title: "Enter PIN",
			ID:    id,
			Error: "Incorrect PIN.",
		})
		return
	}
	if err != nil {
		http.Error(w, "error", http.StatusInternalServerError)
		return
	}

	f, err := s.store.OpenFile(id, info)
	if err != nil {
		s.render(w, "message", http.StatusNotFound, pageData{
			Title:   "Not found",
			Message: "File not found or expired.",
		})
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		http.Error(w, "error", http.StatusInternalServerError)
		return
	}
	disposition := fmt.Sprintf(`attachment; filename="%s"`, path.Base(info.Filename))
	w.Header().Set("Content-Disposition", disposition)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", stat.Size()))
	_, _ = io.Copy(w, f)
}
