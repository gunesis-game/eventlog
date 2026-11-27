package googletest

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	logID   = "log1"
	board   = "eventlog/" + logID + "/devices"
	members = "eventlog/" + logID + "/members"
)

// A sign-in with the openid scope also returns a Google ID token naming the
// account, for the OAuth client; without it there is none.
func TestGoogleIDTokenWithOpenIDScope(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	e.AddTestUser(owner)
	claims := claimsOf(t, googleIDToken(t, e, owner))
	clientID, _ := e.Client()
	if claims["email"] != owner || claims["aud"] != clientID || claims["iss"] != "https://accounts.google.com" || claims["sub"] == "" {
		t.Fatalf("claims = %v", claims)
	}
	e.SignIn(owner)
	redirect := authorize(t, e, driveScope, "")
	if _, body := tokenRequest(t, e, url.Values{"grant_type": {"authorization_code"}, "code": {redirect.Get("code")}, "redirect_uri": {loopback}}); body["id_token"] != nil {
		t.Fatalf("an ID token without the openid scope: %v", body)
	}
}

// D-066: a Google ID token is exchanged for a Firebase token in the owner's
// project, which records the account on its first exchange with a stable
// user ID; a wrong API key, a tampered token, or an expired one is refused.
func TestFirebaseExchangesGoogleSignIn(t *testing.T) {
	clock := newTestClock()
	e := startEmulator(t, t.TempDir(), Options{Now: clock.Now})
	e.AddTestUser(editor)
	_, apiKey := e.Firebase()
	idToken := googleIDToken(t, e, editor)
	status, body := exchange(t, e, apiKey, idToken)
	expectStatus(t, "exchange", status, http.StatusOK, body)
	if body["providerId"] != "google.com" || body["email"] != editor || body["expiresIn"] != "3600" || body["refreshToken"] == nil || body["localId"] == "" {
		t.Fatalf("exchange = %v", body)
	}
	claims := claimsOf(t, body["idToken"].(string))
	if claims["user_id"] != body["localId"] || claims["email"] != editor {
		t.Fatalf("Firebase claims = %v", claims)
	}
	if again := firebaseSignIn(t, e, editor); again.uid != body["localId"] {
		t.Fatal("the user ID changed on a second exchange")
	}

	if status, body := exchange(t, e, "wrong-key", idToken); status != http.StatusBadRequest || !strings.Contains(errorMessage(body), "API key not valid") {
		t.Fatalf("wrong key = %d %v", status, body)
	}
	if status, body := exchange(t, e, apiKey, idToken[:len(idToken)-2]+"xx"); status != http.StatusBadRequest || !strings.HasPrefix(errorMessage(body), "INVALID_IDP_RESPONSE") {
		t.Fatalf("tampered token = %d %v", status, body)
	}
	clock.Advance(time.Hour)
	if status, body := exchange(t, e, apiKey, idToken); status != http.StatusBadRequest || !strings.HasPrefix(errorMessage(body), "INVALID_IDP_RESPONSE") {
		t.Fatalf("expired token = %d %v", status, body)
	}
}

func errorMessage(body map[string]any) string {
	e, _ := body["error"].(map[string]any)
	message, _ := e["message"].(string)
	return message
}

// A Firebase ID token lasts an hour, after which the database refuses it;
// the refresh token, which does not expire, gets a new one.
func TestFirebaseTokenRefresh(t *testing.T) {
	clock := newTestClock()
	e := startEmulator(t, t.TempDir(), Options{Now: clock.Now})
	e.AddTestUser(owner)
	e.InstallRules(owner)
	user := firebaseSignIn(t, e, owner)
	db := newDatabase(t, e, user)
	clock.Advance(time.Hour)
	if status, body := db.call("GET", members, nil); status != http.StatusUnauthorized || !jsonEqual(body, map[string]any{"error": "Auth token is expired"}) {
		t.Fatalf("expired token = %d %v", status, body)
	}
	clock.Advance(30 * 24 * time.Hour)
	_, apiKey := e.Firebase()
	refresh := func(form url.Values) (int, map[string]any) {
		response, err := http.PostForm(e.URL()+"/v1/token?key="+apiKey, form)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		var body map[string]any
		_ = json.NewDecoder(response.Body).Decode(&body)
		return response.StatusCode, body
	}
	status, body := refresh(url.Values{"grant_type": {"refresh_token"}, "refresh_token": {user.refresh}})
	if status != http.StatusOK || body["user_id"] != user.uid || body["token_type"] != "Bearer" || body["expires_in"] != "3600" || body["refresh_token"] != user.refresh {
		t.Fatalf("refresh = %d %v", status, body)
	}
	db.token = body["id_token"].(string)
	if status, _ := db.call("GET", members, nil); status != http.StatusOK {
		t.Fatalf("with the refreshed token: %d", status)
	}
	for form, want := range map[string]string{
		"grant_type=refresh_token&refresh_token=unknown": "INVALID_REFRESH_TOKEN",
		"grant_type=refresh_token":                       "MISSING_REFRESH_TOKEN",
		"grant_type=password&refresh_token=x":            "INVALID_GRANT_TYPE",
	} {
		values, _ := url.ParseQuery(form)
		if status, body := refresh(values); status != http.StatusBadRequest || errorMessage(body) != want {
			t.Fatalf("%s = %d %v, want %s", form, status, body, want)
		}
	}
}

// boardUsers sets up the owner's database with the rules installed and a
// members list naming an editor as writer and a viewer as reader, and
// returns their database clients and a stranger's.
func boardUsers(t *testing.T, e *Emulator) (ownerDB, editorDB, viewerDB, strangerDB *database, editorUID string) {
	t.Helper()
	for _, account := range []string{owner, editor, viewer, "stranger@example.com"} {
		e.AddTestUser(account)
	}
	e.InstallRules(owner)
	ownerDB = newDatabase(t, e, firebaseSignIn(t, e, owner))
	editorUser := firebaseSignIn(t, e, editor)
	editorDB = newDatabase(t, e, editorUser)
	viewerDB = newDatabase(t, e, firebaseSignIn(t, e, viewer))
	strangerDB = newDatabase(t, e, firebaseSignIn(t, e, "stranger@example.com"))
	status, body := ownerDB.call("PUT", members, map[string]any{emailKey(owner): "writer", emailKey(editor): "writer", emailKey(viewer): "reader"})
	expectStatus(t, "write the members list", status, http.StatusOK, body)
	return ownerDB, editorDB, viewerDB, strangerDB, editorUser.uid
}

// Until the owner installs the rules, the database refuses every request,
// as a new database's locked mode does.
func TestDatabaseIsLockedUntilRulesAreInstalled(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	e.AddTestUser(owner)
	db := newDatabase(t, e, firebaseSignIn(t, e, owner))
	if status, body := db.call("PUT", members, map[string]any{"a": "writer"}); status != http.StatusUnauthorized || !jsonEqual(body, map[string]any{"error": "Permission denied"}) {
		t.Fatalf("before the rules = %d %v", status, body)
	}
	e.InstallRules(owner)
	if status, body := db.call("PUT", members, map[string]any{"a": "writer"}); status != http.StatusOK {
		t.Fatalf("after the rules = %d %v", status, body)
	}
}

// D-066: the security rules. Only the owner writes the members list; each
// account reads its own entry; writers write the board only under their own
// user ID; members read the board; a non-member may only write its own
// membership request, which the owner reads; nothing is open to an
// unauthenticated request.
func TestSecurityRules(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	ownerDB, editorDB, viewerDB, strangerDB, editorUID := boardUsers(t, e)
	anonymous := &database{t: t, url: ownerDB.url}
	entry := map[string]any{"path": "devices/d1/segments/segment-1", "size": 10}
	strangerKey := emailKey("stranger@example.com")
	for _, c := range []struct {
		name   string
		db     *database
		method string
		path   string
		body   any
		allow  bool
	}{
		{"editor writes its board entry", editorDB, "PUT", board + "/" + editorUID + "/d1", entry, true},
		{"editor writes under another user ID", editorDB, "PUT", board + "/someone-else/d1", entry, false},
		{"editor writes the whole board", editorDB, "PUT", board, map[string]any{}, false},
		{"viewer writes the board", viewerDB, "PUT", board + "/" + editorUID + "/d2", entry, false},
		{"viewer reads the board", viewerDB, "GET", board, nil, true},
		{"editor reads the board", editorDB, "GET", board, nil, true},
		{"owner reads the board", ownerDB, "GET", board, nil, true},
		{"stranger reads the board", strangerDB, "GET", board, nil, false},
		{"stranger writes its request", strangerDB, "PUT", "eventlog/" + logID + "/requests/" + strangerKey, true, true},
		{"stranger writes another request", strangerDB, "PUT", "eventlog/" + logID + "/requests/" + emailKey(viewer), true, false},
		{"stranger reads requests", strangerDB, "GET", "eventlog/" + logID + "/requests", nil, false},
		{"owner reads requests", ownerDB, "GET", "eventlog/" + logID + "/requests", nil, true},
		{"editor writes the members list", editorDB, "PATCH", members, map[string]any{strangerKey: "writer"}, false},
		{"viewer reads its own entry", viewerDB, "GET", members + "/" + emailKey(viewer), nil, true},
		{"viewer reads another entry", viewerDB, "GET", members + "/" + emailKey(editor), nil, false},
		{"viewer reads the members list", viewerDB, "GET", members, nil, false},
		{"anyone reads the root", ownerDB, "GET", "", nil, false},
		{"unauthenticated reads the board", anonymous, "GET", board, nil, false},
	} {
		status, body := c.db.call(c.method, c.path, c.body)
		if allowed := status == http.StatusOK; allowed != c.allow {
			t.Errorf("%s: %d %v, want allowed %v", c.name, status, body, c.allow)
		}
	}
	if role := e.Database(members + "/" + strangerKey); role != nil {
		t.Fatalf("a refused write changed the members list: %v", role)
	}
}

// REST writes: PUT returns what was written, PATCH merges children and
// accepts paths as keys, DELETE removes, empty objects vanish, and
// print=silent answers 204.
func TestDatabaseRESTWrites(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	ownerDB, _, _, _, _ := boardUsers(t, e)
	path := "eventlog/" + logID + "/members"
	if status, body := ownerDB.call("PATCH", path, map[string]any{"a": "writer", "b/c": "nested"}); status != http.StatusOK || !jsonEqual(body, map[string]any{"a": "writer", "b/c": "nested"}) {
		t.Fatalf("patch = %d %v", status, body)
	}
	got := e.Database(path).(map[string]any)
	if got["a"] != "writer" || !jsonEqual(got["b"], map[string]any{"c": "nested"}) || got[emailKey(owner)] != "writer" {
		t.Fatalf("after patch = %v", got)
	}
	if status, body := ownerDB.call("DELETE", path+"/b/c", nil); status != http.StatusOK || body != nil {
		t.Fatalf("delete = %d %v", status, body)
	}
	if e.Database(path+"/b") != nil {
		t.Fatal("an empty object was kept")
	}
	request, _ := http.NewRequest("PUT", ownerDB.address(path+"/a", url.Values{"print": {"silent"}}), strings.NewReader(`"reader"`))
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNoContent || e.Database(path+"/a") != "reader" {
		t.Fatalf("silent put = %d, value %v", response.StatusCode, e.Database(path+"/a"))
	}
	request, _ = http.NewRequest("PUT", ownerDB.address(path, nil), strings.NewReader("{broken"))
	response, _ = http.DefaultClient.Do(request)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("malformed JSON = %d", response.StatusCode)
	}
}

// D-063: a stream is redirected once, starts with the location's whole
// value, then sends a put for each changed value and a patch for each
// PATCH, with paths relative to the location; writes that change nothing,
// or elsewhere, send nothing; a write above the location sends its new
// value; keep-alive comes while idle.
func TestStreamEvents(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{KeepAlive: 30 * time.Millisecond})
	ownerDB, editorDB, viewerDB, _, editorUID := boardUsers(t, e)
	entry := board + "/" + editorUID + "/d1"
	editorDB.call("PUT", entry, map[string]any{"path": "a", "size": 1})

	request, _ := http.NewRequest("GET", viewerDB.address(board, nil), nil)
	request.Header.Set("Accept", "text/event-stream")
	response, err := noRedirects.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusTemporaryRedirect || !strings.Contains(response.Header.Get("Location"), "ns=") {
		t.Fatalf("first stream request = %d %q", response.StatusCode, response.Header.Get("Location"))
	}

	_, s := viewerDB.stream(board)
	s.expectEvent("put", "/", map[string]any{editorUID: map[string]any{"d1": map[string]any{"path": "a", "size": 1}}})
	editorDB.call("PUT", entry+"/size", 2)
	s.expectEvent("put", "/"+editorUID+"/d1/size", 2)
	editorDB.call("PATCH", entry, map[string]any{"path": "b", "size": 3})
	s.expectEvent("patch", "/"+editorUID+"/d1", map[string]any{"path": "b", "size": 3})
	editorDB.call("PUT", entry+"/size", 3)
	ownerDB.call("PUT", "eventlog/other-log/members", map[string]any{"x": "writer"})
	s.expectQuiet()
	editorDB.call("DELETE", entry, nil)
	s.expectEvent("put", "/"+editorUID+"/d1", nil)

	select {
	case ev := <-s.events:
		if ev.name != "keep-alive" || ev.data != nil {
			t.Fatalf("idle event = %v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("no keep-alive while idle")
	}

	_, whole := ownerDB.stream(members + "/" + emailKey(viewer))
	whole.expectEvent("put", "/", "reader")
	ownerDB.call("PUT", members, map[string]any{emailKey(owner): "writer", emailKey(viewer): "writer"})
	whole.expectEvent("put", "/", "writer")
}

// When the rules stop allowing a stream's read, as when the owner removes
// the account from the members list, the stream sends cancel and ends.
func TestStreamCancelledWhenAccessIsRemoved(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	ownerDB, _, viewerDB, _, _ := boardUsers(t, e)
	_, s := viewerDB.stream(board)
	s.next()
	ownerDB.call("DELETE", members+"/"+emailKey(viewer), nil)
	s.expectEnd("cancel")
	if status, _ := viewerDB.stream(board); status != http.StatusUnauthorized {
		t.Fatalf("reopening = %d, want refused", status)
	}
}

// When a stream's sign-in expires, it sends auth_revoked and ends; the
// device reconnects with a refreshed token.
func TestStreamAuthRevokedWhenTokenExpires(t *testing.T) {
	clock := newTestClock()
	e := startEmulator(t, t.TempDir(), Options{Now: clock.Now, KeepAlive: 20 * time.Millisecond})
	_, _, viewerDB, _, _ := boardUsers(t, e)
	_, s := viewerDB.stream(board)
	s.next()
	clock.Advance(time.Hour)
	ev := s.next()
	if ev.name != "auth_revoked" || ev.data != "credential is no longer valid" {
		t.Fatalf("after expiry = %v", ev)
	}
	s.expectEnd("")
}

// A dropped connection ends the stream without an event; reconnecting
// starts again with the whole current value, changes made meanwhile
// included.
func TestDroppedStreamsReconnect(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	_, editorDB, viewerDB, _, editorUID := boardUsers(t, e)
	_, s := viewerDB.stream(board)
	s.next()
	e.DropStreams()
	s.expectEnd("")
	editorDB.call("PUT", board+"/"+editorUID+"/d1", map[string]any{"size": 5})
	_, again := viewerDB.stream(board)
	again.expectEvent("put", "/", map[string]any{editorUID: map[string]any{"d1": map[string]any{"size": 5}}})
}

// The free plan allows 100 connections at once; beyond that a connection is
// refused until one closes.
func TestConnectionLimit(t *testing.T) {
	if e := startEmulator(t, t.TempDir(), Options{}); e.options.MaxConnections != 100 {
		t.Fatalf("default limit = %d, want the free plan's 100", e.options.MaxConnections)
	}
	e := startEmulator(t, t.TempDir(), Options{MaxConnections: 3})
	ownerDB, _, viewerDB, _, _ := boardUsers(t, e)
	var streams []*eventStream
	for i := 0; i < 3; i++ {
		_, s := viewerDB.stream(board)
		s.next()
		streams = append(streams, s)
	}
	if status, body := ownerDB.call("GET", board, nil); status != http.StatusServiceUnavailable || body == nil {
		t.Fatalf("a request over the limit = %d %v", status, body)
	}
	streams[0].close()
	deadline := time.Now().Add(5 * time.Second)
	for e.OpenStreams() > 2 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if status, _ := ownerDB.call("GET", board, nil); status != http.StatusOK {
		t.Fatalf("after a stream closed = %d", status)
	}
}

// When the database is switched off, as for exceeding the free plan,
// streams are dropped and requests refused until it is switched on.
func TestDatabaseSwitchedOff(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	ownerDB, _, viewerDB, _, _ := boardUsers(t, e)
	_, s := viewerDB.stream(board)
	s.next()
	e.SwitchOffDatabase()
	s.expectEnd("")
	if status, _ := ownerDB.call("GET", board, nil); status != http.StatusServiceUnavailable {
		t.Fatalf("while off = %d", status)
	}
	if status, _ := viewerDB.stream(board); status != http.StatusServiceUnavailable {
		t.Fatalf("stream while off = %d", status)
	}
	e.SwitchOnDatabase()
	if status, _ := ownerDB.call("GET", board, nil); status != http.StatusOK {
		t.Fatalf("after switching on = %d", status)
	}
}

// Board writes, connections, and streamed bytes are counted per account,
// a stream's bytes while it is still open.
func TestDatabaseCallsAreCounted(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	_, editorDB, viewerDB, _, editorUID := boardUsers(t, e)
	e.ResetStats()
	_, s := viewerDB.stream(board)
	s.next()
	editorDB.call("PUT", board+"/"+editorUID+"/d1", map[string]any{"size": 1})
	s.next()
	stats := e.Stats()
	if writes := stats[editor]["rtdb.put"]; writes.Requests != 1 || writes.RequestBytes == 0 {
		t.Fatalf("editor writes = %+v", writes)
	}
	streamed := stats[viewer]["rtdb.stream"]
	if streamed.Requests != 1 || streamed.ResponseBytes == 0 {
		t.Fatalf("viewer stream while open = %+v", streamed)
	}
}

// The database is kept on the real file system as a JSON file, with the
// rules and Firebase sign-ins in the metadata, so a restart keeps them.
func TestDatabaseSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	e := startEmulator(t, dir, Options{})
	_, editorDB, viewerDB, _, editorUID := boardUsers(t, e)
	editorDB.call("PUT", board+"/"+editorUID+"/d1", map[string]any{"size": 7})
	raw, err := os.ReadFile(filepath.Join(dir, databaseFileName))
	if err != nil || !strings.Contains(string(raw), `"size":7`) {
		t.Fatalf("database on disk = %s, %v", raw, err)
	}
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
	restarted := startEmulator(t, dir, Options{})
	viewerDB.url, _ = restarted.Firebase()
	if status, body := viewerDB.call("GET", board+"/"+editorUID+"/d1/size", nil); status != http.StatusOK || body != float64(7) {
		t.Fatalf("after restart = %d %v", status, body)
	}
}
