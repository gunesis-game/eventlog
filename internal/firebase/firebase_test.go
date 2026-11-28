package firebase_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gunesis-game/eventlog/internal/firebase"
	"github.com/gunesis-game/eventlog/internal/googleauth"
	"github.com/gunesis-game/eventlog/internal/googletest"
)

const (
	owner   = "owner@example.com"
	viewer  = "viewer@example.com"
	board   = "eventlog/log1/devices"
	members = "eventlog/log1/members"
)

var ctx = context.Background()

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

func emulator(t *testing.T, options googletest.Options) *googletest.Emulator {
	t.Helper()
	e, err := googletest.New(t.TempDir(), options)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.Close() })
	for _, account := range []string{owner, viewer} {
		e.AddTestUser(account)
	}
	e.InstallRules(owner)
	return e
}

// signIn signs an account in to Google with the openid scope and exchanges
// its ID token for a Firebase session, all through the emulator.
func signIn(t *testing.T, e *googletest.Emulator, email string) (*firebase.Auth, firebase.Session) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "client.json")
	if err := e.WriteClientFile(path); err != nil {
		t.Fatal(err)
	}
	client, err := googleauth.ReadClientFile(path)
	if err != nil {
		t.Fatal(err)
	}
	a := client.Authorize("http://127.0.0.1:8765", []string{googleauth.ScopeDrive, googleauth.ScopeOpenID, googleauth.ScopeEmail}, email)
	redirect, err := e.Browse(a.URL, email)
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
	_, apiKey := e.Firebase()
	auth := firebase.NewAuth(http.DefaultClient, apiKey, e.URL(), e.URL())
	session, err := auth.SignInWithGoogle(ctx, token.IDToken)
	if err != nil {
		t.Fatal(err)
	}
	return auth, session
}

func database(e *googletest.Emulator) *firebase.Database {
	url, _ := e.Firebase()
	return firebase.NewDatabase(http.DefaultClient, url)
}

// A Google sign-in becomes a Firebase session, which a refresh renews; an
// unknown refresh token is refused with its error code.
func TestSessions(t *testing.T) {
	e := emulator(t, googletest.Options{})
	auth, session := signIn(t, e, owner)
	if session.UID == "" || session.Email != owner || session.IDToken == "" || session.RefreshToken == "" || time.Until(session.Expiry) < 50*time.Minute {
		t.Fatalf("session = %+v", session)
	}
	renewed, err := auth.Refresh(ctx, session)
	if err != nil || renewed.UID != session.UID || renewed.IDToken == "" || renewed.Email != owner {
		t.Fatalf("refresh = %+v, %v", renewed, err)
	}
	_, err = auth.Refresh(ctx, firebase.Session{RefreshToken: "unknown"})
	var authErr *firebase.AuthError
	if !errors.As(err, &authErr) || authErr.Code() != "INVALID_REFRESH_TOKEN" {
		t.Fatalf("unknown refresh token = %v", err)
	}
}

// Locations are written, merged, read, and deleted; the rules' refusal is
// a 401 database error.
func TestReadsAndWrites(t *testing.T) {
	e := emulator(t, googletest.Options{})
	_, ownerSession := signIn(t, e, owner)
	_, viewerSession := signIn(t, e, viewer)
	db := database(e)
	if err := db.Put(ctx, members, ownerSession.IDToken, map[string]string{"owner@example,com": "writer"}); err != nil {
		t.Fatal(err)
	}
	if err := db.Patch(ctx, members, ownerSession.IDToken, map[string]any{"viewer@example,com": "reader"}); err != nil {
		t.Fatal(err)
	}
	var list map[string]string
	if err := db.Get(ctx, members, ownerSession.IDToken, &list); err != nil || len(list) != 2 || list["viewer@example,com"] != "reader" {
		t.Fatalf("members = %v, %v", list, err)
	}
	entry := board + "/" + viewerSession.UID + "/d1"
	err := db.Put(ctx, entry, viewerSession.IDToken, map[string]any{"size": 1})
	var dbErr *firebase.DatabaseError
	if !errors.As(err, &dbErr) || dbErr.Status != http.StatusUnauthorized || dbErr.Message != "Permission denied" {
		t.Fatalf("a reader writing the board = %v", err)
	}
	if err := db.Delete(ctx, members+"/viewer@example,com", ownerSession.IDToken); err != nil {
		t.Fatal(err)
	}
	var gone any
	if err := db.Get(ctx, members+"/viewer@example,com", ownerSession.IDToken, &gone); err != nil || gone != nil {
		t.Fatalf("after delete = %v, %v", gone, err)
	}
}

// A stream starts with the whole data, then carries puts and patches with
// paths relative to the location; it ends with cancel when the rules stop
// allowing the read.
func TestStream(t *testing.T) {
	e := emulator(t, googletest.Options{})
	_, ownerSession := signIn(t, e, owner)
	_, viewerSession := signIn(t, e, viewer)
	db := database(e)
	if err := db.Put(ctx, members, ownerSession.IDToken, map[string]string{"owner@example,com": "writer", "viewer@example,com": "reader"}); err != nil {
		t.Fatal(err)
	}
	stream, err := db.Stream(ctx, board, viewerSession.IDToken)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	expect := func(kind, path, data string) {
		t.Helper()
		ev, err := stream.Next()
		if err != nil || ev.Type != kind || ev.Path != path || string(ev.Data) != data {
			t.Fatalf("event = %+v (%s), %v; want %s %s %s", ev, ev.Data, err, kind, path, data)
		}
	}
	expect(firebase.Put, "/", "null")
	entry := board + "/" + ownerSession.UID + "/d1"
	if err := db.Put(ctx, entry, ownerSession.IDToken, map[string]int{"size": 1}); err != nil {
		t.Fatal(err)
	}
	expect(firebase.Put, "/"+ownerSession.UID+"/d1", `{"size":1}`)
	if err := db.Patch(ctx, entry, ownerSession.IDToken, map[string]any{"size": 2}); err != nil {
		t.Fatal(err)
	}
	expect(firebase.Patch, "/"+ownerSession.UID+"/d1", `{"size":2}`)
	if err := db.Delete(ctx, members+"/viewer@example,com", ownerSession.IDToken); err != nil {
		t.Fatal(err)
	}
	if ev, err := stream.Next(); err != nil || ev.Type != firebase.Cancel {
		t.Fatalf("after removal = %+v, %v", ev, err)
	}
	if _, err := stream.Next(); !errors.Is(err, io.EOF) {
		t.Fatalf("after cancel = %v", err)
	}
}

// When the ID token expires, an open stream gets auth_revoked and ends.
func TestStreamAuthRevoked(t *testing.T) {
	c := &clock{now: time.Now()}
	e := emulator(t, googletest.Options{Now: c.Now, KeepAlive: 20 * time.Millisecond})
	_, ownerSession := signIn(t, e, owner)
	db := database(e)
	if err := db.Put(ctx, members, ownerSession.IDToken, map[string]string{"owner@example,com": "writer"}); err != nil {
		t.Fatal(err)
	}
	stream, err := db.Stream(ctx, board, ownerSession.IDToken)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	if _, err := stream.Next(); err != nil {
		t.Fatal(err)
	}
	c.Advance(time.Hour)
	ev, err := stream.Next()
	var reason string
	_ = json.Unmarshal(ev.Data, &reason)
	if err != nil || ev.Type != firebase.AuthRevoked || !strings.Contains(reason, "credential") {
		t.Fatalf("after expiry = %+v, %v", ev, err)
	}
}

// A switched-off database answers 503, reported as unavailable.
func TestUnavailable(t *testing.T) {
	e := emulator(t, googletest.Options{})
	_, session := signIn(t, e, owner)
	e.SwitchOffDatabase()
	_, err := database(e).Stream(ctx, board, session.IDToken)
	var dbErr *firebase.DatabaseError
	if !errors.As(err, &dbErr) || !dbErr.Unavailable() {
		t.Fatalf("switched off = %v", err)
	}
}
