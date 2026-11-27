package googletest

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	defaultPageSize = 100
	maxPageSize     = 1000
	timeLayout      = "2006-01-02T15:04:05.000Z"
)

// fileMetadata is the writable metadata of a create or update request.
type fileMetadata struct {
	Name          *string            `json:"name"`
	MimeType      *string            `json:"mimeType"`
	Parents       []string           `json:"parents"`
	AppProperties map[string]*string `json:"appProperties"`
	Trashed       *bool              `json:"trashed"`
}

// resourceLocked renders f as a Drive files resource, as seen by account.
func (e *Emulator) resourceLocked(f *file, account string) map[string]any {
	owner := e.state.Accounts[f.Owner]
	capabilities := map[string]any{}
	for name, allowed := range e.capabilitiesLocked(f, account) {
		capabilities[name] = allowed
	}
	parents := make([]any, len(f.Parents))
	for i, parent := range f.Parents {
		parents[i] = parent
	}
	resource := map[string]any{
		"kind":         "drive#file",
		"id":           f.ID,
		"name":         f.Name,
		"mimeType":     f.MimeType,
		"parents":      parents,
		"trashed":      f.Trashed,
		"createdTime":  f.CreatedTime.UTC().Format(timeLayout),
		"modifiedTime": f.ModifiedTime.UTC().Format(timeLayout),
		"ownedByMe":    f.Owner == account,
		"owners": []any{map[string]any{
			"kind": "drive#user", "emailAddress": owner.Email, "permissionId": owner.PermissionID, "me": f.Owner == account,
		}},
		"capabilities": capabilities,
	}
	if len(f.AppProperties) > 0 {
		properties := map[string]any{}
		for key, value := range f.AppProperties {
			properties[key] = value
		}
		resource["appProperties"] = properties
	}
	if f.MimeType != folderMimeType {
		resource["size"] = strconv.FormatInt(f.Size, 10)
	}
	return resource
}

// lookupLocked finds a file the account can see, with "root" naming its My
// Drive. It writes a 404 and returns nil for anything the account has no
// access to, as Drive does.
func (e *Emulator) lookupLocked(w http.ResponseWriter, id, account string) (*file, string) {
	if id == "root" {
		id = e.state.Accounts[account].RootID
	}
	f := e.state.Files[id]
	role := ""
	if f != nil {
		role = e.roleOfLocked(f, account)
	}
	if role == "" {
		driveError(w, http.StatusNotFound, "notFound", "File not found: "+id+".")
		return nil, ""
	}
	return f, role
}

func fieldsOf(r *http.Request, fallback string) (fieldSet, error) {
	spec := r.URL.Query().Get("fields")
	if spec == "" {
		spec = fallback
	}
	return parseFields(spec)
}

// listFiles is files.list: the files the account can see that match q, in
// creation order or by orderBy, a page at a time.
func (e *Emulator) listFiles(w http.ResponseWriter, r *http.Request, c *call) {
	c.name = "drive.files.list"
	q := r.URL.Query()
	match, err := parseQuery(q.Get("q"))
	if err != nil {
		driveError(w, http.StatusBadRequest, "invalid", "Invalid Value: q: "+err.Error())
		return
	}
	fields, err := fieldsOf(r, defaultFileListFields)
	if err != nil {
		driveError(w, http.StatusBadRequest, "invalidParameter", "Invalid field selection: "+err.Error())
		return
	}
	pageSize := defaultPageSize
	if raw := q.Get("pageSize"); raw != "" {
		if pageSize, err = strconv.Atoi(raw); err != nil || pageSize < 1 || pageSize > maxPageSize {
			driveError(w, http.StatusBadRequest, "invalid", "Invalid Value: pageSize")
			return
		}
	}
	start := 0
	if token := q.Get("pageToken"); token != "" {
		if start, err = strconv.Atoi(token); err != nil || start < 0 {
			driveError(w, http.StatusBadRequest, "invalid", "Invalid Value: pageToken")
			return
		}
	}
	descending := false
	switch q.Get("orderBy") {
	case "", "createdTime":
	case "createdTime desc":
		descending = true
	default:
		driveError(w, http.StatusBadRequest, "invalid", "Invalid Value: orderBy")
		return
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	var found []*file
	for _, f := range e.state.Files {
		// My Drive folders themselves are never listed.
		if len(f.Parents) > 0 && e.roleOfLocked(f, c.account) != "" && match(e, f, c.account) {
			found = append(found, f)
		}
	}
	slices.SortFunc(found, func(a, b *file) int {
		if order := a.CreatedTime.Compare(b.CreatedTime); order != 0 {
			if descending {
				return -order
			}
			return order
		}
		return strings.Compare(a.ID, b.ID)
	})
	files := []any{}
	for i := start; i < len(found) && i < start+pageSize; i++ {
		files = append(files, e.resourceLocked(found[i], c.account))
	}
	response := map[string]any{"kind": "drive#fileList", "incompleteSearch": false, "files": files}
	if start+pageSize < len(found) {
		response["nextPageToken"] = strconv.Itoa(start + pageSize)
	}
	writeJSON(w, http.StatusOK, selectFields(response, fields))
}

// getFile is files.get: a file's metadata, or with alt=media its content,
// in the byte range a Range header asks for.
func (e *Emulator) getFile(w http.ResponseWriter, r *http.Request, c *call) {
	media := r.URL.Query().Get("alt") == "media"
	c.name = "drive.files.get"
	if media {
		c.name = "drive.files.get.media"
	}
	fields, err := fieldsOf(r, defaultFileFields)
	if err != nil {
		driveError(w, http.StatusBadRequest, "invalidParameter", "Invalid field selection: "+err.Error())
		return
	}
	e.mu.Lock()
	f, _ := e.lookupLocked(w, r.PathValue("id"), c.account)
	if f == nil {
		e.mu.Unlock()
		return
	}
	if !media {
		resource := e.resourceLocked(f, c.account)
		e.mu.Unlock()
		writeJSON(w, http.StatusOK, selectFields(resource, fields))
		return
	}
	if f.MimeType == folderMimeType {
		e.mu.Unlock()
		driveError(w, http.StatusForbidden, "fileNotDownloadable", "Only files with binary content can be downloaded.")
		return
	}
	content, err := os.ReadFile(e.contentPath(f.ID))
	e.mu.Unlock()
	if err != nil {
		driveError(w, http.StatusInternalServerError, "backendError", err.Error())
		return
	}
	header := r.Header.Get("Range")
	if header == "" {
		w.Header().Set("Content-Length", strconv.Itoa(len(content)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(content)
		return
	}
	first, last, ok := parseRange(header, int64(len(content)))
	if !ok {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", len(content)))
		driveError(w, http.StatusRequestedRangeNotSatisfiable, "requestedRangeNotSatisfiable", "Request range not satisfiable")
		return
	}
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", first, last, len(content)))
	w.Header().Set("Content-Length", strconv.FormatInt(last-first+1, 10))
	w.WriteHeader(http.StatusPartialContent)
	_, _ = w.Write(content[first : last+1])
}

// parseRange parses a single "bytes=first-last" or "bytes=first-" range
// against a file of size bytes.
func parseRange(header string, size int64) (first, last int64, ok bool) {
	spec, found := strings.CutPrefix(header, "bytes=")
	from, to, dash := strings.Cut(spec, "-")
	if !found || !dash || strings.Contains(spec, ",") {
		return 0, 0, false
	}
	first, err := strconv.ParseInt(from, 10, 64)
	if err != nil || first < 0 || first >= size {
		return 0, 0, false
	}
	last = size - 1
	if to != "" {
		if last, err = strconv.ParseInt(to, 10, 64); err != nil || last < first {
			return 0, 0, false
		}
		last = min(last, size-1)
	}
	return first, last, true
}

// readUpload reads a request's metadata and content: JSON metadata alone,
// content alone with uploadType=media, or both with uploadType=multipart.
func readUpload(r *http.Request) (fileMetadata, []byte, bool, error) {
	var metadata fileMetadata
	upload := strings.HasPrefix(r.URL.Path, "/upload/")
	if !upload {
		if r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&metadata); err != nil && err != io.EOF {
				return metadata, nil, false, err
			}
		}
		return metadata, nil, false, nil
	}
	switch r.URL.Query().Get("uploadType") {
	case "media":
		content, err := io.ReadAll(r.Body)
		return metadata, content, true, err
	case "multipart":
		mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
			return metadata, nil, false, fmt.Errorf("multipart upload needs a multipart content type")
		}
		parts := multipart.NewReader(r.Body, params["boundary"])
		first, err := parts.NextPart()
		if err != nil {
			return metadata, nil, false, err
		}
		if err := json.NewDecoder(first).Decode(&metadata); err != nil {
			return metadata, nil, false, err
		}
		second, err := parts.NextPart()
		if err != nil {
			return metadata, nil, false, err
		}
		content, err := io.ReadAll(second)
		return metadata, content, true, err
	default:
		return metadata, nil, false, fmt.Errorf("unsupported uploadType %q", r.URL.Query().Get("uploadType"))
	}
}

// createFile is files.create: a folder or file owned by the caller, in a
// folder the caller can write to, or in its My Drive.
func (e *Emulator) createFile(w http.ResponseWriter, r *http.Request, c *call) {
	c.name = "drive.files.create"
	metadata, content, upload, err := readUpload(r)
	if upload {
		c.name = "drive.files.create.upload"
	}
	if err != nil {
		driveError(w, http.StatusBadRequest, "badContent", err.Error())
		return
	}
	fields, err := fieldsOf(r, defaultFileFields)
	if err != nil {
		driveError(w, http.StatusBadRequest, "invalidParameter", "Invalid field selection: "+err.Error())
		return
	}
	if len(metadata.Parents) > 1 {
		driveError(w, http.StatusForbidden, "cannotAddParent", "A shared drive item must have exactly one parent.")
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	parentID := e.state.Accounts[c.account].RootID
	if len(metadata.Parents) == 1 {
		parentID = metadata.Parents[0]
	}
	parent, role := e.lookupLocked(w, parentID, c.account)
	if parent == nil {
		return
	}
	if parent.MimeType != folderMimeType {
		driveError(w, http.StatusBadRequest, "invalidParent", "The parent is not a folder.")
		return
	}
	if rank(role) < rank(roleWriter) {
		driveError(w, http.StatusForbidden, "insufficientFilePermissions", "The user does not have sufficient permissions for this file.")
		return
	}
	now := e.options.Now()
	f := &file{ID: newID(), Name: "Untitled", MimeType: "application/octet-stream", Parents: []string{parent.ID}, Owner: c.account, CreatedTime: now, ModifiedTime: now}
	if metadata.Name != nil {
		f.Name = *metadata.Name
	}
	if metadata.MimeType != nil {
		f.MimeType = *metadata.MimeType
	}
	applyAppProperties(f, metadata.AppProperties)
	if f.MimeType == folderMimeType && upload {
		driveError(w, http.StatusBadRequest, "badContent", "A folder has no content.")
		return
	}
	if f.MimeType != folderMimeType {
		if err := writeAtomically(e.contentPath(f.ID), content); err != nil {
			driveError(w, http.StatusInternalServerError, "backendError", err.Error())
			return
		}
		f.Size = int64(len(content))
	}
	e.state.Files[f.ID] = f
	e.saveLocked()
	writeJSON(w, http.StatusOK, selectFields(e.resourceLocked(f, c.account), fields))
}

// updateFile is files.update: renaming, moving with addParents and
// removeParents, appProperties, trashing, and replacing the whole content.
// Changing a file needs the writer role; trashing or restoring it needs
// ownership.
func (e *Emulator) updateFile(w http.ResponseWriter, r *http.Request, c *call) {
	c.name = "drive.files.update"
	metadata, content, upload, err := readUpload(r)
	if upload {
		c.name = "drive.files.update.upload"
	}
	if err != nil {
		driveError(w, http.StatusBadRequest, "badContent", err.Error())
		return
	}
	fields, err := fieldsOf(r, defaultFileFields)
	if err != nil {
		driveError(w, http.StatusBadRequest, "invalidParameter", "Invalid field selection: "+err.Error())
		return
	}
	if metadata.Parents != nil {
		driveError(w, http.StatusForbidden, "parentsNotWritable", "The parents field is not directly writable in update requests. Use the addParents and removeParents parameters instead.")
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	f, role := e.lookupLocked(w, r.PathValue("id"), c.account)
	if f == nil {
		return
	}
	if rank(role) < rank(roleWriter) || metadata.Trashed != nil && role != roleOwner {
		driveError(w, http.StatusForbidden, "insufficientFilePermissions", "The user does not have sufficient permissions for this file.")
		return
	}
	query := r.URL.Query()
	parents := f.Parents
	if remove := query.Get("removeParents"); remove != "" {
		parents = slices.DeleteFunc(slices.Clone(parents), func(id string) bool { return slices.Contains(strings.Split(remove, ","), id) })
	}
	if add := query.Get("addParents"); add != "" {
		for _, id := range strings.Split(add, ",") {
			parent, parentRole := e.lookupLocked(w, id, c.account)
			if parent == nil {
				return
			}
			if parent.MimeType != folderMimeType || rank(parentRole) < rank(roleWriter) {
				driveError(w, http.StatusForbidden, "insufficientFilePermissions", "The user does not have sufficient permissions for the parent.")
				return
			}
			parents = append(parents, id)
		}
	}
	if len(parents) != 1 {
		driveError(w, http.StatusForbidden, "cannotAddParent", "A file must have exactly one parent.")
		return
	}
	if upload {
		if f.MimeType == folderMimeType {
			driveError(w, http.StatusBadRequest, "badContent", "A folder has no content.")
			return
		}
		if err := writeAtomically(e.contentPath(f.ID), content); err != nil {
			driveError(w, http.StatusInternalServerError, "backendError", err.Error())
			return
		}
		f.Size = int64(len(content))
	}
	f.Parents = parents
	if metadata.Name != nil {
		f.Name = *metadata.Name
	}
	if metadata.Trashed != nil {
		f.Trashed = *metadata.Trashed
	}
	applyAppProperties(f, metadata.AppProperties)
	f.ModifiedTime = maxTime(e.options.Now(), f.ModifiedTime)
	e.saveLocked()
	writeJSON(w, http.StatusOK, selectFields(e.resourceLocked(f, c.account), fields))
}

// applyAppProperties sets each given property, removing one given as null.
func applyAppProperties(f *file, properties map[string]*string) {
	for key, value := range properties {
		if value == nil {
			delete(f.AppProperties, key)
			continue
		}
		if f.AppProperties == nil {
			f.AppProperties = make(map[string]string)
		}
		f.AppProperties[key] = *value
	}
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
