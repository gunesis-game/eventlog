package googletest

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gunesis-game/eventlog/internal/firebase"
	"github.com/gunesis-game/eventlog/internal/googleauth"
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
	claims := claimsOf(t, signIn(t, e, owner, allScopes...).IDToken)
	clientID, _ := e.Client()
	if claims["email"] != owner || claims["aud"] != clientID || claims["iss"] != "https://accounts.google.com" || claims["sub"] == "" {
		t.Fatalf("claims = %v", claims)
	}
	if token := signIn(t, e, owner, googleauth.ScopeDrive); token.IDToken != "" {
		t.Fatal("an ID token without the openid scope")
	}
}

func firebaseCode(err error) string {
	var authErr *firebase.AuthError
	if errors.As(err, &authErr) {
		return authErr.Code()
	}
	return ""
}

// D-066: a Google ID token is exchanged for a Firebase session in the
// owner's project, which records the account on its first exchange with a
// stable user ID; a wrong API key, a tampered token, or an expired one is
// refused.
func TestFirebaseExchangesGoogleSignIn(t *testing.T) {
	clock := newTestClock()
	e := startEmulator(t, t.TempDir(), Options{Now: clock.Now})
	e.AddTestUser(editor)
	_, apiKey := e.Firebase()
	auth := firebase.NewAuth(http.DefaultClient, apiKey, e.URL(), e.URL())
	idToken := signIn(t, e, editor, allScopes...).IDToken
	session, err := auth.SignInWithGoogle(ctx, idToken)
	if err != nil {
		t.Fatal(err)
	}
	if session.UID == "" || session.Email != editor || session.RefreshToken == "" || time.Until(session.Expiry) < 59*time.Minute {
		t.Fatalf("session = %+v", session)
	}
	if claims := claimsOf(t, session.IDToken); claims["user_id"] != session.UID || claims["email"] != editor {
		t.Fatalf("Firebase claims = %v", claims)
	}
	if again, err := auth.SignInWithGoogle(ctx, idToken); err != nil || again.UID != session.UID {
		t.Fatalf("second exchange = %+v, %v", again, err)
	}
	wrongKey := firebase.NewAuth(http.DefaultClient, "wrong-key", e.URL(), e.URL())
	if _, err := wrongKey.SignInWithGoogle(ctx, idToken); !strings.Contains(err.Error(), "API key not valid") {
		t.Fatalf("wrong key = %v", err)
	}
	if _, err := auth.SignInWithGoogle(ctx, idToken[:len(idToken)-2]+"xx"); firebaseCode(err) != "INVALID_IDP_RESPONSE" {
		t.Fatalf("tampered token = %v", err)
	}
	clock.Advance(time.Hour)
	if _, err := auth.SignInWithGoogle(ctx, idToken); firebaseCode(err) != "INVALID_IDP_RESPONSE" {
		t.Fatalf("expired token = %v", err)
	}
}

// A Firebase ID token lasts an hour, after which the database refuses it;
// the refresh token, which does not expire, gets a new one. An unknown or
// missing refresh token is refused with its code, and a request with
// another grant type, which no correct client sends, too.
func TestFirebaseTokenRefresh(t *testing.T) {
	clock := newTestClock()
	e := startEmulator(t, t.TempDir(), Options{Now: clock.Now})
	e.AddTestUser(owner)
	e.InstallRules(owner)
	auth, session := firebaseAs(t, e, owner)
	db := databaseOf(e)
	clock.Advance(time.Hour)
	var value any
	err := db.Get(ctx, members, session.IDToken, &value)
	var dbErr *firebase.DatabaseError
	if !errors.As(err, &dbErr) || dbErr.Status != http.StatusUnauthorized || dbErr.Message != "Auth token is expired" {
		t.Fatalf("expired token = %v", err)
	}
	clock.Advance(30 * 24 * time.Hour)
	renewed, err := auth.Refresh(ctx, session)
	if err != nil || renewed.UID != session.UID || renewed.RefreshToken != session.RefreshToken {
		t.Fatalf("refresh = %+v, %v", renewed, err)
	}
	if err := db.Get(ctx, members, renewed.IDToken, &value); err != nil {
		t.Fatalf("with the refreshed token: %v", err)
	}
	if _, err := auth.Refresh(ctx, firebase.Session{RefreshToken: "unknown"}); firebaseCode(err) != "INVALID_REFRESH_TOKEN" {
		t.Fatalf("unknown refresh token = %v", err)
	}
	if _, err := auth.Refresh(ctx, firebase.Session{}); firebaseCode(err) != "MISSING_REFRESH_TOKEN" {
		t.Fatalf("missing refresh token = %v", err)
	}
	_, apiKey := e.Firebase()
	response, err := http.PostForm(e.URL()+"/v1/token?key="+apiKey, url.Values{"grant_type": {"password"}, "refresh_token": {session.RefreshToken}})
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("another grant type = %d", response.StatusCode)
	}
}

// boardUsers sets up the owner's database with the rules installed and a
// members list naming an editor as writer and a viewer as reader, and
// returns their sessions and a stranger's.
func boardUsers(t *testing.T, e *Emulator) (ownerSession, editorSession, viewerSession, strangerSession firebase.Session) {
	t.Helper()
	for _, account := range []string{owner, editor, viewer, stranger} {
		e.AddTestUser(account)
	}
	e.InstallRules(owner)
	_, ownerSession = firebaseAs(t, e, owner)
	_, editorSession = firebaseAs(t, e, editor)
	_, viewerSession = firebaseAs(t, e, viewer)
	_, strangerSession = firebaseAs(t, e, stranger)
	if err := databaseOf(e).Put(ctx, members, ownerSession.IDToken, map[string]string{emailKey(owner): "writer", emailKey(editor): "writer", emailKey(viewer): "reader"}); err != nil {
		t.Fatal(err)
	}
	return
}

// Until the owner installs the rules, the database refuses every request,
// as a new database's locked mode does.
func TestDatabaseIsLockedUntilRulesAreInstalled(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	e.AddTestUser(owner)
	_, session := firebaseAs(t, e, owner)
	db := databaseOf(e)
	err := db.Put(ctx, members, session.IDToken, map[string]string{"a": "writer"})
	var dbErr *firebase.DatabaseError
	if !errors.As(err, &dbErr) || dbErr.Status != http.StatusUnauthorized || dbErr.Message != "Permission denied" {
		t.Fatalf("before the rules = %v", err)
	}
	e.InstallRules(owner)
	if err := db.Put(ctx, members, session.IDToken, map[string]string{"a": "writer"}); err != nil {
		t.Fatalf("after the rules = %v", err)
	}
}

// D-066: the security rules. Only the owner writes the members list; each
// account reads its own entry; writers write the board only under their own
// user ID; members read the board; a non-member may only write its own
// membership request, which the owner reads; nothing is open to an
// unauthenticated request.
func TestSecurityRules(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	ownerS, editorS, viewerS, strangerS := boardUsers(t, e)
	db := databaseOf(e)
	entry := map[string]any{"path": "devices/d1/segments/segment-1", "size": 10}
	strangerKey := emailKey(stranger)
	requests := "eventlog/" + logID + "/requests"
	for _, c := range []struct {
		name  string
		token string
		write bool
		path  string
		value any
		allow bool
	}{
		{"editor writes its board entry", editorS.IDToken, true, board + "/" + editorS.UID + "/d1", entry, true},
		{"editor writes under another user ID", editorS.IDToken, true, board + "/someone-else/d1", entry, false},
		{"editor writes the whole board", editorS.IDToken, true, board, map[string]any{"x": 1}, false},
		{"viewer writes the board", viewerS.IDToken, true, board + "/" + viewerS.UID + "/d2", entry, false},
		{"viewer reads the board", viewerS.IDToken, false, board, nil, true},
		{"editor reads the board", editorS.IDToken, false, board, nil, true},
		{"owner reads the board", ownerS.IDToken, false, board, nil, true},
		{"stranger reads the board", strangerS.IDToken, false, board, nil, false},
		{"stranger writes its request", strangerS.IDToken, true, requests + "/" + strangerKey, true, true},
		{"stranger writes another request", strangerS.IDToken, true, requests + "/" + emailKey(viewer), true, false},
		{"stranger reads requests", strangerS.IDToken, false, requests, nil, false},
		{"owner reads requests", ownerS.IDToken, false, requests, nil, true},
		{"editor writes the members list", editorS.IDToken, true, members + "/" + strangerKey, "writer", false},
		{"viewer reads its own entry", viewerS.IDToken, false, members + "/" + emailKey(viewer), nil, true},
		{"viewer reads another entry", viewerS.IDToken, false, members + "/" + emailKey(editor), nil, false},
		{"viewer reads the members list", viewerS.IDToken, false, members, nil, false},
		{"anyone reads the root", ownerS.IDToken, false, "", nil, false},
		{"unauthenticated reads the board", "", false, board, nil, false},
	} {
		var err error
		if c.write {
			err = db.Put(ctx, c.path, c.token, c.value)
		} else {
			var value any
			err = db.Get(ctx, c.path, c.token, &value)
		}
		if allowed := err == nil; allowed != c.allow {
			t.Errorf("%s: %v, want allowed %v", c.name, err, c.allow)
		}
	}
	if role := e.Database(members + "/" + strangerKey); role != nil {
		t.Fatalf("a refused write changed the members list: %v", role)
	}
}

// Writes: PATCH merges children and accepts paths as keys, DELETE removes,
// empty objects vanish; malformed JSON, which no correct client sends, is
// refused.
func TestDatabaseWrites(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	ownerS, _, _, _ := boardUsers(t, e)
	db := databaseOf(e)
	if err := db.Patch(ctx, members, ownerS.IDToken, map[string]any{"a": "writer", "b/c": "nested"}); err != nil {
		t.Fatal(err)
	}
	got := e.Database(members).(map[string]any)
	if got["a"] != "writer" || jsonOf(got["b"]) != `{"c":"nested"}` || got[emailKey(owner)] != "writer" {
		t.Fatalf("after patch = %v", got)
	}
	if err := db.Delete(ctx, members+"/b/c", ownerS.IDToken); err != nil {
		t.Fatal(err)
	}
	if e.Database(members+"/b") != nil {
		t.Fatal("an empty object was kept")
	}
	databaseURL, _ := e.Firebase()
	request, _ := http.NewRequest("PUT", databaseURL+"/"+members+".json?auth="+ownerS.IDToken, strings.NewReader("{broken"))
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("malformed JSON = %d", response.StatusCode)
	}
}

// nextEvent waits for a stream's next event.
func nextEvent(t *testing.T, s *firebase.Stream) firebase.Event {
	t.Helper()
	type result struct {
		ev  firebase.Event
		err error
	}
	done := make(chan result, 1)
	go func() {
		ev, err := s.Next()
		done <- result{ev, err}
	}()
	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("stream: %v", r.err)
		}
		return r.ev
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for an event")
		return firebase.Event{}
	}
}

func expectEvent(t *testing.T, s *firebase.Stream, kind, path, data string) {
	t.Helper()
	if ev := nextEvent(t, s); ev.Type != kind || ev.Path != path || string(ev.Data) != data {
		t.Fatalf("event %s %s %s, want %s %s %s", ev.Type, ev.Path, ev.Data, kind, path, data)
	}
}

// expectEnded checks the stream has ended.
func expectEnded(t *testing.T, s *firebase.Stream) {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		_, err := s.Next()
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("the stream did not end")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the stream did not end")
	}
}

func openStream(t *testing.T, e *Emulator, path, token string) *firebase.Stream {
	t.Helper()
	s, err := databaseOf(e).Stream(ctx, path, token)
	if err != nil {
		t.Fatalf("stream %s: %v", path, err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// D-063: a stream starts with the location's whole value, then carries a
// put for each changed value and a patch for each PATCH, with paths
// relative to the location; writes that change nothing, or elsewhere, send
// nothing; a write above the location sends its new value.
func TestStreamEvents(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	ownerS, editorS, viewerS, _ := boardUsers(t, e)
	db := databaseOf(e)
	entry := board + "/" + editorS.UID + "/d1"
	if err := db.Put(ctx, entry, editorS.IDToken, map[string]any{"path": "a", "size": 1}); err != nil {
		t.Fatal(err)
	}
	s := openStream(t, e, board, viewerS.IDToken)
	expectEvent(t, s, firebase.Put, "/", `{"`+editorS.UID+`":{"d1":{"path":"a","size":1}}}`)
	if err := db.Put(ctx, entry+"/size", editorS.IDToken, 2); err != nil {
		t.Fatal(err)
	}
	expectEvent(t, s, firebase.Put, "/"+editorS.UID+"/d1/size", "2")
	if err := db.Patch(ctx, entry, editorS.IDToken, map[string]any{"path": "b", "size": 3}); err != nil {
		t.Fatal(err)
	}
	expectEvent(t, s, firebase.Patch, "/"+editorS.UID+"/d1", `{"path":"b","size":3}`)
	_ = db.Put(ctx, entry+"/size", editorS.IDToken, 3)
	_ = db.Put(ctx, "eventlog/other-log/members", ownerS.IDToken, map[string]string{"x": "writer"})
	if err := db.Delete(ctx, entry, editorS.IDToken); err != nil {
		t.Fatal(err)
	}
	expectEvent(t, s, firebase.Put, "/"+editorS.UID+"/d1", "null")

	own := openStream(t, e, members+"/"+emailKey(viewer), viewerS.IDToken)
	expectEvent(t, own, firebase.Put, "/", `"reader"`)
	if err := db.Put(ctx, members, ownerS.IDToken, map[string]string{emailKey(owner): "writer", emailKey(viewer): "writer"}); err != nil {
		t.Fatal(err)
	}
	expectEvent(t, own, firebase.Put, "/", `"writer"`)
}

// A stream is redirected once to the server holding the data; the client
// follows it, so this is checked with a raw request that does not.
func TestStreamIsRedirected(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	_, _, viewerS, _ := boardUsers(t, e)
	databaseURL, _ := e.Firebase()
	request, _ := http.NewRequest("GET", databaseURL+"/"+board+".json?auth="+viewerS.IDToken, nil)
	request.Header.Set("Accept", "text/event-stream")
	noRedirects := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := noRedirects.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusTemporaryRedirect || !strings.Contains(response.Header.Get("Location"), "ns=") {
		t.Fatalf("first stream request = %d %q", response.StatusCode, response.Header.Get("Location"))
	}
}

// An idle stream gets keep-alive, which the client skips; it shows in the
// streamed bytes.
func TestStreamKeepAlive(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{KeepAlive: 20 * time.Millisecond})
	_, _, viewerS, _ := boardUsers(t, e)
	s := openStream(t, e, board, viewerS.IDToken)
	nextEvent(t, s)
	before := e.Stats()[viewer]["rtdb.stream"].ResponseBytes
	time.Sleep(150 * time.Millisecond)
	if grown := e.Stats()[viewer]["rtdb.stream"].ResponseBytes - before; grown < int64(len("event: keep-alive\ndata: null\n\n")) {
		t.Fatalf("streamed %d bytes while idle", grown)
	}
}

// When the rules stop allowing a stream's read, as when the owner removes
// the account from the members list, the stream sends cancel and ends.
func TestStreamCancelledWhenAccessIsRemoved(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	ownerS, _, viewerS, _ := boardUsers(t, e)
	s := openStream(t, e, board, viewerS.IDToken)
	nextEvent(t, s)
	if err := databaseOf(e).Delete(ctx, members+"/"+emailKey(viewer), ownerS.IDToken); err != nil {
		t.Fatal(err)
	}
	if ev := nextEvent(t, s); ev.Type != firebase.Cancel {
		t.Fatalf("after removal = %+v", ev)
	}
	if _, err := s.Next(); !errors.Is(err, io.EOF) {
		t.Fatalf("after cancel = %v", err)
	}
	_, err := databaseOf(e).Stream(ctx, board, viewerS.IDToken)
	expectDatabaseError(t, "reopening", err, http.StatusUnauthorized)
}

// When a stream's sign-in expires, it sends auth_revoked and ends.
func TestStreamAuthRevokedWhenTokenExpires(t *testing.T) {
	clock := newTestClock()
	e := startEmulator(t, t.TempDir(), Options{Now: clock.Now, KeepAlive: 20 * time.Millisecond})
	_, _, viewerS, _ := boardUsers(t, e)
	s := openStream(t, e, board, viewerS.IDToken)
	nextEvent(t, s)
	clock.Advance(time.Hour)
	ev := nextEvent(t, s)
	var reason string
	_ = json.Unmarshal(ev.Data, &reason)
	if ev.Type != firebase.AuthRevoked || reason != "credential is no longer valid" {
		t.Fatalf("after expiry = %+v", ev)
	}
	expectEnded(t, s)
}

// A dropped connection ends the stream without an event; reconnecting
// starts again with the whole current value, changes made meanwhile
// included.
func TestDroppedStreamsReconnect(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	_, editorS, viewerS, _ := boardUsers(t, e)
	s := openStream(t, e, board, viewerS.IDToken)
	nextEvent(t, s)
	e.DropStreams()
	expectEnded(t, s)
	if err := databaseOf(e).Put(ctx, board+"/"+editorS.UID+"/d1", editorS.IDToken, map[string]int{"size": 5}); err != nil {
		t.Fatal(err)
	}
	again := openStream(t, e, board, viewerS.IDToken)
	expectEvent(t, again, firebase.Put, "/", `{"`+editorS.UID+`":{"d1":{"size":5}}}`)
}

// The free plan allows 100 connections at once; beyond that a connection is
// refused until one closes.
func TestConnectionLimit(t *testing.T) {
	if e := startEmulator(t, t.TempDir(), Options{}); e.options.MaxConnections != 100 {
		t.Fatalf("default limit = %d, want the free plan's 100", e.options.MaxConnections)
	}
	e := startEmulator(t, t.TempDir(), Options{MaxConnections: 3})
	ownerS, _, viewerS, _ := boardUsers(t, e)
	var streams []*firebase.Stream
	for i := 0; i < 3; i++ {
		s := openStream(t, e, board, viewerS.IDToken)
		nextEvent(t, s)
		streams = append(streams, s)
	}
	var value any
	err := databaseOf(e).Get(ctx, board, ownerS.IDToken, &value)
	var dbErr *firebase.DatabaseError
	if !errors.As(err, &dbErr) || !dbErr.Unavailable() {
		t.Fatalf("a request over the limit = %v", err)
	}
	_ = streams[0].Close()
	deadline := time.Now().Add(5 * time.Second)
	for e.OpenStreams() > 2 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if err := databaseOf(e).Get(ctx, board, ownerS.IDToken, &value); err != nil {
		t.Fatalf("after a stream closed = %v", err)
	}
}

// When the database is switched off, as for exceeding the free plan,
// streams are dropped and requests refused until it is switched on.
func TestDatabaseSwitchedOff(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	ownerS, _, viewerS, _ := boardUsers(t, e)
	s := openStream(t, e, board, viewerS.IDToken)
	nextEvent(t, s)
	e.SwitchOffDatabase()
	expectEnded(t, s)
	var value any
	err := databaseOf(e).Get(ctx, board, ownerS.IDToken, &value)
	expectDatabaseError(t, "while off", err, http.StatusServiceUnavailable)
	_, err = databaseOf(e).Stream(ctx, board, viewerS.IDToken)
	expectDatabaseError(t, "stream while off", err, http.StatusServiceUnavailable)
	e.SwitchOnDatabase()
	if err := databaseOf(e).Get(ctx, board, ownerS.IDToken, &value); err != nil {
		t.Fatalf("after switching on = %v", err)
	}
}

// Board writes, connections, and streamed bytes are counted per account,
// a stream's bytes while it is still open.
func TestDatabaseCallsAreCounted(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	_, editorS, viewerS, _ := boardUsers(t, e)
	e.ResetStats()
	s := openStream(t, e, board, viewerS.IDToken)
	nextEvent(t, s)
	if err := databaseOf(e).Put(ctx, board+"/"+editorS.UID+"/d1", editorS.IDToken, map[string]int{"size": 1}); err != nil {
		t.Fatal(err)
	}
	nextEvent(t, s)
	stats := e.Stats()
	if writes := stats[editor]["rtdb.put"]; writes.Requests != 1 || writes.RequestBytes == 0 {
		t.Fatalf("editor writes = %+v", writes)
	}
	if streamed := stats[viewer]["rtdb.stream"]; streamed.Requests != 1 || streamed.ResponseBytes == 0 {
		t.Fatalf("viewer stream while open = %+v", streamed)
	}
}

// The database is kept on the real file system as a JSON file, with the
// rules and Firebase sign-ins in the metadata, so a restart keeps them.
func TestDatabaseSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	e := startEmulator(t, dir, Options{})
	_, editorS, viewerS, _ := boardUsers(t, e)
	if err := databaseOf(e).Put(ctx, board+"/"+editorS.UID+"/d1", editorS.IDToken, map[string]int{"size": 7}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, databaseFileName))
	if err != nil || !strings.Contains(string(raw), `"size":7`) {
		t.Fatalf("database on disk = %s, %v", raw, err)
	}
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
	restarted := startEmulator(t, dir, Options{})
	var size int
	if err := databaseOf(restarted).Get(ctx, board+"/"+editorS.UID+"/d1/size", viewerS.IDToken, &size); err != nil || size != 7 {
		t.Fatalf("after restart = %d, %v", size, err)
	}
}
