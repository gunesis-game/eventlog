package googletest

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gunesis-game/eventlog/internal/drive"
	"github.com/gunesis-game/eventlog/internal/firebase"
	"github.com/gunesis-game/eventlog/internal/googleauth"
)

// The emulator's tests make every request a correct client makes through
// eventlog's service clients, written from Google's contracts (D-081), and
// use raw HTTP only for requests no correct client sends.

const (
	owner    = "owner@example.com"
	editor   = "editor@example.com"
	viewer   = "viewer@example.com"
	stranger = "stranger@example.com"
	loopback = "http://127.0.0.1:8765"
)

var ctx = context.Background()

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

// emulatedClient reads the emulator's client file, as a device reads the
// owner's.
func emulatedClient(t *testing.T, e *Emulator) googleauth.Client {
	t.Helper()
	path := filepath.Join(t.TempDir(), "oauth-client.json")
	if err := e.WriteClientFile(path); err != nil {
		t.Fatal(err)
	}
	client, err := googleauth.ReadClientFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

var allScopes = []string{googleauth.ScopeDrive, googleauth.ScopeOpenID, googleauth.ScopeEmail}

// signIn runs the whole sign-in of an account for scopes and returns its
// tokens.
func signIn(t *testing.T, e *Emulator, email string, scopes ...string) googleauth.Token {
	t.Helper()
	client := emulatedClient(t, e)
	a := client.Authorize(loopback, scopes, email)
	redirect, err := e.Browse(a.URL, email)
	if err != nil {
		t.Fatal(err)
	}
	code, err := a.Code(redirect)
	if err != nil {
		t.Fatalf("sign-in of %s: %v", email, err)
	}
	token, err := client.Exchange(ctx, http.DefaultClient, a, code)
	if err != nil {
		t.Fatalf("code exchange for %s: %v", email, err)
	}
	return token
}

// driveWith returns a Drive client pointed at the emulator using an access
// token.
func driveWith(e *Emulator, accessToken string) *drive.Client {
	return drive.New(http.DefaultClient, e.URL(), func(context.Context) (string, error) { return accessToken, nil })
}

// driveAs signs an account in with the drive scope and returns its Drive
// client.
func driveAs(t *testing.T, e *Emulator, email string) *drive.Client {
	t.Helper()
	return driveWith(e, signIn(t, e, email, googleauth.ScopeDrive).AccessToken)
}

// sharedFolder sets up an owner's tagged folder shared with an editor and a
// viewer, each signed in, and returns their Drive clients and the folder.
func sharedFolder(t *testing.T, e *Emulator) (ownerDrive, editorDrive, viewerDrive *drive.Client, folder string) {
	t.Helper()
	for _, account := range []string{owner, editor, viewer} {
		e.AddTestUser(account)
	}
	ownerDrive, editorDrive, viewerDrive = driveAs(t, e, owner), driveAs(t, e, editor), driveAs(t, e, viewer)
	created, err := ownerDrive.CreateFolder(ctx, "eventlog", "root", map[string]string{"eventlog": "root"})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Share(created.ID, editor, roleWriter); err != nil {
		t.Fatal(err)
	}
	if err := e.Share(created.ID, viewer, roleReader); err != nil {
		t.Fatal(err)
	}
	return ownerDrive, editorDrive, viewerDrive, created.ID
}

// mustCreate creates a file with content and returns its ID.
func mustCreate(t *testing.T, d *drive.Client, name, parent string, content []byte) string {
	t.Helper()
	f, err := d.Create(ctx, name, parent, content)
	if err != nil {
		t.Fatalf("create %s: %v", name, err)
	}
	return f.ID
}

func names(files []drive.File) []string {
	var out []string
	for _, f := range files {
		out = append(out, f.Name)
	}
	return out
}

// expectDriveError checks err is a Drive error with the status and reason.
func expectDriveError(t *testing.T, what string, err error, status int, reason string) {
	t.Helper()
	var driveErr *drive.Error
	if !errors.As(err, &driveErr) || driveErr.Status != status || driveErr.Reason != reason {
		t.Fatalf("%s: %v, want %d %s", what, err, status, reason)
	}
}

// tokenRequest posts a raw form to the token endpoint, for requests no
// correct client sends.
func tokenRequest(t *testing.T, e *Emulator, form url.Values) (int, map[string]any) {
	t.Helper()
	response, err := http.PostForm(e.URL()+"/token", form)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var body map[string]any
	_ = json.NewDecoder(response.Body).Decode(&body)
	return response.StatusCode, body
}

// firebaseAs signs an account in to Google and Firebase and returns its
// Firebase Authentication client and session.
func firebaseAs(t *testing.T, e *Emulator, email string) (*firebase.Auth, firebase.Session) {
	t.Helper()
	_, apiKey := e.Firebase()
	auth := firebase.NewAuth(http.DefaultClient, apiKey, e.URL(), e.URL())
	session, err := auth.SignInWithGoogle(ctx, signIn(t, e, email, allScopes...).IDToken)
	if err != nil {
		t.Fatalf("Firebase sign-in of %s: %v", email, err)
	}
	return auth, session
}

// databaseOf returns a client of the emulator's database.
func databaseOf(e *Emulator) *firebase.Database {
	databaseURL, _ := e.Firebase()
	return firebase.NewDatabase(http.DefaultClient, databaseURL)
}

// expectDatabaseError checks err is a database error with the status.
func expectDatabaseError(t *testing.T, what string, err error, status int) {
	t.Helper()
	var dbErr *firebase.DatabaseError
	if !errors.As(err, &dbErr) || dbErr.Status != status {
		t.Fatalf("%s: %v, want status %d", what, err, status)
	}
}

// claimsOf decodes a token's claims without checking it.
func claimsOf(t *testing.T, token string) map[string]any {
	t.Helper()
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("not a JWT: %q", token)
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatal(err)
	}
	var claims map[string]any
	_ = json.Unmarshal(raw, &claims)
	return claims
}

func jsonOf(value any) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}
