package googletest

import (
	"net/http"
	"net/url"
	"testing"
	"time"
)

const (
	owner  = "owner@example.com"
	editor = "editor@example.com"
	viewer = "viewer@example.com"
)

// The browser step redirects to the loopback address with a code and the
// state, the code is exchanged once for an access and a refresh token, and
// the access token makes Drive calls.
func TestSignInIssuesTokens(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	e.AddTestUser(owner)
	e.SignIn(owner)
	redirect := authorize(t, e, driveScope, challengeOf(verifier))
	if redirect.Get("state") != "s1" || redirect.Get("code") == "" || redirect.Get("error") != "" {
		t.Fatalf("redirect query = %v", redirect)
	}
	exchange := url.Values{"grant_type": {"authorization_code"}, "code": {redirect.Get("code")}, "redirect_uri": {loopback}, "code_verifier": {verifier}}
	status, body := tokenRequest(t, e, exchange)
	expectStatus(t, "code exchange", status, http.StatusOK, body)
	if body["token_type"] != "Bearer" || body["scope"] != driveScope || body["refresh_token"] == nil || body["expires_in"] != float64(3599) {
		t.Fatalf("token response = %v", body)
	}
	drive := &driveClient{t: t, base: e.URL(), token: body["access_token"].(string)}
	status, root := drive.call("GET", "/drive/v3/files/root?fields=name,ownedByMe", nil)
	if status != http.StatusOK || root["name"] != "My Drive" || root["ownedByMe"] != true {
		t.Fatalf("My Drive = %d %v", status, root)
	}
	status, body = tokenRequest(t, e, exchange)
	if status != http.StatusBadRequest || body["error"] != "invalid_grant" {
		t.Fatalf("second use of a code = %d %v", status, body)
	}
}

// Only test users may sign in; any other account is sent back with
// access_denied.
func TestSignInRefusesAccountsNotListedAsTestUsers(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	e.AddAccount("stranger@example.com")
	e.SignIn("stranger@example.com")
	redirect := authorize(t, e, driveScope, "")
	if redirect.Get("error") != "access_denied" || redirect.Get("code") != "" || redirect.Get("state") != "s1" {
		t.Fatalf("redirect query = %v", redirect)
	}
}

// A code exchange is refused for another client, another redirect address,
// or a wrong PKCE verifier.
func TestCodeExchangeChecksTheRequest(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	e.AddTestUser(owner)
	e.SignIn(owner)
	clientID, _ := e.Client()
	for _, c := range []struct {
		name  string
		form  url.Values
		code  int
		error string
	}{
		{"wrong secret", url.Values{"client_id": {clientID}, "client_secret": {"wrong"}, "redirect_uri": {loopback}, "code_verifier": {verifier}}, http.StatusUnauthorized, "invalid_client"},
		{"other redirect", url.Values{"redirect_uri": {"http://127.0.0.1:9999/"}, "code_verifier": {verifier}}, http.StatusBadRequest, "redirect_uri_mismatch"},
		{"wrong verifier", url.Values{"redirect_uri": {loopback}, "code_verifier": {"another-verifier"}}, http.StatusBadRequest, "invalid_grant"},
	} {
		redirect := authorize(t, e, driveScope, challengeOf(verifier))
		c.form.Set("grant_type", "authorization_code")
		c.form.Set("code", redirect.Get("code"))
		if status, body := tokenRequest(t, e, c.form); status != c.code || body["error"] != c.error {
			t.Fatalf("%s: %d %v, want %d %s", c.name, status, body, c.code, c.error)
		}
	}
}

// An access token lasts an hour, after which Drive refuses it with 401 and
// a refresh issues a new one.
func TestAccessTokenExpiresAfterAnHour(t *testing.T) {
	clock := newTestClock()
	e := startEmulator(t, t.TempDir(), Options{Now: clock.Now})
	e.AddTestUser(owner)
	drive, refresh := signIn(t, e, owner, driveScope)
	clock.Advance(time.Hour - time.Second)
	if status, body := drive.call("GET", "/drive/v3/files/root", nil); status != http.StatusOK {
		t.Fatalf("before expiry: %d %v", status, body)
	}
	clock.Advance(time.Second)
	status, body := drive.call("GET", "/drive/v3/files/root", nil)
	expectRefused(t, "after expiry", status, body, http.StatusUnauthorized, "authError")

	status, body = tokenRequest(t, e, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}})
	expectStatus(t, "refresh", status, http.StatusOK, body)
	if body["refresh_token"] != nil {
		t.Fatalf("a refresh returned a new refresh token: %v", body)
	}
	drive.token = body["access_token"].(string)
	if status, body := drive.call("GET", "/drive/v3/files/root", nil); status != http.StatusOK {
		t.Fatalf("with the refreshed token: %d %v", status, body)
	}
}

// In Testing status a refresh token expires seven days after consent, and a
// new sign-in is needed.
func TestRefreshTokenExpiresSevenDaysAfterConsent(t *testing.T) {
	clock := newTestClock()
	e := startEmulator(t, t.TempDir(), Options{Now: clock.Now})
	e.AddTestUser(owner)
	_, refresh := signIn(t, e, owner, driveScope)
	refreshing := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}}
	clock.Advance(7*24*time.Hour - time.Minute)
	if status, body := tokenRequest(t, e, refreshing); status != http.StatusOK {
		t.Fatalf("before seven days: %d %v", status, body)
	}
	clock.Advance(time.Minute)
	status, body := tokenRequest(t, e, refreshing)
	if status != http.StatusBadRequest || body["error"] != "invalid_grant" || body["error_description"] != "Token has been expired or revoked." {
		t.Fatalf("after seven days: %d %v", status, body)
	}
	if _, renewed := signIn(t, e, owner, driveScope); renewed == "" {
		t.Fatal("a new sign-in gave no refresh token")
	}
}

// Every Drive call needs the full drive scope; a token for drive.file alone
// is refused, as the feasibility check found.
func TestDriveNeedsTheFullDriveScope(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	e.AddTestUser(owner)
	drive, _ := signIn(t, e, owner, "https://www.googleapis.com/auth/drive.file")
	status, body := drive.call("GET", "/drive/v3/files/root", nil)
	expectRefused(t, "drive.file token", status, body, http.StatusForbidden, "insufficientPermissions")
	drive.token = "not-a-token"
	status, body = drive.call("GET", "/drive/v3/files/root", nil)
	expectRefused(t, "unknown token", status, body, http.StatusUnauthorized, "authError")
}
