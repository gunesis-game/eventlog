package googletest

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Firebase returns the owner's Firebase project as the settings file names
// it: the database URL and the web API key.
func (e *Emulator) Firebase() (databaseURL, apiKey string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.URL() + databasePrefix, e.state.Firebase.APIKey
}

// idClaims are the claims of a Google or Firebase ID token.
type idClaims struct {
	Issuer   string `json:"iss"`
	Audience string `json:"aud"`
	Subject  string `json:"sub"`
	UserID   string `json:"user_id,omitempty"`
	Email    string `json:"email"`
	Verified bool   `json:"email_verified"`
	Issued   int64  `json:"iat"`
	Expires  int64  `json:"exp"`
}

// signLocked encodes claims as a JWT signed by the emulator. Real Google
// tokens are signed with RS256 by Google's keys; eventlog only passes them on,
// so the emulator signs with its own key, which only it checks.
func (e *Emulator) signLocked(claims idClaims) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload, _ := json.Marshal(claims)
	unsigned := header + "." + base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(e.state.SigningKey))
	mac.Write([]byte(unsigned))
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

var (
	errTokenParse   = errors.New("Could not parse auth token.")
	errTokenExpired = errors.New("Auth token is expired")
)

// verifyLocked checks a token's signature and expiry and returns its claims.
func (e *Emulator) verifyLocked(token string) (idClaims, error) {
	var claims idClaims
	header, rest, ok := strings.Cut(token, ".")
	payload, signature, ok2 := strings.Cut(rest, ".")
	if !ok || !ok2 {
		return claims, errTokenParse
	}
	mac := hmac.New(sha256.New, []byte(e.state.SigningKey))
	mac.Write([]byte(header + "." + payload))
	want := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if !hmac.Equal([]byte(signature), []byte(want)) || err != nil || json.Unmarshal(raw, &claims) != nil {
		return claims, errTokenParse
	}
	if e.options.Now().Unix() >= claims.Expires {
		return claims, errTokenExpired
	}
	return claims, nil
}

// googleIDTokenLocked issues a Google ID token for an account, for the
// OAuth client, as a sign-in with the openid scope returns.
func (e *Emulator) googleIDTokenLocked(key string, now time.Time) string {
	a := e.state.Accounts[key]
	return e.signLocked(idClaims{Issuer: "https://accounts.google.com", Audience: e.state.Client.ID, Subject: a.Subject, Email: a.Email, Verified: true, Issued: now.Unix(), Expires: now.Add(idTokenLife).Unix()})
}

// firebaseIDTokenLocked issues a Firebase ID token for an account in the
// owner's project.
func (e *Emulator) firebaseIDTokenLocked(key string, now time.Time) string {
	a := e.state.Accounts[key]
	project := e.state.Firebase.ProjectID
	return e.signLocked(idClaims{Issuer: "https://securetoken.google.com/" + project, Audience: project, Subject: a.UID, UserID: a.UID, Email: strings.ToLower(a.Email), Verified: true, Issued: now.Unix(), Expires: now.Add(idTokenLife).Unix()})
}

// googleAPIError writes an error in the form of Google's Identity Toolkit
// and Secure Token APIs, with the error code as the message.
func googleAPIError(w http.ResponseWriter, code int, message string) {
	status := map[int]string{400: "INVALID_ARGUMENT", 403: "PERMISSION_DENIED"}[code]
	writeJSON(w, code, map[string]any{"error": map[string]any{
		"code": code, "message": message, "status": status,
		"errors": []any{map[string]any{"message": message, "domain": "global", "reason": "invalid"}},
	}})
}

// checkAPIKey refuses a request without the project's web API key.
func (e *Emulator) checkAPIKey(w http.ResponseWriter, r *http.Request) bool {
	e.mu.Lock()
	valid := r.URL.Query().Get("key") == e.state.Firebase.APIKey
	e.mu.Unlock()
	if !valid {
		googleAPIError(w, http.StatusBadRequest, "API key not valid. Please pass a valid API key.")
	}
	return valid
}

// signInWithIdp exchanges a Google ID token for a Firebase ID token and
// refresh token in the owner's project. The account is recorded in the
// project on its first exchange; nobody signs up.
func (e *Emulator) signInWithIdp(w http.ResponseWriter, r *http.Request) {
	c := named(r, "firebase.signInWithIdp")
	if !e.checkAPIKey(w, r) {
		return
	}
	var request struct {
		RequestURI        string `json:"requestUri"`
		PostBody          string `json:"postBody"`
		ReturnSecureToken bool   `json:"returnSecureToken"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.RequestURI == "" {
		googleAPIError(w, http.StatusBadRequest, "INVALID_REQUEST_URI")
		return
	}
	credential, err := url.ParseQuery(request.PostBody)
	if err != nil || credential.Get("providerId") != "google.com" {
		googleAPIError(w, http.StatusBadRequest, "INVALID_IDP_RESPONSE : the provider is not google.com")
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	claims, err := e.verifyLocked(credential.Get("id_token"))
	if err != nil || claims.Issuer != "https://accounts.google.com" || claims.Audience != e.state.Client.ID {
		googleAPIError(w, http.StatusBadRequest, "INVALID_IDP_RESPONSE : Invalid Idp Response: the Google id_token is not allowed to be used with this application.")
		return
	}
	key := strings.ToLower(claims.Email)
	a := e.state.Accounts[key]
	c.account = key
	now := e.options.Now()
	response := map[string]any{
		"kind":          "identitytoolkit#VerifyAssertionResponse",
		"federatedId":   "https://accounts.google.com/" + a.Subject,
		"providerId":    "google.com",
		"localId":       a.UID,
		"email":         a.Email,
		"emailVerified": true,
		"idToken":       e.firebaseIDTokenLocked(key, now),
		"expiresIn":     strconv.Itoa(int(idTokenLife.Seconds())),
	}
	if request.ReturnSecureToken {
		refresh := "AMf-" + newID() + newID()
		e.state.Firebase.Refresh[refresh] = key
		response["refreshToken"] = refresh
		e.saveLocked()
	}
	writeJSON(w, http.StatusOK, response)
}

// firebaseToken exchanges a Firebase refresh token for a new ID token.
// Firebase refresh tokens do not expire.
func (e *Emulator) firebaseToken(w http.ResponseWriter, r *http.Request) {
	c := named(r, "firebase.token")
	if !e.checkAPIKey(w, r) {
		return
	}
	if err := r.ParseForm(); err != nil {
		googleAPIError(w, http.StatusBadRequest, "INVALID_GRANT_TYPE")
		return
	}
	switch {
	case r.PostForm.Get("grant_type") != "refresh_token":
		googleAPIError(w, http.StatusBadRequest, "INVALID_GRANT_TYPE")
		return
	case r.PostForm.Get("refresh_token") == "":
		googleAPIError(w, http.StatusBadRequest, "MISSING_REFRESH_TOKEN")
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	key, ok := e.state.Firebase.Refresh[r.PostForm.Get("refresh_token")]
	if !ok {
		googleAPIError(w, http.StatusBadRequest, "INVALID_REFRESH_TOKEN")
		return
	}
	c.account = key
	writeJSON(w, http.StatusOK, map[string]any{
		"expires_in":    strconv.Itoa(int(idTokenLife.Seconds())),
		"token_type":    "Bearer",
		"refresh_token": r.PostForm.Get("refresh_token"),
		"id_token":      e.firebaseIDTokenLocked(key, e.options.Now()),
		"user_id":       e.state.Accounts[key].UID,
		"project_id":    e.state.Firebase.ProjectID,
	})
}
