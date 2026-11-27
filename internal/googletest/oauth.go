package googletest

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// authCode is an authorization code issued by the browser step, exchanged
// once for tokens.
type authCode struct {
	Account     string    `json:"account"`
	Scope       string    `json:"scope"`
	RedirectURI string    `json:"redirect_uri"`
	Challenge   string    `json:"challenge"`
	Offline     bool      `json:"offline"`
	Issued      time.Time `json:"issued"`
}

// refreshToken is a long-lived token, valid until seven days after consent
// while the app is in Testing.
type refreshToken struct {
	Account string    `json:"account"`
	Scope   string    `json:"scope"`
	Consent time.Time `json:"consent"`
}

type accessToken struct {
	Account string    `json:"account"`
	Scope   string    `json:"scope"`
	Expires time.Time `json:"expires"`
}

// authorize is the browser step: the account set with SignIn consents, and
// the browser is redirected to the loopback address with a code, or with
// access_denied for an account that is not a test user. A request Google
// would refuse outright gets an error page instead of a redirect.
func (e *Emulator) authorize(w http.ResponseWriter, r *http.Request) {
	c := named(r, "oauth.authorize")
	q := r.URL.Query()
	e.mu.Lock()
	defer e.mu.Unlock()
	if q.Get("client_id") != e.state.Client.ID {
		http.Error(w, "Error 401: invalid_client", http.StatusUnauthorized)
		return
	}
	redirect, err := url.Parse(q.Get("redirect_uri"))
	if err != nil || redirect.Scheme != "http" || (redirect.Hostname() != "127.0.0.1" && redirect.Hostname() != "localhost") {
		http.Error(w, "Error 400: invalid_request: redirect_uri must be a loopback address", http.StatusBadRequest)
		return
	}
	if q.Get("response_type") != "code" || q.Get("scope") == "" {
		http.Error(w, "Error 400: invalid_request", http.StatusBadRequest)
		return
	}
	if q.Get("code_challenge") != "" && q.Get("code_challenge_method") != "S256" {
		http.Error(w, "Error 400: invalid_request: code_challenge_method", http.StatusBadRequest)
		return
	}
	who := e.state.Accounts[e.signingIn]
	if who == nil {
		http.Error(w, "no account is signing in; call SignIn", http.StatusBadRequest)
		return
	}
	c.account = e.signingIn
	values := url.Values{}
	if state := q.Get("state"); state != "" {
		values.Set("state", state)
	}
	if !who.TestUser {
		values.Set("error", "access_denied")
	} else {
		code := "4/" + newID()
		e.state.Codes[code] = &authCode{Account: e.signingIn, Scope: q.Get("scope"), RedirectURI: q.Get("redirect_uri"), Challenge: q.Get("code_challenge"), Offline: q.Get("access_type") == "offline", Issued: e.options.Now()}
		e.saveLocked()
		values.Set("code", code)
		values.Set("scope", q.Get("scope"))
	}
	redirect.RawQuery = values.Encode()
	http.Redirect(w, r, redirect.String(), http.StatusFound)
}

// token exchanges an authorization code, or refreshes an access token.
func (e *Emulator) token(w http.ResponseWriter, r *http.Request) {
	c := named(r, "oauth.token")
	if err := r.ParseForm(); err != nil {
		oauthError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if r.PostForm.Get("client_id") != e.state.Client.ID || r.PostForm.Get("client_secret") != e.state.Client.Secret {
		oauthError(w, http.StatusUnauthorized, "invalid_client", "Unauthorized")
		return
	}
	now := e.options.Now()
	switch r.PostForm.Get("grant_type") {
	case "authorization_code":
		code := e.state.Codes[r.PostForm.Get("code")]
		if code == nil || now.Sub(code.Issued) > codeLife {
			oauthError(w, http.StatusBadRequest, "invalid_grant", "Malformed auth code.")
			return
		}
		delete(e.state.Codes, r.PostForm.Get("code"))
		c.account = code.Account
		if r.PostForm.Get("redirect_uri") != code.RedirectURI {
			e.saveLocked()
			oauthError(w, http.StatusBadRequest, "redirect_uri_mismatch", "Bad Request")
			return
		}
		if code.Challenge != "" && challengeOf(r.PostForm.Get("code_verifier")) != code.Challenge {
			e.saveLocked()
			oauthError(w, http.StatusBadRequest, "invalid_grant", "Invalid code verifier.")
			return
		}
		response := e.issueAccessLocked(code.Account, code.Scope, now)
		if code.Offline {
			refresh := "1//" + newID()
			e.state.Refresh[refresh] = &refreshToken{Account: code.Account, Scope: code.Scope, Consent: now}
			response["refresh_token"] = refresh
		}
		e.saveLocked()
		writeJSON(w, http.StatusOK, response)
	case "refresh_token":
		refresh := e.state.Refresh[r.PostForm.Get("refresh_token")]
		if refresh == nil || now.Sub(refresh.Consent) >= refreshTokenLife {
			oauthError(w, http.StatusBadRequest, "invalid_grant", "Token has been expired or revoked.")
			return
		}
		c.account = refresh.Account
		response := e.issueAccessLocked(refresh.Account, refresh.Scope, now)
		e.saveLocked()
		writeJSON(w, http.StatusOK, response)
	default:
		oauthError(w, http.StatusBadRequest, "unsupported_grant_type", "Invalid grant_type: "+r.PostForm.Get("grant_type"))
	}
}

func (e *Emulator) issueAccessLocked(account, scope string, now time.Time) map[string]any {
	token := "ya29." + newID() + newID()
	e.state.Access[token] = &accessToken{Account: account, Scope: scope, Expires: now.Add(accessTokenLife)}
	return map[string]any{"access_token": token, "expires_in": int(accessTokenLife.Seconds()) - 1, "scope": scope, "token_type": "Bearer"}
}

func challengeOf(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func oauthError(w http.ResponseWriter, status int, code, description string) {
	writeJSON(w, status, map[string]string{"error": code, "error_description": description})
}

// authorized checks the bearer token of a Drive call and passes on the
// calling account: an unknown or expired token is refused with 401, and a
// token without the full drive scope with 403.
func (e *Emulator) authorized(handle func(w http.ResponseWriter, r *http.Request, c *call)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := named(r, "drive")
		bearer, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		e.mu.Lock()
		token := e.state.Access[bearer]
		valid := ok && token != nil && e.options.Now().Before(token.Expires)
		e.mu.Unlock()
		if !valid {
			driveError(w, http.StatusUnauthorized, "authError", "Request had invalid authentication credentials. Expected OAuth 2 access token, login cookie or other valid authentication credential.")
			return
		}
		c.account = token.Account
		if !hasScope(token.Scope, driveScope) {
			driveError(w, http.StatusForbidden, "insufficientPermissions", "Request had insufficient authentication scopes.")
			return
		}
		handle(w, r, c)
	}
}

func hasScope(scopes, want string) bool {
	for _, scope := range strings.Fields(scopes) {
		if scope == want {
			return true
		}
	}
	return false
}
