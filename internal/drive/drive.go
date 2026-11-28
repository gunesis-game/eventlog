// Package drive is a client of the Google Drive API v3, written from Google's
// reference (developers.google.com/workspace/drive/api/reference/rest/v3):
// files.list, files.get with alt=media and Range, files.create and
// files.update with metadata, multipart, and media uploads, and the
// permissions calls. It knows nothing of eventlog's storage. Its base
// address is Google's unless given another, as tests give the emulator's.
package drive

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DefaultBaseURL is Google's address for the Drive API.
const DefaultBaseURL = "https://www.googleapis.com"

// FolderMimeType is the MIME type of a Drive folder.
const FolderMimeType = "application/vnd.google-apps.folder"

// MaxSimpleUpload is the largest content a multipart or media upload may
// carry; larger content needs a resumable upload.
const MaxSimpleUpload = 5 << 20

// The fields the client asks for and reads; Drive returns only the fields
// a request names.
const (
	fileFields       = "id,name,mimeType,parents,size,trashed,createdTime,modifiedTime,appProperties,owners(emailAddress,permissionId),capabilities(canModifyContent,canShare,canTrash,canDelete)"
	permissionFields = "id,type,emailAddress,role,pendingOwner,permissionDetails(inherited)"
)

// TokenSource returns a valid access token for the signed-in account.
type TokenSource func(ctx context.Context) (string, error)

// Client calls the Drive API as one signed-in account.
type Client struct {
	http  *http.Client
	base  string
	token TokenSource
}

// New returns a client calling baseURL (DefaultBaseURL when empty) with h,
// authorized by token.
func New(h *http.Client, baseURL string, token TokenSource) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{http: h, base: strings.TrimSuffix(baseURL, "/"), token: token}
}

// File is a Drive file or folder's metadata.
type File struct {
	ID            string
	Name          string
	MimeType      string
	Parents       []string
	Size          int64
	Trashed       bool
	CreatedTime   time.Time
	ModifiedTime  time.Time
	AppProperties map[string]string
	Owners        []User
	Capabilities  Capabilities
}

// User is a file's owner.
type User struct {
	EmailAddress string `json:"emailAddress"`
	PermissionID string `json:"permissionId"`
}

// Capabilities is what the signed-in account may do with a file.
type Capabilities struct {
	CanModifyContent bool `json:"canModifyContent"`
	CanShare         bool `json:"canShare"`
	CanTrash         bool `json:"canTrash"`
	CanDelete        bool `json:"canDelete"`
}

// IsFolder reports whether f is a folder.
func (f File) IsFolder() bool {
	return f.MimeType == FolderMimeType
}

// fileResource is the JSON form of a files resource; Drive sends int64
// values such as size as strings.
type fileResource struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	MimeType      string            `json:"mimeType"`
	Parents       []string          `json:"parents"`
	Size          string            `json:"size"`
	Trashed       bool              `json:"trashed"`
	CreatedTime   time.Time         `json:"createdTime"`
	ModifiedTime  time.Time         `json:"modifiedTime"`
	AppProperties map[string]string `json:"appProperties"`
	Owners        []User            `json:"owners"`
	Capabilities  Capabilities      `json:"capabilities"`
}

func (r fileResource) file() (File, error) {
	f := File{ID: r.ID, Name: r.Name, MimeType: r.MimeType, Parents: r.Parents, Trashed: r.Trashed, CreatedTime: r.CreatedTime, ModifiedTime: r.ModifiedTime, AppProperties: r.AppProperties, Owners: r.Owners, Capabilities: r.Capabilities}
	if r.Size != "" {
		size, err := strconv.ParseInt(r.Size, 10, 64)
		if err != nil {
			return File{}, fmt.Errorf("drive: file %s size %q: %w", r.ID, r.Size, err)
		}
		f.Size = size
	}
	return f, nil
}

// Error is an error response of the Drive API.
type Error struct {
	Status  int
	Reason  string
	Message string
}

func (e *Error) Error() string {
	return fmt.Sprintf("drive: %d %s: %s", e.Status, e.Reason, e.Message)
}

// Error reasons the Drive API documents.
const (
	ReasonNotFound                    = "notFound"
	ReasonInsufficientFilePermissions = "insufficientFilePermissions"
	ReasonAuthError                   = "authError"
)

// HasReason reports whether err is a Drive error with the given reason.
func HasReason(err error, reason string) bool {
	var driveErr *Error
	return errors.As(err, &driveErr) && driveErr.Reason == reason
}

// do sends a request with the access token and returns the response body,
// or the Drive error the response carries.
func (c *Client) do(ctx context.Context, method, path string, query url.Values, contentType string, body []byte, header http.Header) ([]byte, http.Header, error) {
	token, err := c.token(ctx)
	if err != nil {
		return nil, nil, err
	}
	address := c.base + path
	if len(query) > 0 {
		address += "?" + query.Encode()
	}
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	request, err := http.NewRequestWithContext(ctx, method, address, reader)
	if err != nil {
		return nil, nil, err
	}
	for key, values := range header {
		request.Header[key] = values
	}
	request.Header.Set("Authorization", "Bearer "+token)
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	response, err := c.http.Do(request)
	if err != nil {
		return nil, nil, err
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, nil, err
	}
	if response.StatusCode >= 300 {
		return nil, nil, decodeError(response.StatusCode, raw)
	}
	return raw, response.Header, nil
}

func decodeError(status int, raw []byte) error {
	var body struct {
		Error struct {
			Message string `json:"message"`
			Errors  []struct {
				Reason string `json:"reason"`
			} `json:"errors"`
		} `json:"error"`
	}
	_ = json.Unmarshal(raw, &body)
	e := &Error{Status: status, Message: body.Error.Message}
	if len(body.Error.Errors) > 0 {
		e.Reason = body.Error.Errors[0].Reason
	}
	return e
}

func (c *Client) fileCall(ctx context.Context, method, path string, query url.Values, contentType string, body []byte) (File, error) {
	if query == nil {
		query = url.Values{}
	}
	query.Set("fields", fileFields)
	raw, _, err := c.do(ctx, method, path, query, contentType, body, nil)
	if err != nil {
		return File{}, err
	}
	var resource fileResource
	if err := json.Unmarshal(raw, &resource); err != nil {
		return File{}, fmt.Errorf("drive: %w", err)
	}
	return resource.file()
}

// List returns every file matching a query in Drive's search syntax, in
// orderBy order (Drive's when empty), following pages to the end.
func (c *Client) List(ctx context.Context, q, orderBy string) ([]File, error) {
	var files []File
	query := url.Values{"q": {q}, "fields": {"nextPageToken,files(" + fileFields + ")"}, "pageSize": {"1000"}}
	if orderBy != "" {
		query.Set("orderBy", orderBy)
	}
	for {
		raw, _, err := c.do(ctx, http.MethodGet, "/drive/v3/files", query, "", nil, nil)
		if err != nil {
			return nil, err
		}
		var page struct {
			NextPageToken string         `json:"nextPageToken"`
			Files         []fileResource `json:"files"`
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, fmt.Errorf("drive: %w", err)
		}
		for _, resource := range page.Files {
			f, err := resource.file()
			if err != nil {
				return nil, err
			}
			files = append(files, f)
		}
		if page.NextPageToken == "" {
			return files, nil
		}
		query.Set("pageToken", page.NextPageToken)
	}
}

// Get returns a file's metadata; "root" names My Drive.
func (c *Client) Get(ctx context.Context, id string) (File, error) {
	return c.fileCall(ctx, http.MethodGet, "/drive/v3/files/"+url.PathEscape(id), nil, "", nil)
}

// Download returns length bytes of a file's content from offset, or all of
// it from offset when length is negative, with a partial download through a
// Range header.
func (c *Client) Download(ctx context.Context, id string, offset, length int64) ([]byte, error) {
	header := http.Header{}
	switch {
	case length == 0:
		return []byte{}, nil
	case length > 0:
		header.Set("Range", fmt.Sprintf("bytes=%d-%d", offset, offset+length-1))
	case offset > 0:
		header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}
	raw, _, err := c.do(ctx, http.MethodGet, "/drive/v3/files/"+url.PathEscape(id), url.Values{"alt": {"media"}}, "", nil, header)
	return raw, err
}

// newFile is the metadata of a file or folder being created.
type newFile struct {
	Name          string            `json:"name"`
	MimeType      string            `json:"mimeType,omitempty"`
	Parents       []string          `json:"parents"`
	AppProperties map[string]string `json:"appProperties,omitempty"`
}

// CreateFolder creates a folder in parent, with appProperties.
func (c *Client) CreateFolder(ctx context.Context, name, parent string, appProperties map[string]string) (File, error) {
	body, err := json.Marshal(newFile{Name: name, MimeType: FolderMimeType, Parents: []string{parent}, AppProperties: appProperties})
	if err != nil {
		return File{}, err
	}
	return c.fileCall(ctx, http.MethodPost, "/drive/v3/files", nil, "application/json; charset=UTF-8", body)
}

// ErrTooLarge reports content beyond MaxSimpleUpload.
var ErrTooLarge = fmt.Errorf("drive: content over %d bytes needs a resumable upload", MaxSimpleUpload)

// Create creates a file with content in parent, with one multipart upload:
// the JSON metadata part first, then the media part.
func (c *Client) Create(ctx context.Context, name, parent string, content []byte) (File, error) {
	if len(content) > MaxSimpleUpload {
		return File{}, ErrTooLarge
	}
	metadata, err := json.Marshal(newFile{Name: name, Parents: []string{parent}})
	if err != nil {
		return File{}, err
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreatePart(textproto.MIMEHeader{"Content-Type": {"application/json; charset=UTF-8"}})
	if err != nil {
		return File{}, err
	}
	_, _ = part.Write(metadata)
	if part, err = writer.CreatePart(textproto.MIMEHeader{"Content-Type": {"application/octet-stream"}}); err != nil {
		return File{}, err
	}
	_, _ = part.Write(content)
	if err := writer.Close(); err != nil {
		return File{}, err
	}
	return c.fileCall(ctx, http.MethodPost, "/upload/drive/v3/files", url.Values{"uploadType": {"multipart"}}, "multipart/related; boundary="+writer.Boundary(), body.Bytes())
}

// UpdateContent replaces all of a file's content with one media upload.
func (c *Client) UpdateContent(ctx context.Context, id string, content []byte) (File, error) {
	if len(content) > MaxSimpleUpload {
		return File{}, ErrTooLarge
	}
	return c.fileCall(ctx, http.MethodPatch, "/upload/drive/v3/files/"+url.PathEscape(id), url.Values{"uploadType": {"media"}}, "application/octet-stream", content)
}

// Changes are metadata changes of files.update; only what is set changes.
type Changes struct {
	Name    *string
	Trashed *bool
	// AppProperties sets each property, and removes one given as nil.
	AppProperties map[string]*string
	AddParents    []string
	RemoveParents []string
}

// Update changes a file's metadata: renaming, moving between folders,
// trashing or restoring, and its appProperties.
func (c *Client) Update(ctx context.Context, id string, changes Changes) (File, error) {
	patch := map[string]any{}
	if changes.Name != nil {
		patch["name"] = *changes.Name
	}
	if changes.Trashed != nil {
		patch["trashed"] = *changes.Trashed
	}
	if changes.AppProperties != nil {
		patch["appProperties"] = changes.AppProperties
	}
	body, err := json.Marshal(patch)
	if err != nil {
		return File{}, err
	}
	query := url.Values{}
	if len(changes.AddParents) > 0 {
		query.Set("addParents", strings.Join(changes.AddParents, ","))
	}
	if len(changes.RemoveParents) > 0 {
		query.Set("removeParents", strings.Join(changes.RemoveParents, ","))
	}
	return c.fileCall(ctx, http.MethodPatch, "/drive/v3/files/"+url.PathEscape(id), query, "application/json; charset=UTF-8", body)
}

// Permission is one account's permission on a file.
type Permission struct {
	ID           string
	Type         string
	EmailAddress string
	Role         string
	PendingOwner bool
	// Inherited reports that the permission comes from a folder above.
	Inherited bool
}

type permissionResource struct {
	ID                string `json:"id"`
	Type              string `json:"type"`
	EmailAddress      string `json:"emailAddress"`
	Role              string `json:"role"`
	PendingOwner      bool   `json:"pendingOwner"`
	PermissionDetails []struct {
		Inherited bool `json:"inherited"`
	} `json:"permissionDetails"`
}

func (r permissionResource) permission() Permission {
	p := Permission{ID: r.ID, Type: r.Type, EmailAddress: r.EmailAddress, Role: r.Role, PendingOwner: r.PendingOwner}
	for _, detail := range r.PermissionDetails {
		p.Inherited = p.Inherited || detail.Inherited
	}
	return p
}

func (c *Client) permissionCall(ctx context.Context, method, path string, query url.Values, body any) (Permission, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return Permission{}, err
	}
	if query == nil {
		query = url.Values{}
	}
	query.Set("fields", permissionFields)
	response, _, err := c.do(ctx, method, path, query, "application/json; charset=UTF-8", raw, nil)
	if err != nil {
		return Permission{}, err
	}
	var resource permissionResource
	if err := json.Unmarshal(response, &resource); err != nil {
		return Permission{}, fmt.Errorf("drive: %w", err)
	}
	return resource.permission(), nil
}

// Permissions lists every permission on a file, inherited ones included.
func (c *Client) Permissions(ctx context.Context, id string) ([]Permission, error) {
	query := url.Values{"fields": {"nextPageToken,permissions(" + permissionFields + ")"}}
	var permissions []Permission
	for {
		raw, _, err := c.do(ctx, http.MethodGet, "/drive/v3/files/"+url.PathEscape(id)+"/permissions", query, "", nil, nil)
		if err != nil {
			return nil, err
		}
		var page struct {
			NextPageToken string               `json:"nextPageToken"`
			Permissions   []permissionResource `json:"permissions"`
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, fmt.Errorf("drive: %w", err)
		}
		for _, resource := range page.Permissions {
			permissions = append(permissions, resource.permission())
		}
		if page.NextPageToken == "" {
			return permissions, nil
		}
		query.Set("pageToken", page.NextPageToken)
	}
}

// Share gives a user a role ("reader" or "writer") on a file.
func (c *Client) Share(ctx context.Context, id, email, role string) (Permission, error) {
	return c.permissionCall(ctx, http.MethodPost, "/drive/v3/files/"+url.PathEscape(id)+"/permissions", nil, map[string]string{"type": "user", "role": role, "emailAddress": email})
}

// OfferOwnership offers ownership of a file to the account holding a
// permission, by setting pendingOwner on that permission.
func (c *Client) OfferOwnership(ctx context.Context, id, permissionID string) (Permission, error) {
	return c.permissionCall(ctx, http.MethodPatch, "/drive/v3/files/"+url.PathEscape(id)+"/permissions/"+url.PathEscape(permissionID), nil, map[string]bool{"pendingOwner": true})
}

// AcceptOwnership accepts an ownership offer of a file: the pending owner
// updates its own permission to role owner, acknowledging the transfer.
func (c *Client) AcceptOwnership(ctx context.Context, id, permissionID string) (Permission, error) {
	return c.permissionCall(ctx, http.MethodPatch, "/drive/v3/files/"+url.PathEscape(id)+"/permissions/"+url.PathEscape(permissionID), url.Values{"transferOwnership": {"true"}}, map[string]string{"role": "owner"})
}
