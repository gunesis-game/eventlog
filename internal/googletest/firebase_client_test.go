package googletest

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

const openIDScope = "openid email " + driveScope

// googleIDToken signs an account in with the openid scope and returns the
// Google ID token from the code exchange.
func googleIDToken(t *testing.T, e *Emulator, email string) string {
	t.Helper()
	e.SignIn(email)
	redirect := authorize(t, e, openIDScope, challengeOf(verifier))
	status, body := tokenRequest(t, e, url.Values{"grant_type": {"authorization_code"}, "code": {redirect.Get("code")}, "redirect_uri": {loopback}, "code_verifier": {verifier}})
	if status != http.StatusOK || body["id_token"] == nil {
		t.Fatalf("code exchange: %d %v", status, body)
	}
	return body["id_token"].(string)
}

// postJSON posts a JSON body and decodes the JSON response.
func postJSON(t *testing.T, address string, body any) (int, map[string]any) {
	t.Helper()
	raw, _ := json.Marshal(body)
	response, err := http.Post(address, "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var decoded map[string]any
	_ = json.NewDecoder(response.Body).Decode(&decoded)
	return response.StatusCode, decoded
}

// exchange calls signInWithIdp with a Google ID token.
func exchange(t *testing.T, e *Emulator, apiKey, idToken string) (int, map[string]any) {
	t.Helper()
	return postJSON(t, e.URL()+"/v1/accounts:signInWithIdp?key="+apiKey, map[string]any{
		"requestUri": "http://localhost", "postBody": "id_token=" + idToken + "&providerId=google.com", "returnSecureToken": true,
	})
}

// firebaseUser is an account signed in to the owner's Firebase project.
type firebaseUser struct {
	uid     string
	idToken string
	refresh string
}

// firebaseSignIn signs an account in to Google and exchanges its ID token
// for a Firebase one.
func firebaseSignIn(t *testing.T, e *Emulator, email string) firebaseUser {
	t.Helper()
	_, apiKey := e.Firebase()
	status, body := exchange(t, e, apiKey, googleIDToken(t, e, email))
	if status != http.StatusOK {
		t.Fatalf("signInWithIdp for %s: %d %v", email, status, body)
	}
	return firebaseUser{uid: body["localId"].(string), idToken: body["idToken"].(string), refresh: body["refreshToken"].(string)}
}

// claimsOf decodes a token's claims without checking it.
func claimsOf(t *testing.T, token string) map[string]any {
	t.Helper()
	parts := strings.Split(token, ".")
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatal(err)
	}
	var claims map[string]any
	_ = json.Unmarshal(raw, &claims)
	return claims
}

// database makes Realtime Database REST calls as one Firebase user, or
// unauthenticated when token is "".
type database struct {
	t     *testing.T
	url   string
	token string
}

func newDatabase(t *testing.T, e *Emulator, user firebaseUser) *database {
	databaseURL, _ := e.Firebase()
	return &database{t: t, url: databaseURL, token: user.idToken}
}

func (d *database) address(path string, extra url.Values) string {
	query := url.Values{}
	if d.token != "" {
		query.Set("auth", d.token)
	}
	for key, values := range extra {
		query[key] = values
	}
	return d.url + "/" + strings.TrimPrefix(path, "/") + ".json?" + query.Encode()
}

// call sends a request with a JSON body (nil for none) and decodes the
// JSON response.
func (d *database) call(method, path string, body any) (int, any) {
	d.t.Helper()
	var reader io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	}
	request, _ := http.NewRequest(method, d.address(path, nil), reader)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		d.t.Fatal(err)
	}
	defer response.Body.Close()
	var decoded any
	_ = json.NewDecoder(response.Body).Decode(&decoded)
	return response.StatusCode, decoded
}

// event is one received server-sent event.
type event struct {
	name string
	data any
}

// eventStream is an open stream of a database location.
type eventStream struct {
	t        *testing.T
	response *http.Response
	events   chan event
}

// stream opens a stream of a location, following the redirect to the
// server holding the data, and returns its status and, when it opened, the
// stream.
func (d *database) stream(path string) (int, *eventStream) {
	d.t.Helper()
	request, _ := http.NewRequest("GET", d.address(path, nil), nil)
	request.Header.Set("Accept", "text/event-stream")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		d.t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		_ = response.Body.Close()
		return response.StatusCode, nil
	}
	s := &eventStream{t: d.t, response: response, events: make(chan event, 64)}
	d.t.Cleanup(s.close)
	go func() {
		defer close(s.events)
		reader := bufio.NewReader(response.Body)
		var name string
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimRight(line, "\n")
			switch {
			case strings.HasPrefix(line, "event: "):
				name = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				var data any
				_ = json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &data)
				s.events <- event{name: name, data: data}
			}
		}
	}()
	return http.StatusOK, s
}

func (s *eventStream) close() {
	_ = s.response.Body.Close()
}

// next returns the next event other than keep-alive.
func (s *eventStream) next() event {
	s.t.Helper()
	for {
		select {
		case ev, ok := <-s.events:
			if !ok {
				s.t.Fatal("the stream ended")
			}
			if ev.name != "keep-alive" {
				return ev
			}
		case <-time.After(5 * time.Second):
			s.t.Fatal("timed out waiting for an event")
		}
	}
}

// expectEvent waits for the next event and checks its name, path, and data.
func (s *eventStream) expectEvent(name, path string, data any) {
	s.t.Helper()
	ev := s.next()
	payload, _ := ev.data.(map[string]any)
	if ev.name != name || payload["path"] != path || !jsonEqual(payload["data"], data) {
		s.t.Fatalf("event %s %v, want %s {path: %s, data: %v}", ev.name, ev.data, name, path, data)
	}
}

// expectEnd waits for the stream to end, after an event named name when
// name is not "".
func (s *eventStream) expectEnd(name string) {
	s.t.Helper()
	if name != "" {
		if ev := s.next(); ev.name != name {
			s.t.Fatalf("event %s %v, want %s", ev.name, ev.data, name)
		}
	}
	for {
		select {
		case ev, ok := <-s.events:
			if !ok {
				return
			}
			if ev.name != "keep-alive" {
				s.t.Fatalf("event %s after the stream should have ended", ev.name)
			}
		case <-time.After(5 * time.Second):
			s.t.Fatal("the stream did not end")
		}
	}
}

// expectQuiet checks no event other than keep-alive arrives for a while.
func (s *eventStream) expectQuiet() {
	s.t.Helper()
	deadline := time.After(100 * time.Millisecond)
	for {
		select {
		case ev, ok := <-s.events:
			if ok && ev.name == "keep-alive" {
				continue
			}
			s.t.Fatalf("unexpected event %v", ev)
		case <-deadline:
			return
		}
	}
}

func jsonEqual(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return bytes.Equal(x, y)
}
