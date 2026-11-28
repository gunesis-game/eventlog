// Package googleauth is a client of Google's OAuth 2.0 for installed
// (desktop) apps, written from Google's documentation of that flow
// (developers.google.com/identity/protocols/oauth2/native-app): an
// authorization URL with PKCE and state for a loopback redirect, the
// redirect's code or error, the code exchange, and token refresh. It knows
// nothing of eventlog's storage. The addresses come from the client file, so
// tests point it at the emulator by giving it the emulator's client file.
package googleauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Google's documented endpoints, used when the client file names none.
const (
	defaultAuthURI  = "https://accounts.google.com/o/oauth2/v2/auth"
	defaultTokenURI = "https://oauth2.googleapis.com/token"
)

// Scopes eventlog's web storage asks for: the full drive scope, and openid
// and email for the Google ID token Firebase exchanges.
const (
	ScopeDrive  = "https://www.googleapis.com/auth/drive"
	ScopeOpenID = "openid"
	ScopeEmail  = "email"
)

// Client is a Desktop-app OAuth client, as in the client file Google Cloud
// downloads.
type Client struct {
	ID       string
	Secret   string
	AuthURI  string
	TokenURI string
}

// ReadClientFile reads a Desktop-app OAuth client file: its "installed"
// section's client_id and client_secret, and auth_uri and token_uri, which
// default to Google's endpoints.
func ReadClientFile(path string) (Client, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Client{}, err
	}
	var file struct {
		Installed *struct {
			ClientID     string `json:"client_id"`
			ClientSecret string `json:"client_secret"`
			AuthURI      string `json:"auth_uri"`
			TokenURI     string `json:"token_uri"`
		} `json:"installed"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		return Client{}, fmt.Errorf("OAuth client file %s: %w", path, err)
	}
	if file.Installed == nil || file.Installed.ClientID == "" {
		return Client{}, fmt.Errorf("OAuth client file %s is not a Desktop-app client: no installed client_id", path)
	}
	c := Client{ID: file.Installed.ClientID, Secret: file.Installed.ClientSecret, AuthURI: file.Installed.AuthURI, TokenURI: file.Installed.TokenURI}
	if c.AuthURI == "" {
		c.AuthURI = defaultAuthURI
	}
	if c.TokenURI == "" {
		c.TokenURI = defaultTokenURI
	}
	return c, nil
}

// Authorization is one sign-in in progress: the URL to open in a browser,
// and what checking its redirect and exchanging its code need.
type Authorization struct {
	// URL is the authorization request to open in a browser.
	URL         string
	redirectURI string
	state       string
	verifier    string
}

// Authorize starts a sign-in that returns to redirectURI, a loopback address
// such as http://127.0.0.1:8765, asking for scopes, with an S256 PKCE
// challenge and a random state. loginHint, when not empty, suggests the
// account to sign in with.
func (c Client) Authorize(redirectURI string, scopes []string, loginHint string) Authorization {
	a := Authorization{redirectURI: redirectURI, state: randomToken(24), verifier: randomToken(48)}
	sum := sha256.Sum256([]byte(a.verifier))
	query := url.Values{
		"client_id":             {c.ID},
		"redirect_uri":          {redirectURI},
		"response_type":         {"code"},
		"scope":                 {strings.Join(scopes, " ")},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(sum[:])},
		"code_challenge_method": {"S256"},
		"state":                 {a.state},
	}
	if loginHint != "" {
		query.Set("login_hint", loginHint)
	}
	a.URL = c.AuthURI + "?" + query.Encode()
	return a
}

// ErrAccessDenied reports that the person at the browser declined, or that
// the account may not sign in, as an account not listed as a test user.
var ErrAccessDenied = errors.New("google sign-in: access denied")

// Code returns the authorization code from the query of the loopback
// redirect, after checking its state.
func (a Authorization) Code(redirect url.Values) (string, error) {
	if redirect.Get("state") != a.state {
		return "", errors.New("google sign-in: the redirect's state does not match the request")
	}
	switch reason := redirect.Get("error"); {
	case reason == "access_denied":
		return "", ErrAccessDenied
	case reason != "":
		return "", fmt.Errorf("google sign-in: %s", reason)
	}
	code := redirect.Get("code")
	if code == "" {
		return "", errors.New("google sign-in: the redirect has no code")
	}
	return code, nil
}

// Token is what the token endpoint returns.
type Token struct {
	AccessToken string
	// RefreshToken is set by a code exchange; a refresh keeps the earlier
	// one.
	RefreshToken string
	// IDToken is set when the openid scope was granted.
	IDToken string
	Scope   string
	Expiry  time.Time
}

// Error is an error response of the token endpoint.
type Error struct {
	Status      int
	Code        string
	Description string
}

func (e *Error) Error() string {
	return fmt.Sprintf("google token endpoint: %d %s: %s", e.Status, e.Code, e.Description)
}

// ErrReapprovalNeeded reports a refresh token that no longer works, as when
// Google expires an app's authorization seven days after consent while it is
// in Testing status: the person must sign in again.
var ErrReapprovalNeeded = errors.New("google sign-in: the authorization has expired or been revoked; sign in again")

// Exchange exchanges an authorization's code for tokens.
func (c Client) Exchange(ctx context.Context, h *http.Client, a Authorization, code string) (Token, error) {
	return c.token(ctx, h, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {a.redirectURI},
		"code_verifier": {a.verifier},
	}, "")
}

// Refresh gets a new access token with a refresh token. A refresh token
// Google no longer accepts is reported as ErrReapprovalNeeded.
func (c Client) Refresh(ctx context.Context, h *http.Client, refreshToken string) (Token, error) {
	t, err := c.token(ctx, h, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refreshToken}}, refreshToken)
	var tokenErr *Error
	if errors.As(err, &tokenErr) && tokenErr.Code == "invalid_grant" {
		return Token{}, fmt.Errorf("%w: %v", ErrReapprovalNeeded, err)
	}
	return t, err
}

func (c Client) token(ctx context.Context, h *http.Client, form url.Values, refreshToken string) (Token, error) {
	form.Set("client_id", c.ID)
	if c.Secret != "" {
		form.Set("client_secret", c.Secret)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.TokenURI, strings.NewReader(form.Encode()))
	if err != nil {
		return Token{}, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	now := time.Now()
	response, err := h.Do(request)
	if err != nil {
		return Token{}, err
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		return Token{}, err
	}
	if response.StatusCode != http.StatusOK {
		var body struct {
			Error       string `json:"error"`
			Description string `json:"error_description"`
		}
		_ = json.Unmarshal(raw, &body)
		return Token{}, &Error{Status: response.StatusCode, Code: body.Error, Description: body.Description}
	}
	var body struct {
		AccessToken  string `json:"access_token"`
		ExpiresIn    int    `json:"expires_in"`
		RefreshToken string `json:"refresh_token"`
		IDToken      string `json:"id_token"`
		Scope        string `json:"scope"`
		TokenType    string `json:"token_type"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return Token{}, fmt.Errorf("google token endpoint: %w", err)
	}
	if body.AccessToken == "" || !strings.EqualFold(body.TokenType, "Bearer") {
		return Token{}, fmt.Errorf("google token endpoint: no bearer access token in the response")
	}
	t := Token{AccessToken: body.AccessToken, RefreshToken: body.RefreshToken, IDToken: body.IDToken, Scope: body.Scope, Expiry: now.Add(time.Duration(body.ExpiresIn) * time.Second)}
	if t.RefreshToken == "" {
		t.RefreshToken = refreshToken
	}
	return t, nil
}

// randomToken returns n random bytes, URL-safe base64 encoded: 48 bytes make
// a 64-character PKCE verifier, within its 43 to 128 characters.
func randomToken(n int) string {
	raw := make([]byte, n)
	_, _ = rand.Read(raw)
	return base64.RawURLEncoding.EncodeToString(raw)
}
