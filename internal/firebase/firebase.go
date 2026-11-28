// Package firebase is a client of Firebase Authentication and the Firebase
// Realtime Database REST API, written from Firebase's reference
// (firebase.google.com/docs/reference/rest/auth and
// firebase.google.com/docs/reference/rest/database): exchanging a Google ID
// token for a Firebase one and refreshing it, reading and writing database
// locations, and streaming their changes as server-sent events. It knows
// nothing of eventlog's storage. Its addresses are Firebase's unless given
// others, as tests give the emulator's.
package firebase

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Firebase's documented addresses for Authentication.
const (
	DefaultIdentityToolkitURL = "https://identitytoolkit.googleapis.com"
	DefaultSecureTokenURL     = "https://securetoken.googleapis.com"
)

// Auth calls Firebase Authentication for one project, identified by its web
// API key.
type Auth struct {
	http        *http.Client
	apiKey      string
	identityURL string
	tokenURL    string
}

// NewAuth returns an Authentication client for the project of apiKey,
// calling identityURL and tokenURL, Firebase's when empty.
func NewAuth(h *http.Client, apiKey, identityURL, tokenURL string) *Auth {
	if identityURL == "" {
		identityURL = DefaultIdentityToolkitURL
	}
	if tokenURL == "" {
		tokenURL = DefaultSecureTokenURL
	}
	return &Auth{http: h, apiKey: apiKey, identityURL: strings.TrimSuffix(identityURL, "/"), tokenURL: strings.TrimSuffix(tokenURL, "/")}
}

// Session is a Firebase sign-in: the user's ID in the project, an ID token
// for database requests, and the refresh token that renews it.
type Session struct {
	UID          string
	Email        string
	IDToken      string
	RefreshToken string
	Expiry       time.Time
}

// AuthError is an error response of Firebase Authentication, whose message
// starts with an error code such as INVALID_REFRESH_TOKEN.
type AuthError struct {
	Status  int
	Message string
}

func (e *AuthError) Error() string {
	return fmt.Sprintf("firebase auth: %d %s", e.Status, e.Message)
}

// Code returns the error code the message starts with.
func (e *AuthError) Code() string {
	code, _, _ := strings.Cut(e.Message, " ")
	return code
}

// SignInWithGoogle exchanges a Google ID token for a Firebase session with
// accounts:signInWithIdp. The account is recorded in the project on its first
// sign-in.
func (a *Auth) SignInWithGoogle(ctx context.Context, googleIDToken string) (Session, error) {
	body, err := json.Marshal(map[string]any{
		"requestUri":        "http://localhost",
		"postBody":          url.Values{"id_token": {googleIDToken}, "providerId": {"google.com"}}.Encode(),
		"returnSecureToken": true,
	})
	if err != nil {
		return Session{}, err
	}
	var response struct {
		LocalID      string `json:"localId"`
		Email        string `json:"email"`
		IDToken      string `json:"idToken"`
		RefreshToken string `json:"refreshToken"`
		ExpiresIn    string `json:"expiresIn"`
	}
	start := time.Now()
	if err := a.post(ctx, a.identityURL+"/v1/accounts:signInWithIdp", "application/json", body, &response); err != nil {
		return Session{}, err
	}
	expiry, err := expiresAt(start, response.ExpiresIn)
	if err != nil {
		return Session{}, err
	}
	return Session{UID: response.LocalID, Email: response.Email, IDToken: response.IDToken, RefreshToken: response.RefreshToken, Expiry: expiry}, nil
}

// Refresh exchanges a session's refresh token for a new ID token.
func (a *Auth) Refresh(ctx context.Context, s Session) (Session, error) {
	body := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {s.RefreshToken}}.Encode()
	var response struct {
		ExpiresIn    string `json:"expires_in"`
		RefreshToken string `json:"refresh_token"`
		IDToken      string `json:"id_token"`
		UserID       string `json:"user_id"`
	}
	start := time.Now()
	if err := a.post(ctx, a.tokenURL+"/v1/token", "application/x-www-form-urlencoded", []byte(body), &response); err != nil {
		return Session{}, err
	}
	expiry, err := expiresAt(start, response.ExpiresIn)
	if err != nil {
		return Session{}, err
	}
	return Session{UID: response.UserID, Email: s.Email, IDToken: response.IDToken, RefreshToken: response.RefreshToken, Expiry: expiry}, nil
}

func expiresAt(start time.Time, seconds string) (time.Time, error) {
	n, err := strconv.Atoi(seconds)
	if err != nil {
		return time.Time{}, fmt.Errorf("firebase auth: expiry %q: %w", seconds, err)
	}
	return start.Add(time.Duration(n) * time.Second), nil
}

func (a *Auth) post(ctx context.Context, address, contentType string, body []byte, out any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, address+"?"+url.Values{"key": {a.apiKey}}.Encode(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", contentType)
	response, err := a.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK {
		var failure struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(raw, &failure)
		return &AuthError{Status: response.StatusCode, Message: failure.Error.Message}
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("firebase auth: %w", err)
	}
	return nil
}

// Database calls one Realtime Database through its REST API.
type Database struct {
	http *http.Client
	url  string
}

// NewDatabase returns a client of the database at databaseURL, as the
// Firebase console shows it, such as https://<name>.firebaseio.com.
func NewDatabase(h *http.Client, databaseURL string) *Database {
	return &Database{http: h, url: strings.TrimSuffix(databaseURL, "/")}
}

// DatabaseError is an error response of the database, such as 401
// "Permission denied" when the security rules refuse a request.
type DatabaseError struct {
	Status  int
	Message string
}

func (e *DatabaseError) Error() string {
	return fmt.Sprintf("firebase database: %d %s", e.Status, e.Message)
}

// Unavailable reports the database answering 503, temporarily unavailable.
func (e *DatabaseError) Unavailable() bool {
	return e.Status == http.StatusServiceUnavailable
}

// address returns the REST address of a location: its path with .json,
// authorized by idToken when not empty.
func (d *Database) address(path, idToken string, query url.Values) string {
	if query == nil {
		query = url.Values{}
	}
	if idToken != "" {
		query.Set("auth", idToken)
	}
	return d.url + "/" + strings.Trim(path, "/") + ".json?" + query.Encode()
}

func (d *Database) call(ctx context.Context, method, path, idToken string, query url.Values, value any, out any) error {
	var body io.Reader
	if value != nil {
		raw, err := json.Marshal(value)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}
	request, err := http.NewRequestWithContext(ctx, method, d.address(path, idToken, query), body)
	if err != nil {
		return err
	}
	response, err := d.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if response.StatusCode >= 300 {
		var failure struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(raw, &failure)
		return &DatabaseError{Status: response.StatusCode, Message: failure.Error}
	}
	if out != nil && response.StatusCode != http.StatusNoContent {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("firebase database: %w", err)
		}
	}
	return nil
}

var silent = url.Values{"print": {"silent"}}

// Get reads a location into out; a location with no data reads as JSON
// null.
func (d *Database) Get(ctx context.Context, path, idToken string, out any) error {
	return d.call(ctx, http.MethodGet, path, idToken, nil, nil, out)
}

// Put replaces a location's data with value.
func (d *Database) Put(ctx context.Context, path, idToken string, value any) error {
	return d.call(ctx, http.MethodPut, path, idToken, silent, value, nil)
}

// Patch replaces the named children of a location, leaving the others;
// a key may be a path below the location.
func (d *Database) Patch(ctx context.Context, path, idToken string, children map[string]any) error {
	return d.call(ctx, http.MethodPatch, path, idToken, silent, children, nil)
}

// Delete removes a location's data.
func (d *Database) Delete(ctx context.Context, path, idToken string) error {
	return d.call(ctx, http.MethodDelete, path, idToken, silent, nil, nil)
}

// Event is one change of a streamed location. For put, the data at Path,
// relative to the streamed location, is replaced by Data; for patch, each
// key of Data replaces that child at Path. Cancel and AuthRevoked end the
// stream: the rules no longer allow the read, or the ID token has expired.
type Event struct {
	Type string
	Path string
	Data json.RawMessage
}

// Event types of a stream.
const (
	Put         = "put"
	Patch       = "patch"
	Cancel      = "cancel"
	AuthRevoked = "auth_revoked"
)

// Stream is an open stream of a location's changes.
type Stream struct {
	body   io.ReadCloser
	reader *bufio.Reader
	ended  bool
}

// Stream opens a stream of a location: an HTTP GET with Accept:
// text/event-stream, following the redirect to the server holding the data.
// Its first event is a put of the location's whole data at "/".
func (d *Database) Stream(ctx context.Context, path, idToken string) (*Stream, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, d.address(path, idToken, nil), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "text/event-stream")
	response, err := d.http.Do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		defer response.Body.Close()
		raw, _ := io.ReadAll(response.Body)
		var failure struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(raw, &failure)
		return nil, &DatabaseError{Status: response.StatusCode, Message: failure.Error}
	}
	return &Stream{body: response.Body, reader: bufio.NewReader(response.Body)}, nil
}

// Next returns the stream's next event, skipping keep-alive. After cancel or
// auth_revoked, and when the connection ends, it returns io.EOF.
func (s *Stream) Next() (Event, error) {
	for !s.ended {
		name, data, err := s.readEvent()
		if err != nil {
			if errors.Is(err, io.ErrUnexpectedEOF) {
				err = io.EOF
			}
			return Event{}, err
		}
		switch name {
		case "keep-alive":
			continue
		case Cancel, AuthRevoked:
			s.ended = true
			return Event{Type: name, Data: json.RawMessage(data)}, nil
		case Put, Patch:
			var payload struct {
				Path string          `json:"path"`
				Data json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal([]byte(data), &payload); err != nil {
				return Event{}, fmt.Errorf("firebase stream: %s event: %w", name, err)
			}
			return Event{Type: name, Path: payload.Path, Data: payload.Data}, nil
		default:
			return Event{}, fmt.Errorf("firebase stream: unknown event %q", name)
		}
	}
	return Event{}, io.EOF
}

// readEvent reads one server-sent event: its event line and data lines, up
// to the blank line that ends it.
func (s *Stream) readEvent() (name, data string, err error) {
	var lines []string
	for {
		line, err := s.reader.ReadString('\n')
		if err != nil {
			if err == io.EOF && (name != "" || len(lines) > 0) {
				return "", "", io.ErrUnexpectedEOF
			}
			return "", "", err
		}
		line = strings.TrimRight(line, "\r\n")
		switch {
		case line == "":
			if name != "" || len(lines) > 0 {
				return name, strings.Join(lines, "\n"), nil
			}
		case strings.HasPrefix(line, "event:"):
			name = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			lines = append(lines, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
}

// Close closes the stream's connection.
func (s *Stream) Close() error {
	return s.body.Close()
}
