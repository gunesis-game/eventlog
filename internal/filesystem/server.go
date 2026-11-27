package filesystem

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"strconv"
	"strings"
)

// Server exposes a Backend over HTTP on 127.0.0.1, at a random port, to
// clients presenting its random bearer token.
type Server struct {
	backend  Backend
	listener net.Listener
	http     *http.Server
	token    string
}

// Serve starts a server for backend. Close stops it and closes the backend.
func Serve(backend Backend) (*Server, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("generate file-system token: %w", err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen for file-system server: %w", err)
	}
	s := &Server{backend: backend, listener: listener, token: hex.EncodeToString(secret)}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /files/{path...}", s.readFile)
	mux.HandleFunc("PUT /files/{path...}", s.putFile)
	mux.HandleFunc("POST /files/{path...}", s.changeFile)
	mux.HandleFunc("DELETE /files/{path...}", s.removeFile)
	mux.HandleFunc("GET /dirs/{path...}", s.listDirectory)
	mux.HandleFunc("PUT /dirs/{path...}", s.makeDirectory)
	mux.HandleFunc("POST /locks/{name}", s.lock)
	mux.HandleFunc("DELETE /locks/{name}", s.unlock)
	s.http = &http.Server{Handler: s.authenticate(mux)}
	go func() {
		_ = s.http.Serve(listener)
	}()
	return s, nil
}

// URL returns the server's base URL.
func (s *Server) URL() string {
	return "http://" + s.listener.Addr().String()
}

// Token returns the bearer token clients must present.
func (s *Server) Token() string {
	return s.token
}

// Close stops serving and closes the backend.
func (s *Server) Close() error {
	return errors.Join(s.http.Close(), s.backend.Close())
}

func (s *Server) authenticate(next http.Handler) http.Handler {
	expected := []byte("Bearer " + s.token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), expected) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func requestPath(w http.ResponseWriter, r *http.Request, name string) (string, bool) {
	p, err := cleanPath(r.PathValue(name))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return "", false
	}
	return p, true
}

// writeError maps backend errors to the contract's status codes.
func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, fs.ErrNotExist):
		status = http.StatusNotFound
	case errors.Is(err, fs.ErrExist):
		status = http.StatusPreconditionFailed
	case errors.Is(err, ErrLocked):
		status = http.StatusConflict
	}
	http.Error(w, err.Error(), status)
}

func (s *Server) readFile(w http.ResponseWriter, r *http.Request) {
	p, ok := requestPath(w, r, "path")
	if !ok {
		return
	}
	offset, length, err := parseRange(r.Header.Get("Range"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusRequestedRangeNotSatisfiable)
		return
	}
	data, err := s.backend.Read(p, offset, length)
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	_, _ = w.Write(data)
}

// parseRange parses "bytes=a-b" (inclusive) into an offset and length; an
// absent header means the whole file (length -1).
func parseRange(header string) (int64, int64, error) {
	if header == "" {
		return 0, -1, nil
	}
	spec, ok := strings.CutPrefix(header, "bytes=")
	first, last, found := strings.Cut(spec, "-")
	if !ok || !found {
		return 0, 0, fmt.Errorf("unsupported range %q", header)
	}
	start, err := strconv.ParseInt(first, 10, 64)
	if err != nil || start < 0 {
		return 0, 0, fmt.Errorf("unsupported range %q", header)
	}
	if last == "" {
		return start, -1, nil
	}
	end, err := strconv.ParseInt(last, 10, 64)
	if err != nil || end < start {
		return 0, 0, fmt.Errorf("unsupported range %q", header)
	}
	return start, end - start + 1, nil
}

func (s *Server) putFile(w http.ResponseWriter, r *http.Request) {
	p, ok := requestPath(w, r, "path")
	if !ok {
		return
	}
	if r.Header.Get("If-None-Match") == "*" {
		if err := s.backend.CreateExclusive(p); err != nil {
			writeError(w, err)
			return
		}
		w.WriteHeader(http.StatusCreated)
		return
	}
	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.backend.Replace(p, data); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) changeFile(w http.ResponseWriter, r *http.Request) {
	p, ok := requestPath(w, r, "path")
	if !ok {
		return
	}
	query := r.URL.Query()
	switch query.Get("op") {
	case "append":
		data, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		size, err := s.backend.Append(p, data)
		if err != nil {
			writeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(struct {
			Size int64 `json:"size"`
		}{size})
	case "truncate":
		size, err := strconv.ParseInt(query.Get("size"), 10, 64)
		if err != nil {
			http.Error(w, "invalid size", http.StatusBadRequest)
			return
		}
		s.respond(w, s.backend.Truncate(p, size))
	case "sync":
		s.respond(w, s.backend.Sync(p))
	case "rename":
		to, err := cleanPath(query.Get("to"))
		if err != nil || to == "" {
			http.Error(w, "invalid rename target", http.StatusBadRequest)
			return
		}
		s.respond(w, s.backend.Rename(p, to))
	default:
		http.Error(w, "unknown operation", http.StatusBadRequest)
	}
}

func (s *Server) respond(w http.ResponseWriter, err error) {
	if err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) removeFile(w http.ResponseWriter, r *http.Request) {
	if p, ok := requestPath(w, r, "path"); ok {
		s.respond(w, s.backend.Remove(p))
	}
}

func (s *Server) listDirectory(w http.ResponseWriter, r *http.Request) {
	p, ok := requestPath(w, r, "path")
	if !ok {
		return
	}
	entries, err := s.backend.List(p)
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(entries)
}

func (s *Server) makeDirectory(w http.ResponseWriter, r *http.Request) {
	if p, ok := requestPath(w, r, "path"); ok {
		s.respond(w, s.backend.MakeDirectory(p))
	}
}

func (s *Server) lock(w http.ResponseWriter, r *http.Request) {
	if name, ok := requestPath(w, r, "name"); ok {
		s.respond(w, s.backend.Lock(name))
	}
}

func (s *Server) unlock(w http.ResponseWriter, r *http.Request) {
	if name, ok := requestPath(w, r, "name"); ok {
		s.respond(w, s.backend.Unlock(name))
	}
}
