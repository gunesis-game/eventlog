package googletest

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	metadataFileName = "metadata.json"
	contentDirName   = "content"
	folderMimeType   = "application/vnd.google-apps.folder"
	driveScope       = "https://www.googleapis.com/auth/drive"
	accessTokenLife  = time.Hour
	// refreshTokenLife is how long a refresh token lasts in Testing status:
	// seven days from consent.
	refreshTokenLife = 7 * 24 * time.Hour
	codeLife         = 10 * time.Minute
)

// Options configures an emulator.
type Options struct {
	// Now reads the emulator's clock; time.Now when nil. Tests set it to move
	// time forward, as for token expiry.
	Now func() time.Time
	// Latency is added to every call before it is handled, to model network
	// latency.
	Latency time.Duration
}

// Emulator is a running emulator of Google's OAuth and Drive services.
type Emulator struct {
	dir      string
	options  Options
	listener net.Listener
	server   *http.Server

	mu    sync.Mutex
	state *state
	// signingIn is the account signing in at the browser for the next
	// authorization request.
	signingIn string
	stats     map[string]map[string]*CallStats
}

// state is everything the emulator keeps in its metadata file.
type state struct {
	Client   oauthClient              `json:"client"`
	Accounts map[string]*account      `json:"accounts"`
	Files    map[string]*file         `json:"files"`
	Codes    map[string]*authCode     `json:"codes"`
	Refresh  map[string]*refreshToken `json:"refresh_tokens"`
	Access   map[string]*accessToken  `json:"access_tokens"`
}

type oauthClient struct {
	ID     string `json:"id"`
	Secret string `json:"secret"`
}

// account is a Google account known to the emulator, keyed by its lowercased
// email address.
type account struct {
	Email        string `json:"email"`
	TestUser     bool   `json:"test_user"`
	RootID       string `json:"root_id"`
	PermissionID string `json:"permission_id"`
}

// file is one Drive file or folder's metadata.
type file struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	MimeType      string            `json:"mime_type"`
	Parents       []string          `json:"parents"`
	Owner         string            `json:"owner"`
	AppProperties map[string]string `json:"app_properties"`
	Trashed       bool              `json:"trashed"`
	Size          int64             `json:"size"`
	CreatedTime   time.Time         `json:"created_time"`
	ModifiedTime  time.Time         `json:"modified_time"`
	// Permissions are the file's own permissions, not those inherited from
	// its folders; the owner's is implied by Owner.
	Permissions []*permission `json:"permissions"`
}

// permission is a direct permission of one account on a file.
type permission struct {
	Email        string `json:"email"`
	Role         string `json:"role"`
	PendingOwner bool   `json:"pending_owner"`
}

// CallStats counts the requests of one call by one account.
type CallStats struct {
	Requests      int
	RequestBytes  int64
	ResponseBytes int64
}

// New starts an emulator keeping its state in dir, continuing from the state
// already there.
func New(dir string, options Options) (*Emulator, error) {
	if options.Now == nil {
		options.Now = time.Now
	}
	if err := os.MkdirAll(filepath.Join(dir, contentDirName), 0o755); err != nil {
		return nil, err
	}
	e := &Emulator{dir: dir, options: options, stats: make(map[string]map[string]*CallStats)}
	if err := e.load(); err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	e.listener = listener
	e.server = &http.Server{Handler: e.routes(), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		_ = e.server.Serve(listener)
	}()
	return e, nil
}

// URL is the base address of every emulated service.
func (e *Emulator) URL() string {
	return "http://" + e.listener.Addr().String()
}

// Close stops the emulator at once, closing its connections. Every change is
// saved as it is made, so its state stays in its folder.
func (e *Emulator) Close() error {
	return e.server.Close()
}

// Client returns the OAuth client registered with the emulator, as the
// client ID and secret of an OAuth client file.
func (e *Emulator) Client() (id, secret string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.state.Client.ID, e.state.Client.Secret
}

// AddTestUser adds a Google account listed as a test user of the app, which
// may sign in.
func (e *Emulator) AddTestUser(email string) {
	e.addAccount(email, true)
}

// AddAccount adds a Google account that is not a test user of the app, which
// is refused at sign-in.
func (e *Emulator) AddAccount(email string) {
	e.addAccount(email, false)
}

func (e *Emulator) addAccount(email string, testUser bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	key := strings.ToLower(email)
	if existing := e.state.Accounts[key]; existing != nil {
		existing.TestUser = testUser
		e.saveLocked()
		return
	}
	now := e.options.Now()
	root := &file{ID: newID(), Name: "My Drive", MimeType: folderMimeType, Owner: key, CreatedTime: now, ModifiedTime: now}
	e.state.Files[root.ID] = root
	e.state.Accounts[key] = &account{Email: email, TestUser: testUser, RootID: root.ID, PermissionID: newDigits(20)}
	e.saveLocked()
}

// SignIn sets the account that signs in at the browser for the following
// authorization requests, as a person choosing an account on Google's
// sign-in page.
func (e *Emulator) SignIn(email string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.signingIn = strings.ToLower(email)
}

// Share gives an account a role ("writer" or "reader") on a file or folder,
// as its owner does in Drive's sharing dialog.
func (e *Emulator) Share(fileID, email, role string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	f := e.state.Files[fileID]
	if f == nil {
		return fmt.Errorf("no file %s", fileID)
	}
	key := strings.ToLower(email)
	if e.state.Accounts[key] == nil {
		return fmt.Errorf("no account %s", email)
	}
	f.setPermission(key, role)
	e.saveLocked()
	return nil
}

// Stats returns a copy of the call counts, per account and per call name,
// such as "drive.files.list". Calls made without a known account are counted
// under the empty account.
func (e *Emulator) Stats() map[string]map[string]CallStats {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make(map[string]map[string]CallStats, len(e.stats))
	for account, calls := range e.stats {
		out[account] = make(map[string]CallStats, len(calls))
		for call, s := range calls {
			out[account][call] = *s
		}
	}
	return out
}

// ResetStats clears the call counts.
func (e *Emulator) ResetStats() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.stats = make(map[string]map[string]*CallStats)
}

// load reads the metadata file, or starts a fresh state with a new OAuth
// client when there is none.
func (e *Emulator) load() error {
	raw, err := os.ReadFile(filepath.Join(e.dir, metadataFileName))
	if errors.Is(err, os.ErrNotExist) {
		e.state = &state{
			Client:   oauthClient{ID: newDigits(12) + "-" + newID() + ".apps.googleusercontent.com", Secret: "GOCSPX-" + newID()},
			Accounts: make(map[string]*account),
			Files:    make(map[string]*file),
			Codes:    make(map[string]*authCode),
			Refresh:  make(map[string]*refreshToken),
			Access:   make(map[string]*accessToken),
		}
		return e.writeState()
	}
	if err != nil {
		return err
	}
	var s state
	if err := json.Unmarshal(raw, &s); err != nil {
		return fmt.Errorf("emulator metadata: %w", err)
	}
	e.state = &s
	return nil
}

// saveLocked writes the metadata file. A failure is a broken test setup, so
// it panics rather than letting the emulator drift from its folder.
func (e *Emulator) saveLocked() {
	if err := e.writeState(); err != nil {
		panic(fmt.Sprintf("googletest: save metadata: %v", err))
	}
}

func (e *Emulator) writeState() error {
	raw, err := json.MarshalIndent(e.state, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomically(filepath.Join(e.dir, metadataFileName), raw)
}

func (e *Emulator) contentPath(id string) string {
	return filepath.Join(e.dir, contentDirName, id)
}

// writeAtomically replaces path with data through a temporary file. On
// Windows a replace is refused while another process, such as a virus
// scanner, has the file open, so a refused rename is retried for a moment.
func writeAtomically(path string, data []byte) error {
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o644); err != nil {
		return err
	}
	var err error
	for attempt := 0; attempt < 100; attempt++ {
		if err = os.Rename(temporary, path); !errors.Is(err, fs.ErrPermission) {
			return err
		}
		time.Sleep(10 * time.Millisecond)
	}
	return err
}

const idAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"

// newID returns a random identifier in the form of a Drive file ID.
func newID() string {
	return randomString(33, idAlphabet)
}

func newDigits(n int) string {
	return randomString(n, "0123456789")
}

func randomString(n int, alphabet string) string {
	raw := make([]byte, n)
	_, _ = rand.Read(raw)
	for i := range raw {
		raw[i] = alphabet[int(raw[i])%len(alphabet)]
	}
	return string(raw)
}

// call is one request in progress: its name and account, filled in by the
// handler, for counting.
type call struct {
	name    string
	account string
}

type callKey struct{}

// routes counts and delays every call, then dispatches it.
func (e *Emulator) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /o/oauth2/v2/auth", e.authorize)
	mux.HandleFunc("POST /token", e.token)
	mux.HandleFunc("GET /drive/v3/files", e.authorized(e.listFiles))
	mux.HandleFunc("POST /drive/v3/files", e.authorized(e.createFile))
	mux.HandleFunc("POST /upload/drive/v3/files", e.authorized(e.createFile))
	mux.HandleFunc("GET /drive/v3/files/{id}", e.authorized(e.getFile))
	mux.HandleFunc("PATCH /drive/v3/files/{id}", e.authorized(e.updateFile))
	mux.HandleFunc("PATCH /upload/drive/v3/files/{id}", e.authorized(e.updateFile))
	mux.HandleFunc("GET /drive/v3/files/{id}/permissions", e.authorized(e.listPermissions))
	mux.HandleFunc("POST /drive/v3/files/{id}/permissions", e.authorized(e.createPermission))
	mux.HandleFunc("PATCH /drive/v3/files/{id}/permissions/{permissionID}", e.authorized(e.updatePermission))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if e.options.Latency > 0 {
			time.Sleep(e.options.Latency)
		}
		c := &call{name: "unknown"}
		body := &countingReader{reader: r.Body}
		r.Body = body
		counted := &countingWriter{ResponseWriter: w}
		mux.ServeHTTP(counted, r.WithContext(context.WithValue(r.Context(), callKey{}, c)))
		e.count(c, body.n, counted.n)
	})
}

func (e *Emulator) count(c *call, requestBytes, responseBytes int64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	calls := e.stats[c.account]
	if calls == nil {
		calls = make(map[string]*CallStats)
		e.stats[c.account] = calls
	}
	s := calls[c.name]
	if s == nil {
		s = &CallStats{}
		calls[c.name] = s
	}
	s.Requests++
	s.RequestBytes += requestBytes
	s.ResponseBytes += responseBytes
}

// named records the call's name for counting.
func named(r *http.Request, name string) *call {
	c := r.Context().Value(callKey{}).(*call)
	c.name = name
	return c
}

type countingReader struct {
	reader io.ReadCloser
	n      int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.reader.Read(p)
	c.n += int64(n)
	return n, err
}

func (c *countingReader) Close() error {
	return c.reader.Close()
}

type countingWriter struct {
	http.ResponseWriter
	n int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.ResponseWriter.Write(p)
	c.n += int64(n)
	return n, err
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	raw, _ := json.Marshal(value)
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(status)
	_, _ = w.Write(raw)
}

// driveError writes an error in the form of the Drive v3 API.
func driveError(w http.ResponseWriter, code int, reason, message string) {
	status := map[int]string{400: "INVALID_ARGUMENT", 401: "UNAUTHENTICATED", 403: "PERMISSION_DENIED", 404: "NOT_FOUND", 416: "OUT_OF_RANGE"}[code]
	detail := map[string]any{"message": message, "domain": "global", "reason": reason}
	if code == http.StatusUnauthorized {
		detail["location"], detail["locationType"] = "Authorization", "header"
	}
	writeJSON(w, code, map[string]any{"error": map[string]any{"code": code, "message": message, "errors": []any{detail}, "status": status}})
}
