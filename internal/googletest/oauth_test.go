package googletest

import (
	"errors"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/gunesis-game/eventlog/internal/drive"
	"github.com/gunesis-game/eventlog/internal/googleauth"
)

// The browser step redirects to the loopback address with a code and the
// state; the code is exchanged once for an access token and, as for every
// installed app, a refresh token; the access token makes Drive calls.
func TestSignInIssuesTokens(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	e.AddTestUser(owner)
	client := emulatedClient(t, e)
	a := client.Authorize(loopback, []string{googleauth.ScopeDrive}, owner)
	redirect, err := e.Browse(a.URL, owner)
	if err != nil {
		t.Fatal(err)
	}
	code, err := a.Code(redirect)
	if err != nil {
		t.Fatal(err)
	}
	token, err := client.Exchange(ctx, http.DefaultClient, a, code)
	if err != nil {
		t.Fatal(err)
	}
	if token.RefreshToken == "" || token.Scope != googleauth.ScopeDrive || token.IDToken != "" || time.Until(token.Expiry) < 59*time.Minute {
		t.Fatalf("token = %+v", token)
	}
	if root, err := driveWith(e, token.AccessToken).Get(ctx, "root"); err != nil || root.Name != "My Drive" || root.Owners[0].EmailAddress != owner {
		t.Fatalf("My Drive = %+v, %v", root, err)
	}
	_, err = client.Exchange(ctx, http.DefaultClient, a, code)
	var tokenErr *googleauth.Error
	if !errors.As(err, &tokenErr) || tokenErr.Code != "invalid_grant" {
		t.Fatalf("second use of a code = %v", err)
	}
}

// Only test users may sign in; any other account is sent back with
// access_denied.
func TestSignInRefusesAccountsNotListedAsTestUsers(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	e.AddAccount(stranger)
	a := emulatedClient(t, e).Authorize(loopback, []string{googleauth.ScopeDrive}, "")
	redirect, err := e.Browse(a.URL, stranger)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Code(redirect); !errors.Is(err, googleauth.ErrAccessDenied) {
		t.Fatalf("not a test user: %v", err)
	}
}

// A code exchange is refused for a wrong client secret, another redirect
// address, or a wrong PKCE verifier; these are requests no correct client
// sends, so they are made raw.
func TestCodeExchangeChecksTheRequest(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	e.AddTestUser(owner)
	client := emulatedClient(t, e)
	for _, c := range []struct {
		name  string
		form  url.Values
		code  int
		error string
	}{
		{"wrong secret", url.Values{"client_secret": {"wrong"}, "redirect_uri": {loopback}}, http.StatusUnauthorized, "invalid_client"},
		{"other redirect", url.Values{"redirect_uri": {"http://127.0.0.1:9999"}}, http.StatusBadRequest, "redirect_uri_mismatch"},
		{"wrong verifier", url.Values{"redirect_uri": {loopback}, "code_verifier": {"another-verifier"}}, http.StatusBadRequest, "invalid_grant"},
	} {
		a := client.Authorize(loopback, []string{googleauth.ScopeDrive}, owner)
		redirect, err := e.Browse(a.URL, owner)
		if err != nil {
			t.Fatal(err)
		}
		c.form.Set("grant_type", "authorization_code")
		c.form.Set("client_id", client.ID)
		c.form.Set("code", redirect.Get("code"))
		if status, body := tokenRequest(t, e, c.form); status != c.code || body["error"] != c.error {
			t.Fatalf("%s: %d %v, want %d %s", c.name, status, body, c.code, c.error)
		}
	}
}

// An access token lasts an hour, after which Drive refuses it with 401
// authError and a refresh issues a new one.
func TestAccessTokenExpiresAfterAnHour(t *testing.T) {
	clock := newTestClock()
	e := startEmulator(t, t.TempDir(), Options{Now: clock.Now})
	e.AddTestUser(owner)
	token := signIn(t, e, owner, googleauth.ScopeDrive)
	d := driveWith(e, token.AccessToken)
	clock.Advance(time.Hour - time.Second)
	if _, err := d.Get(ctx, "root"); err != nil {
		t.Fatalf("before expiry: %v", err)
	}
	clock.Advance(time.Second)
	_, err := d.Get(ctx, "root")
	expectDriveError(t, "after expiry", err, http.StatusUnauthorized, drive.ReasonAuthError)
	refreshed, err := emulatedClient(t, e).Refresh(ctx, http.DefaultClient, token.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := driveWith(e, refreshed.AccessToken).Get(ctx, "root"); err != nil {
		t.Fatalf("with the refreshed token: %v", err)
	}
}

// In Testing status a refresh token expires seven days after consent, and a
// new sign-in is needed.
func TestRefreshTokenExpiresSevenDaysAfterConsent(t *testing.T) {
	clock := newTestClock()
	e := startEmulator(t, t.TempDir(), Options{Now: clock.Now})
	e.AddTestUser(owner)
	token := signIn(t, e, owner, googleauth.ScopeDrive)
	client := emulatedClient(t, e)
	clock.Advance(7*24*time.Hour - time.Minute)
	if _, err := client.Refresh(ctx, http.DefaultClient, token.RefreshToken); err != nil {
		t.Fatalf("before seven days: %v", err)
	}
	clock.Advance(time.Minute)
	if _, err := client.Refresh(ctx, http.DefaultClient, token.RefreshToken); !errors.Is(err, googleauth.ErrReapprovalNeeded) {
		t.Fatalf("after seven days: %v", err)
	}
	if renewed := signIn(t, e, owner, googleauth.ScopeDrive); renewed.RefreshToken == "" {
		t.Fatal("a new sign-in gave no refresh token")
	}
}

// Every Drive call needs the full drive scope; a token for drive.file alone
// is refused, as the feasibility check found, and an unknown token is not
// authenticated.
func TestDriveNeedsTheFullDriveScope(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	e.AddTestUser(owner)
	narrow := signIn(t, e, owner, "https://www.googleapis.com/auth/drive.file")
	_, err := driveWith(e, narrow.AccessToken).Get(ctx, "root")
	expectDriveError(t, "drive.file token", err, http.StatusForbidden, "insufficientPermissions")
	_, err = driveWith(e, "not-a-token").Get(ctx, "root")
	expectDriveError(t, "unknown token", err, http.StatusUnauthorized, drive.ReasonAuthError)
}
