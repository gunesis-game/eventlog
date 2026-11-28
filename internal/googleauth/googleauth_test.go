package googleauth_test

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gunesis-game/eventlog/internal/googleauth"
	"github.com/gunesis-game/eventlog/internal/googletest"
)

const (
	owner    = "owner@example.com"
	loopback = "http://127.0.0.1:8765"
)

type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// emulated starts the emulator and reads its client file, as a device reads
// the owner's.
func emulated(t *testing.T, options googletest.Options) (*googletest.Emulator, googleauth.Client) {
	t.Helper()
	e, err := googletest.New(t.TempDir(), options)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.Close() })
	path := filepath.Join(t.TempDir(), "oauth-client.json")
	if err := e.WriteClientFile(path); err != nil {
		t.Fatal(err)
	}
	client, err := googleauth.ReadClientFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return e, client
}

var scopes = []string{googleauth.ScopeDrive, googleauth.ScopeOpenID, googleauth.ScopeEmail}

// signIn runs the whole flow: the authorization URL, the browser step, the
// redirect's code, and the exchange.
func signIn(t *testing.T, e *googletest.Emulator, client googleauth.Client, email string) googleauth.Token {
	t.Helper()
	a := client.Authorize(loopback, scopes, email)
	redirect, err := e.Browse(a.URL, email)
	if err != nil {
		t.Fatal(err)
	}
	code, err := a.Code(redirect)
	if err != nil {
		t.Fatal(err)
	}
	token, err := client.Exchange(context.Background(), http.DefaultClient, a, code)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

// A Desktop-app client file names the client and, in a full download, the
// endpoints; Google's are used when it names none.
func TestReadClientFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "client.json")
	if err := os.WriteFile(path, []byte(`{"installed":{"client_id":"id.apps.googleusercontent.com","client_secret":"s"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	client, err := googleauth.ReadClientFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if client.ID != "id.apps.googleusercontent.com" || client.Secret != "s" || client.AuthURI != "https://accounts.google.com/o/oauth2/v2/auth" || client.TokenURI != "https://oauth2.googleapis.com/token" {
		t.Fatalf("client = %+v", client)
	}
	if err := os.WriteFile(path, []byte(`{"web":{"client_id":"x"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := googleauth.ReadClientFile(path); err == nil || !strings.Contains(err.Error(), "Desktop-app") {
		t.Fatalf("a web client = %v", err)
	}
}

// The authorization URL carries the documented parameters, PKCE with S256,
// and a state.
func TestAuthorizationURL(t *testing.T) {
	client := googleauth.Client{ID: "id", AuthURI: "https://accounts.google.com/o/oauth2/v2/auth"}
	a := client.Authorize(loopback, scopes, owner)
	parsed, err := url.Parse(a.URL)
	if err != nil {
		t.Fatal(err)
	}
	q := parsed.Query()
	if parsed.Host != "accounts.google.com" || q.Get("client_id") != "id" || q.Get("redirect_uri") != loopback || q.Get("response_type") != "code" ||
		q.Get("scope") != strings.Join(scopes, " ") || q.Get("code_challenge_method") != "S256" || len(q.Get("code_challenge")) != 43 || q.Get("state") == "" || q.Get("login_hint") != owner {
		t.Fatalf("authorization query = %v", q)
	}
}

// The whole sign-in against the emulator gives an access token, a refresh
// token, and a Google ID token; a refresh gives a new access token and keeps
// the refresh token.
func TestSignInAndRefresh(t *testing.T) {
	e, client := emulated(t, googletest.Options{})
	e.AddTestUser(owner)
	token := signIn(t, e, client, owner)
	if token.AccessToken == "" || token.RefreshToken == "" || token.IDToken == "" || time.Until(token.Expiry) < 50*time.Minute {
		t.Fatalf("token = %+v", token)
	}
	refreshed, err := client.Refresh(context.Background(), http.DefaultClient, token.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.AccessToken == "" || refreshed.AccessToken == token.AccessToken || refreshed.RefreshToken != token.RefreshToken {
		t.Fatalf("refreshed = %+v", refreshed)
	}
}

// An account that is not a test user is refused at the browser; a redirect
// whose state differs is rejected.
func TestSignInRefusals(t *testing.T) {
	e, client := emulated(t, googletest.Options{})
	e.AddAccount("stranger@example.com")
	a := client.Authorize(loopback, scopes, "")
	redirect, err := e.Browse(a.URL, "stranger@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Code(redirect); !errors.Is(err, googleauth.ErrAccessDenied) {
		t.Fatalf("not a test user: %v", err)
	}
	other := client.Authorize(loopback, scopes, "")
	if _, err := other.Code(url.Values{"state": {"forged"}, "code": {"x"}}); err == nil {
		t.Fatal("a redirect with another state was accepted")
	}
}

// Seven days after consent, in Testing status, the refresh token stops
// working and the person must sign in again.
func TestReapprovalAfterSevenDays(t *testing.T) {
	c := &clock{now: time.Now()}
	e, client := emulated(t, googletest.Options{Now: c.Now})
	e.AddTestUser(owner)
	token := signIn(t, e, client, owner)
	c.Advance(7 * 24 * time.Hour)
	_, err := client.Refresh(context.Background(), http.DefaultClient, token.RefreshToken)
	if !errors.Is(err, googleauth.ErrReapprovalNeeded) {
		t.Fatalf("after seven days: %v", err)
	}
}
