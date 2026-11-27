package googletest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

// testClock is a clock a test moves forward by hand.
type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func newTestClock() *testClock {
	return &testClock{now: time.Date(2026, 11, 27, 9, 0, 0, 0, time.UTC)}
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func startEmulator(t *testing.T, dir string, options Options) *Emulator {
	t.Helper()
	e, err := New(dir, options)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = e.Close()
	})
	return e
}

// noRedirects is an HTTP client that returns redirects instead of following
// them, as the browser step's response is read here.
var noRedirects = &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

const loopback = "http://127.0.0.1:8765/"

// authorize runs the browser step for the account signing in and returns
// the query of the loopback redirect.
func authorize(t *testing.T, e *Emulator, scope, challenge string) url.Values {
	t.Helper()
	clientID, _ := e.Client()
	q := url.Values{"client_id": {clientID}, "redirect_uri": {loopback}, "response_type": {"code"}, "scope": {scope}, "state": {"s1"}, "access_type": {"offline"}, "prompt": {"consent"}}
	if challenge != "" {
		q.Set("code_challenge", challenge)
		q.Set("code_challenge_method", "S256")
	}
	response, err := noRedirects.Get(e.URL() + "/o/oauth2/v2/auth?" + q.Encode())
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusFound {
		t.Fatalf("authorization status %d", response.StatusCode)
	}
	location, err := url.Parse(response.Header.Get("Location"))
	if err != nil || !strings.HasPrefix(location.String(), loopback) {
		t.Fatalf("redirect to %q", response.Header.Get("Location"))
	}
	return location.Query()
}

// tokenRequest posts to the token endpoint with the emulator's client
// credentials and returns the status and decoded body.
func tokenRequest(t *testing.T, e *Emulator, form url.Values) (int, map[string]any) {
	t.Helper()
	clientID, secret := e.Client()
	if !form.Has("client_id") {
		form.Set("client_id", clientID)
		form.Set("client_secret", secret)
	}
	response, err := http.PostForm(e.URL()+"/token", form)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var body map[string]any
	_ = json.NewDecoder(response.Body).Decode(&body)
	return response.StatusCode, body
}

const verifier = "a-code-verifier-of-enough-length-for-pkce-0123456789"

// signIn signs an account in with the given scope and returns a client
// holding its access token and its refresh token.
func signIn(t *testing.T, e *Emulator, email, scope string) (*driveClient, string) {
	t.Helper()
	e.SignIn(email)
	redirect := authorize(t, e, scope, challengeOf(verifier))
	status, body := tokenRequest(t, e, url.Values{"grant_type": {"authorization_code"}, "code": {redirect.Get("code")}, "redirect_uri": {loopback}, "code_verifier": {verifier}})
	if status != http.StatusOK {
		t.Fatalf("code exchange for %s: %d %v", email, status, body)
	}
	refresh, _ := body["refresh_token"].(string)
	return &driveClient{t: t, base: e.URL(), token: body["access_token"].(string)}, refresh
}

// driveClient makes Drive calls with one account's access token.
type driveClient struct {
	t     *testing.T
	base  string
	token string
}

func (c *driveClient) do(method, path string, body io.Reader, header http.Header) (int, http.Header, []byte) {
	c.t.Helper()
	request, err := http.NewRequest(method, c.base+path, body)
	if err != nil {
		c.t.Fatal(err)
	}
	for key, values := range header {
		request.Header[key] = values
	}
	request.Header.Set("Authorization", "Bearer "+c.token)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		c.t.Fatal(err)
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		c.t.Fatal(err)
	}
	return response.StatusCode, response.Header, raw
}

// call sends a JSON body (or none) and decodes a JSON response.
func (c *driveClient) call(method, path string, body any) (int, map[string]any) {
	c.t.Helper()
	var reader io.Reader
	header := http.Header{}
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
		header.Set("Content-Type", "application/json")
	}
	status, _, raw := c.do(method, path, reader, header)
	var decoded map[string]any
	_ = json.Unmarshal(raw, &decoded)
	return status, decoded
}

// createFolder creates a folder in parent ("" for My Drive) and returns its
// ID.
func (c *driveClient) createFolder(name, parent string, properties map[string]string) string {
	c.t.Helper()
	metadata := map[string]any{"name": name, "mimeType": folderMimeType}
	if parent != "" {
		metadata["parents"] = []string{parent}
	}
	if properties != nil {
		metadata["appProperties"] = properties
	}
	status, body := c.call("POST", "/drive/v3/files", metadata)
	if status != http.StatusOK {
		c.t.Fatalf("create folder %s: %d %v", name, status, body)
	}
	return body["id"].(string)
}

// upload creates a file with content in parent with a multipart upload, and
// returns the status and response.
func (c *driveClient) upload(name, parent string, content []byte) (int, map[string]any) {
	c.t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	metadata, _ := writer.CreatePart(textproto.MIMEHeader{"Content-Type": {"application/json; charset=UTF-8"}})
	raw, _ := json.Marshal(map[string]any{"name": name, "parents": []string{parent}})
	_, _ = metadata.Write(raw)
	media, _ := writer.CreatePart(textproto.MIMEHeader{"Content-Type": {"application/octet-stream"}})
	_, _ = media.Write(content)
	_ = writer.Close()
	status, _, response := c.do("POST", "/upload/drive/v3/files?uploadType=multipart", &body, http.Header{"Content-Type": {"multipart/related; boundary=" + writer.Boundary()}})
	var decoded map[string]any
	_ = json.Unmarshal(response, &decoded)
	return status, decoded
}

// mustUpload uploads a file and returns its ID.
func (c *driveClient) mustUpload(name, parent string, content []byte) string {
	c.t.Helper()
	status, body := c.upload(name, parent, content)
	if status != http.StatusOK {
		c.t.Fatalf("upload %s: %d %v", name, status, body)
	}
	return body["id"].(string)
}

// replaceContent uploads new content for a file, replacing all of it.
func (c *driveClient) replaceContent(id string, content []byte) (int, map[string]any) {
	c.t.Helper()
	status, _, raw := c.do("PATCH", "/upload/drive/v3/files/"+id+"?uploadType=media", bytes.NewReader(content), http.Header{"Content-Type": {"application/octet-stream"}})
	var decoded map[string]any
	_ = json.Unmarshal(raw, &decoded)
	return status, decoded
}

// read reads a file's content, in a byte range when rangeHeader is set.
func (c *driveClient) read(id, rangeHeader string) (int, []byte) {
	c.t.Helper()
	header := http.Header{}
	if rangeHeader != "" {
		header.Set("Range", rangeHeader)
	}
	status, _, raw := c.do("GET", "/drive/v3/files/"+id+"?alt=media", nil, header)
	return status, raw
}

// list runs files.list with a query and returns the file names.
func (c *driveClient) list(query string) []string {
	c.t.Helper()
	status, body := c.call("GET", "/drive/v3/files?"+url.Values{"q": {query}, "fields": {"files(name)"}}.Encode(), nil)
	if status != http.StatusOK {
		c.t.Fatalf("list %q: %d %v", query, status, body)
	}
	var names []string
	for _, f := range body["files"].([]any) {
		names = append(names, f.(map[string]any)["name"].(string))
	}
	return names
}

// reason returns the first error reason of a Drive error response.
func reason(body map[string]any) string {
	e, _ := body["error"].(map[string]any)
	errs, _ := e["errors"].([]any)
	if len(errs) == 0 {
		return ""
	}
	r, _ := errs[0].(map[string]any)["reason"].(string)
	return r
}

func expectStatus(t *testing.T, what string, status, want int, body any) {
	t.Helper()
	if status != want {
		t.Fatalf("%s: status %d, want %d: %v", what, status, want, body)
	}
}

func expectRefused(t *testing.T, what string, status int, body map[string]any, wantStatus int, wantReason string) {
	t.Helper()
	if status != wantStatus || reason(body) != wantReason {
		t.Fatalf("%s: %d %q, want %d %q: %v", what, status, reason(body), wantStatus, wantReason, body)
	}
}

func q(format string, args ...any) string {
	return fmt.Sprintf(format, args...)
}
