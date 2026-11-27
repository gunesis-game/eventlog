package googletest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"time"
)

// databasePrefix is where the Realtime Database is served; the database URL
// the settings file names is the emulator's address followed by it.
const databasePrefix = "/rtdb"

// streamBuffer is how many events a stream may fall behind before the
// emulator drops it, as a server drops a client that does not keep up.
const streamBuffer = 256

// stream is one open server-sent event stream of a database location.
type stream struct {
	path    []string
	claims  *idClaims
	events  chan streamEvent
	dropped chan struct{}
	once    sync.Once
}

// streamEvent is one event of a stream; a final event ends the stream.
type streamEvent struct {
	name  string
	data  any
	final bool
}

func (s *stream) drop() {
	s.once.Do(func() { close(s.dropped) })
}

// InstallRules installs eventlog's published security rules with the
// owner's address filled in, as the owner pastes them into the Firebase
// console. Until then the database refuses every request, as a new
// database's locked mode does.
func (e *Emulator) InstallRules(owner string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.state.Firebase.RulesOwner = strings.ToLower(owner)
	e.saveLocked()
}

// SwitchOffDatabase switches the database off, as Firebase does when the
// free plan's monthly limits are exceeded: open streams are dropped and every
// request is refused until SwitchOnDatabase.
func (e *Emulator) SwitchOffDatabase() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.state.Firebase.Off = true
	e.saveLocked()
	for s := range e.streams {
		s.drop()
	}
}

// SwitchOnDatabase switches the database on again, as at the start of a new
// month.
func (e *Emulator) SwitchOnDatabase() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.state.Firebase.Off = false
	e.saveLocked()
}

// DropStreams drops every open stream without an event, as a lost network
// connection does.
func (e *Emulator) DropStreams() {
	e.mu.Lock()
	defer e.mu.Unlock()
	for s := range e.streams {
		s.drop()
	}
}

// OpenStreams returns how many streams are open.
func (e *Emulator) OpenStreams() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.streams)
}

// Database returns the value stored at a database path such as
// "eventlog/<logID>/devices", or nil.
func (e *Emulator) Database(path string) any {
	e.mu.Lock()
	defer e.mu.Unlock()
	return valueAt(e.database, splitPath(path))
}

func splitPath(path string) []string {
	var segments []string
	for _, segment := range strings.Split(path, "/") {
		if segment != "" {
			segments = append(segments, segment)
		}
	}
	return segments
}

func databaseError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// databaseRequest serves the Realtime Database's REST interface: GET, PUT,
// PATCH, and DELETE of a location's JSON, and a GET with Accept:
// text/event-stream for a stream of its changes.
func (e *Emulator) databaseRequest(w http.ResponseWriter, r *http.Request) {
	streaming := r.Method == http.MethodGet && strings.Contains(r.Header.Get("Accept"), "text/event-stream")
	c := named(r, "rtdb."+strings.ToLower(r.Method))
	if streaming {
		c.name = "rtdb.stream"
	}
	location, ok := strings.CutSuffix(strings.TrimPrefix(r.URL.Path, databasePrefix), ".json")
	if !ok {
		databaseError(w, http.StatusNotFound, "Not Found")
		return
	}
	path := splitPath(location)
	query := r.URL.Query()
	if streaming && query.Get("ns") == "" {
		// Streams are redirected to the server holding the data.
		e.mu.Lock()
		query.Set("ns", e.state.Firebase.ProjectID)
		e.mu.Unlock()
		redirect := *r.URL
		redirect.RawQuery = query.Encode()
		http.Redirect(w, r, redirect.String(), http.StatusTemporaryRedirect)
		return
	}

	e.mu.Lock()
	var claims *idClaims
	if token := query.Get("auth"); token != "" {
		verified, err := e.verifyLocked(token)
		if err != nil {
			e.mu.Unlock()
			databaseError(w, http.StatusUnauthorized, err.Error())
			return
		}
		claims = &verified
		c.account = verified.Email
	}
	switch {
	case e.state.Firebase.Off:
		e.mu.Unlock()
		databaseError(w, http.StatusServiceUnavailable, "The database has been disabled for exceeding the plan's limits.")
		return
	case e.connections >= e.options.MaxConnections:
		e.mu.Unlock()
		databaseError(w, http.StatusServiceUnavailable, "Too many simultaneous connections for the plan.")
		return
	}
	e.connections++
	defer func() {
		e.mu.Lock()
		e.connections--
		e.mu.Unlock()
	}()

	if r.Method == http.MethodGet {
		if !e.canReadLocked(path, claims) {
			e.mu.Unlock()
			databaseError(w, http.StatusUnauthorized, "Permission denied")
			return
		}
		value := valueAt(e.database, path)
		if !streaming {
			e.mu.Unlock()
			writeJSON(w, http.StatusOK, value)
			return
		}
		s := &stream{path: path, claims: claims, events: make(chan streamEvent, streamBuffer), dropped: make(chan struct{})}
		e.streams[s] = struct{}{}
		e.mu.Unlock()
		e.countRequest(c, 0)
		e.serveStream(w, r, s, value)
		return
	}

	var body any
	if r.Method != http.MethodDelete {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			e.mu.Unlock()
			databaseError(w, http.StatusBadRequest, "Invalid data; couldn't parse JSON object, array, or value.")
			return
		}
	}
	writes := map[string]any{}
	switch r.Method {
	case http.MethodPut:
		writes[""] = body
	case http.MethodDelete:
		writes[""] = nil
	case http.MethodPatch:
		children, ok := body.(map[string]any)
		if !ok {
			e.mu.Unlock()
			databaseError(w, http.StatusBadRequest, "Invalid data; couldn't parse JSON object.")
			return
		}
		writes = children
	default:
		e.mu.Unlock()
		databaseError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	for child := range writes {
		if !e.canWriteLocked(append(slices.Clone(path), splitPath(child)...), claims) {
			e.mu.Unlock()
			databaseError(w, http.StatusUnauthorized, "Permission denied")
			return
		}
	}
	e.writeLocked(path, writes, r.Method == http.MethodPatch)
	e.mu.Unlock()
	if query.Get("print") == "silent" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(w, http.StatusOK, body)
}

// writeLocked applies writes, each a value for a path below path, saves the
// database, and sends each stream whose location changed its event.
func (e *Emulator) writeLocked(path []string, writes map[string]any, patch bool) {
	before := e.database
	root := e.database
	for child, value := range writes {
		root = setAt(root, append(slices.Clone(path), splitPath(child)...), normalize(value))
	}
	e.database = root
	raw, _ := json.Marshal(e.database)
	if err := writeAtomically(filepath.Join(e.dir, databaseFileName), raw); err != nil {
		panic(fmt.Sprintf("googletest: save database: %v", err))
	}
	for s := range e.streams {
		if !e.canReadLocked(s.path, s.claims) {
			e.sendLocked(s, streamEvent{name: "cancel", final: true})
			continue
		}
		if reflect.DeepEqual(valueAt(before, s.path), valueAt(root, s.path)) {
			continue
		}
		switch {
		case hasPrefix(path, s.path) && patch:
			e.sendLocked(s, streamEvent{name: "patch", data: map[string]any{"path": relative(path, s.path), "data": writes}})
		case hasPrefix(path, s.path):
			e.sendLocked(s, streamEvent{name: "put", data: map[string]any{"path": relative(path, s.path), "data": writes[""]}})
		default:
			e.sendLocked(s, streamEvent{name: "put", data: map[string]any{"path": "/", "data": valueAt(root, s.path)}})
		}
	}
}

// sendLocked queues an event for a stream, dropping the stream when it has
// fallen too far behind.
func (e *Emulator) sendLocked(s *stream, event streamEvent) {
	select {
	case s.events <- event:
	default:
		s.drop()
	}
}

// serveStream sends a stream's events: first a put of the location's whole
// value, then its changes, with keep-alive while idle. When the sign-in
// expires it sends auth_revoked, and when the rules stop allowing the read,
// cancel; either ends the stream.
func (e *Emulator) serveStream(w http.ResponseWriter, r *http.Request, s *stream, initial any) {
	defer func() {
		e.mu.Lock()
		delete(e.streams, s)
		e.mu.Unlock()
	}()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)
	send := func(event streamEvent) bool {
		data, _ := json.Marshal(event.data)
		if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.name, data); err != nil {
			return false
		}
		if flusher != nil {
			flusher.Flush()
		}
		return !event.final
	}
	if !send(streamEvent{name: "put", data: map[string]any{"path": "/", "data": initial}}) {
		return
	}
	ticker := time.NewTicker(e.options.KeepAlive)
	defer ticker.Stop()
	for {
		select {
		case event := <-s.events:
			if !send(event) {
				return
			}
		case <-ticker.C:
			e.mu.Lock()
			expired := s.claims != nil && e.options.Now().Unix() >= s.claims.Expires
			allowed := e.canReadLocked(s.path, s.claims)
			e.mu.Unlock()
			switch {
			case expired:
				send(streamEvent{name: "auth_revoked", data: "credential is no longer valid", final: true})
				return
			case !allowed:
				send(streamEvent{name: "cancel", final: true})
				return
			case !send(streamEvent{name: "keep-alive"}):
				return
			}
		case <-s.dropped:
			return
		case <-r.Context().Done():
			return
		}
	}
}

// The security rules (D-066), as published for the owner to install:
//
//	/eventlog/{logID}/members              read, write: the owner
//	/eventlog/{logID}/members/{emailKey}   read: that account
//	/eventlog/{logID}/requests             read: the owner
//	/eventlog/{logID}/requests/{emailKey}  write: that account
//	/eventlog/{logID}/devices              read: the owner and members
//	/eventlog/{logID}/devices/{uid}        write: the owner or a writer
//	                                       member signed in as that uid
//
// emailKey is the lowercased address with "." replaced by ",". As in
// Firebase, a grant at a location covers everything below it, and nothing
// is allowed that no rule grants.

func emailKey(email string) string {
	return strings.ReplaceAll(strings.ToLower(email), ".", ",")
}

// canReadLocked reports whether the rules let the signed-in account read
// path.
func (e *Emulator) canReadLocked(path []string, claims *idClaims) bool {
	return e.grantedLocked(path, claims, e.readRuleLocked)
}

// canWriteLocked reports whether the rules let the signed-in account write
// path.
func (e *Emulator) canWriteLocked(path []string, claims *idClaims) bool {
	return e.grantedLocked(path, claims, e.writeRuleLocked)
}

// grantedLocked reports whether a rule grants access at path or above it.
func (e *Emulator) grantedLocked(path []string, claims *idClaims, rule func([]string, *idClaims) bool) bool {
	if claims == nil || e.state.Firebase.RulesOwner == "" || e.state.Firebase.Off {
		return false
	}
	for depth := 1; depth <= len(path); depth++ {
		if rule(path[:depth], claims) {
			return true
		}
	}
	return false
}

func (e *Emulator) isOwner(claims *idClaims) bool {
	return claims.Email == e.state.Firebase.RulesOwner
}

// memberRoleLocked returns the account's role in a log's members list.
func (e *Emulator) memberRoleLocked(logID string, claims *idClaims) string {
	role, _ := valueAt(e.database, []string{"eventlog", logID, "members", emailKey(claims.Email)}).(string)
	return role
}

func (e *Emulator) readRuleLocked(path []string, claims *idClaims) bool {
	if len(path) < 3 || path[0] != "eventlog" {
		return false
	}
	switch {
	case len(path) == 3 && (path[2] == "members" || path[2] == "requests"):
		return e.isOwner(claims)
	case len(path) == 4 && path[2] == "members":
		return path[3] == emailKey(claims.Email)
	case len(path) == 3 && path[2] == "devices":
		role := e.memberRoleLocked(path[1], claims)
		return e.isOwner(claims) || role == roleWriter || role == roleReader
	}
	return false
}

func (e *Emulator) writeRuleLocked(path []string, claims *idClaims) bool {
	if len(path) < 3 || path[0] != "eventlog" {
		return false
	}
	switch {
	case len(path) == 3 && path[2] == "members":
		return e.isOwner(claims)
	case len(path) == 4 && path[2] == "requests":
		return path[3] == emailKey(claims.Email)
	case len(path) == 4 && path[2] == "devices":
		writer := e.isOwner(claims) || e.memberRoleLocked(path[1], claims) == roleWriter
		return writer && path[3] == claims.UserID
	}
	return false
}

func hasPrefix(path, prefix []string) bool {
	return len(path) >= len(prefix) && slices.Equal(path[:len(prefix)], prefix)
}

// relative returns path below base as a stream event path such as "/a/b".
func relative(path, base []string) string {
	return "/" + strings.Join(path[len(base):], "/")
}

// valueAt returns the value at path in a JSON tree, or nil.
func valueAt(root any, path []string) any {
	for _, segment := range path {
		children, ok := root.(map[string]any)
		if !ok {
			return nil
		}
		root = children[segment]
	}
	return root
}

// setAt returns a tree with value at path, copying only the objects along
// the path so earlier trees stay unchanged. A nil value removes the path,
// and objects left empty are removed, as the database keeps no empty
// objects.
func setAt(root any, path []string, value any) any {
	if len(path) == 0 {
		return value
	}
	children, _ := root.(map[string]any)
	copied := make(map[string]any, len(children)+1)
	for key, child := range children {
		copied[key] = child
	}
	if child := setAt(copied[path[0]], path[1:], value); child == nil {
		delete(copied, path[0])
	} else {
		copied[path[0]] = child
	}
	if len(copied) == 0 {
		return nil
	}
	return copied
}

// normalize removes nulls and empty objects from a written value.
func normalize(value any) any {
	children, ok := value.(map[string]any)
	if !ok {
		return value
	}
	out := make(map[string]any, len(children))
	for key, child := range children {
		if child = normalize(child); child != nil {
			out[key] = child
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
