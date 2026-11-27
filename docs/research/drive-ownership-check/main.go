// Command drive-ownership-check is a throwaway feasibility check, not part of
// the eventlog library. It answers one question: when a second Google account
// creates a file in a folder the owner shared with it, can that file's
// ownership be handed to the owner automatically through the Drive API, and
// what can the second account still do with the file afterwards?
//
// The folder and the file are found, not passed in: the folder by an
// appProperties tag and the owner's ownership (oldest wins), as eventlog
// finds its folder, and the file by its fixed name in that folder. Each is
// created only when it does not exist yet. It runs in four steps, each with
// its own browser sign-in:
//
//	setup   (owner) finds or creates the folder and shares it with the
//	        second account as Editor.
//	offer   (second account) finds or creates the file in the folder and
//	        offers its ownership to the owner (pending owner).
//	accept  (owner) accepts every pending ownership offer in the folder.
//	verify  (second account) reports its access to the file, updates its
//	        content, and tries to move it to the trash, restoring it if that
//	        succeeds.
//
// See README.md. Standard library only.
package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const (
	scope      = "https://www.googleapis.com/auth/drive"
	driveAPI   = "https://www.googleapis.com/drive/v3"
	upload     = "https://www.googleapis.com/upload/drive/v3"
	folderName = "eventlog-ownership-check"
	folderTag  = "eventlogOwnershipCheck"
	fileName   = "ownership-check.txt"
	folderMIME = "application/vnd.google-apps.folder"
)

type oauthClient struct {
	Installed struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	} `json:"installed"`
}

type permission struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	Role         string `json:"role"`
	EmailAddress string `json:"emailAddress"`
	PendingOwner bool   `json:"pendingOwner"`
}

type driveFile struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Owners []struct {
		EmailAddress string `json:"emailAddress"`
	} `json:"owners"`
}

func (f driveFile) owner() string {
	if len(f.Owners) == 0 {
		return ""
	}
	return strings.ToLower(f.Owners[0].EmailAddress)
}

type check struct {
	client      oauthClient
	step        string
	owner       string
	member      string
	folderID    string
	redirect    string
	verifier    string
	state       string
	accessToken string
	email       string
	report      strings.Builder
	done        chan struct{}
}

func main() {
	clientFile := flag.String("client", "oauth-client.json", "OAuth client JSON (Desktop app) downloaded from Google Cloud")
	step := flag.String("step", "", "setup, offer, accept, or verify")
	owner := flag.String("owner", "", "the owner's Google account email")
	member := flag.String("member", "", "the second account's email, shared as Editor (setup)")
	port := flag.Int("port", 8765, "fixed local port the browser returns to after sign-in")
	flag.Parse()
	switch *step {
	case "setup":
		if *owner == "" || *member == "" {
			log.Fatal("-step setup needs -owner and -member; see README.md")
		}
	case "offer", "accept", "verify":
		if *owner == "" {
			log.Fatalf("-step %s needs -owner; see README.md", *step)
		}
	default:
		log.Fatal("-step must be setup, offer, accept, or verify; see README.md")
	}
	raw, err := os.ReadFile(*clientFile)
	if err != nil {
		log.Fatal(err)
	}
	c := &check{step: *step, owner: strings.ToLower(*owner), member: strings.ToLower(*member), done: make(chan struct{})}
	if err := json.Unmarshal(raw, &c.client); err != nil || c.client.Installed.ClientID == "" {
		log.Fatal("the client file must be a Desktop app OAuth client (it has an \"installed\" section)")
	}
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		log.Fatal(err)
	}
	c.redirect = "http://" + listener.Addr().String() + "/"
	c.verifier = randomString()
	c.state = randomString()

	mux := http.NewServeMux()
	mux.HandleFunc("/{$}", c.callback)
	go func() {
		_ = http.Serve(listener, mux)
	}()

	challenge := sha256.Sum256([]byte(c.verifier))
	authURL := "https://accounts.google.com/o/oauth2/v2/auth?" + url.Values{
		"client_id":             {c.client.Installed.ClientID},
		"redirect_uri":          {c.redirect},
		"response_type":         {"code"},
		"scope":                 {scope},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(challenge[:])},
		"code_challenge_method": {"S256"},
		"state":                 {c.state},
		"prompt":                {"select_account consent"},
	}.Encode()
	fmt.Println("Opening the browser for sign-in. If it does not open, visit:")
	fmt.Println(authURL)
	openBrowser(authURL)
	select {
	case <-c.done:
	case <-time.After(10 * time.Minute):
		log.Fatal("timed out after 10 minutes")
	}
}

func (c *check) logf(format string, args ...any) {
	line := fmt.Sprintf(format, args...)
	c.report.WriteString(line + "\n")
	fmt.Println(line)
}

func (c *check) callback(w http.ResponseWriter, r *http.Request) {
	defer close(c.done)
	defer func() {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprint(w, c.report.String())
	}()
	query := r.URL.Query()
	if query.Get("state") != c.state {
		c.logf("State mismatch; start again.")
		return
	}
	if e := query.Get("error"); e != "" {
		c.logf("Sign-in failed: %s", e)
		return
	}
	response, err := http.PostForm("https://oauth2.googleapis.com/token", url.Values{
		"client_id":     {c.client.Installed.ClientID},
		"client_secret": {c.client.Installed.ClientSecret},
		"code":          {query.Get("code")},
		"code_verifier": {c.verifier},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {c.redirect},
	})
	if err != nil {
		c.logf("Token exchange failed: %v", err)
		return
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	var tokens struct {
		AccessToken string `json:"access_token"`
		Scope       string `json:"scope"`
	}
	if response.StatusCode != http.StatusOK || json.Unmarshal(body, &tokens) != nil {
		c.logf("Token exchange failed: %s", compact(body))
		return
	}
	c.accessToken = tokens.AccessToken
	c.email = strings.ToLower(c.signedInEmail())
	c.logf("Step %q, signed in as: %s (granted: %s)", c.step, c.email, tokens.Scope)
	if !strings.Contains(" "+tokens.Scope+" ", " "+scope+" ") {
		c.logf("The Drive permission was not granted; sign in again and allow Drive access.")
		return
	}
	switch c.step {
	case "setup":
		c.setup()
	case "offer":
		c.offer()
	case "accept":
		c.accept()
	case "verify":
		c.verify()
	}
}

// findFolder finds the tagged folder owned by the owner, oldest first.
func (c *check) findFolder() bool {
	query := url.Values{
		"q": {fmt.Sprintf("appProperties has { key='%s' and value='root' } and '%s' in owners and mimeType = '%s' and trashed = false",
			folderTag, c.owner, folderMIME)},
		"orderBy": {"createdTime"},
		"fields":  {"files(id,name,owners(emailAddress))"},
	}
	status, body := c.do(http.MethodGet, driveAPI+"/files?"+query.Encode(), nil, "")
	var listing struct {
		Files []driveFile `json:"files"`
	}
	if status != http.StatusOK || json.Unmarshal(body, &listing) != nil {
		c.logf("Find the folder -> HTTP %d %s", status, compact(body))
		return false
	}
	if len(listing.Files) == 0 {
		c.logf("Find the folder -> none tagged and owned by %s", c.owner)
		return false
	}
	c.folderID = listing.Files[0].ID
	c.logf("Find the folder -> found %q (%s), %d tagged folder(s)", listing.Files[0].Name, c.folderID, len(listing.Files))
	return true
}

// findFile finds the check's file by its fixed name in the folder.
func (c *check) findFile() (driveFile, bool) {
	query := url.Values{
		"q":      {fmt.Sprintf("name = '%s' and '%s' in parents and trashed = false", fileName, c.folderID)},
		"fields": {"files(id,name,owners(emailAddress))"},
	}
	status, body := c.do(http.MethodGet, driveAPI+"/files?"+query.Encode(), nil, "")
	var listing struct {
		Files []driveFile `json:"files"`
	}
	if status != http.StatusOK || json.Unmarshal(body, &listing) != nil {
		c.logf("Find the file -> HTTP %d %s", status, compact(body))
		return driveFile{}, false
	}
	if len(listing.Files) == 0 {
		return driveFile{}, false
	}
	return listing.Files[0], true
}

// setup finds or creates the folder as the owner and shares it with the
// second account as Editor.
func (c *check) setup() {
	if c.email != c.owner {
		c.logf("Sign in as the owner %s for setup; only the owner may create the folder.", c.owner)
		return
	}
	if !c.findFolder() {
		metadata, _ := json.Marshal(map[string]any{
			"name":          folderName,
			"mimeType":      folderMIME,
			"appProperties": map[string]string{folderTag: "root"},
		})
		status, body := c.do(http.MethodPost, driveAPI+"/files?fields=id,name,owners(emailAddress)", metadata, "application/json")
		c.logf("Create the folder -> HTTP %d %s", status, compact(body))
		var created driveFile
		if status != http.StatusOK || json.Unmarshal(body, &created) != nil {
			return
		}
		c.folderID = created.ID
	}
	if existing := c.findPermission(c.folderID, c.member); existing != nil {
		c.logf("Share with %s -> already shared as %s", c.member, existing.Role)
	} else {
		share, _ := json.Marshal(map[string]any{"type": "user", "role": "writer", "emailAddress": c.member})
		status, body := c.do(http.MethodPost, driveAPI+"/files/"+url.PathEscape(c.folderID)+"/permissions?sendNotificationEmail=false&fields=id,role,emailAddress", share, "application/json")
		c.logf("Share with %s as Editor -> HTTP %d %s", c.member, status, compact(body))
	}
	c.logf("Folder permissions -> %s", c.describePermissions(c.folderID))
}

// offer finds or creates the file as the second account and offers its
// ownership to the owner.
func (c *check) offer() {
	if !c.findFolder() {
		c.logf("Run the setup step as the owner first, or check the folder is shared with %s.", c.email)
		return
	}
	file, found := c.findFile()
	if found {
		c.logf("Find the file -> found %q (%s), owned by %s", file.Name, file.ID, file.owner())
	} else {
		status, body := c.createFile()
		c.logf("Create the file -> HTTP %d %s", status, compact(body))
		if status != http.StatusOK || json.Unmarshal(body, &file) != nil {
			return
		}
	}
	if file.owner() == c.owner {
		c.logf("The file is already owned by the owner; nothing to offer.")
		return
	}
	c.logf("Permissions before the offer -> %s", c.describePermissions(file.ID))
	if existing := c.findPermission(file.ID, c.owner); existing != nil && existing.PendingOwner {
		c.logf("Offer ownership to %s -> already offered", c.owner)
		return
	}
	offer, _ := json.Marshal(map[string]any{"type": "user", "role": "writer", "emailAddress": c.owner, "pendingOwner": true})
	status, body := c.do(http.MethodPost, driveAPI+"/files/"+url.PathEscape(file.ID)+"/permissions?fields=id,role,emailAddress,pendingOwner", offer, "application/json")
	c.logf("Offer ownership to %s (create permission, pendingOwner) -> HTTP %d %s", c.owner, status, compact(body))
	// The owner usually already has a writer permission inherited from the
	// folder; creating one then returns it unchanged, without pendingOwner.
	// Set pendingOwner on that permission instead.
	if existing := c.findPermission(file.ID, c.owner); existing == nil || !existing.PendingOwner {
		if existing != nil {
			update, _ := json.Marshal(map[string]any{"role": "writer", "pendingOwner": true})
			status, body = c.do(http.MethodPatch, driveAPI+"/files/"+url.PathEscape(file.ID)+"/permissions/"+url.PathEscape(existing.ID)+"?fields=id,role,emailAddress,pendingOwner", update, "application/json")
			c.logf("  Retry by updating the owner's existing permission %s -> HTTP %d %s", existing.ID, status, compact(body))
		}
	}
	c.logf("Permissions after the offer -> %s", c.describePermissions(file.ID))
}

// accept accepts every pending ownership offer to the owner in the folder, as
// an owner's device would at start.
func (c *check) accept() {
	if c.email != c.owner {
		c.logf("Sign in as the owner %s to accept.", c.owner)
		return
	}
	if !c.findFolder() {
		return
	}
	query := url.Values{
		"q":      {fmt.Sprintf("'%s' in parents and trashed = false", c.folderID)},
		"fields": {"files(id,name,owners(emailAddress))"},
	}
	status, body := c.do(http.MethodGet, driveAPI+"/files?"+query.Encode(), nil, "")
	var listing struct {
		Files []driveFile `json:"files"`
	}
	if status != http.StatusOK || json.Unmarshal(body, &listing) != nil {
		c.logf("List the folder -> HTTP %d %s", status, compact(body))
		return
	}
	accepted := 0
	for _, file := range listing.Files {
		if file.owner() == c.email {
			c.logf("%q is already owned by me", file.Name)
			continue
		}
		mine := c.findPermission(file.ID, c.email)
		if mine == nil || !mine.PendingOwner {
			c.logf("%q owned by %s: no pending offer to me", file.Name, file.owner())
			continue
		}
		update, _ := json.Marshal(map[string]any{"role": "owner"})
		status, body = c.do(http.MethodPatch, driveAPI+"/files/"+url.PathEscape(file.ID)+"/permissions/"+url.PathEscape(mine.ID)+"?transferOwnership=true&fields=id,role,emailAddress", update, "application/json")
		c.logf("%q owned by %s: accept ownership -> HTTP %d %s", file.Name, file.owner(), status, compact(body))
		c.logf("  Permissions after acceptance -> %s", c.describePermissions(file.ID))
		accepted++
	}
	c.logf("Accepted %d offer(s).", accepted)
}

// verify reports what the second account can still do with the file after
// the ownership transfer.
func (c *check) verify() {
	if !c.findFolder() {
		return
	}
	file, found := c.findFile()
	if !found {
		c.logf("Find the file -> not found; run the offer step first.")
		return
	}
	status, body := c.do(http.MethodGet, driveAPI+"/files/"+url.PathEscape(file.ID)+
		"?fields=name,owners(emailAddress),capabilities(canEdit,canModifyContent,canTrash,canDelete,canShare)", nil, "")
	c.logf("1. File owner and my capabilities -> HTTP %d %s", status, compact(body))

	content := []byte("updated by drive-ownership-check as " + c.email + " at " + time.Now().UTC().Format(time.RFC3339) + "\n")
	status, body = c.do(http.MethodPatch, upload+"/files/"+url.PathEscape(file.ID)+"?uploadType=media&fields=id,size,owners(emailAddress)", content, "text/plain")
	c.logf("2. Update the file's content -> HTTP %d %s", status, compact(body))

	trash, _ := json.Marshal(map[string]any{"trashed": true})
	status, body = c.do(http.MethodPatch, driveAPI+"/files/"+url.PathEscape(file.ID)+"?fields=id,trashed", trash, "application/json")
	c.logf("3. Move the file to the trash -> HTTP %d %s", status, compact(body))
	if status == http.StatusOK {
		restore, _ := json.Marshal(map[string]any{"trashed": false})
		status, body = c.do(http.MethodPatch, driveAPI+"/files/"+url.PathEscape(file.ID)+"?fields=id,trashed", restore, "application/json")
		c.logf("   Restore it from the trash -> HTTP %d %s", status, compact(body))
	}
}

func (c *check) findPermission(fileID, email string) *permission {
	status, body := c.do(http.MethodGet, driveAPI+"/files/"+url.PathEscape(fileID)+"/permissions?fields=permissions(id,type,role,emailAddress,pendingOwner)", nil, "")
	var list struct {
		Permissions []permission `json:"permissions"`
	}
	if status != http.StatusOK || json.Unmarshal(body, &list) != nil {
		return nil
	}
	for _, p := range list.Permissions {
		if strings.EqualFold(p.EmailAddress, email) {
			found := p
			return &found
		}
	}
	return nil
}

func (c *check) describePermissions(fileID string) string {
	status, body := c.do(http.MethodGet, driveAPI+"/files/"+url.PathEscape(fileID)+"?fields=owners(emailAddress),permissions(id,type,role,emailAddress,pendingOwner)", nil, "")
	return fmt.Sprintf("HTTP %d %s", status, compact(body))
}

func (c *check) createFile() (int, []byte) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	metadata, _ := json.Marshal(map[string]any{"name": fileName, "parents": []string{c.folderID}})
	part, _ := writer.CreatePart(textproto.MIMEHeader{"Content-Type": {"application/json; charset=UTF-8"}})
	_, _ = part.Write(metadata)
	part, _ = writer.CreatePart(textproto.MIMEHeader{"Content-Type": {"text/plain"}})
	_, _ = part.Write([]byte("written by drive-ownership-check as " + c.email + "\n"))
	_ = writer.Close()
	return c.do(http.MethodPost, upload+"/files?uploadType=multipart&fields=id,name,owners(emailAddress)", body.Bytes(), "multipart/related; boundary="+writer.Boundary())
}

func (c *check) signedInEmail() string {
	status, body := c.do(http.MethodGet, driveAPI+"/about?fields=user(emailAddress)", nil, "")
	var about struct {
		User struct {
			EmailAddress string `json:"emailAddress"`
		} `json:"user"`
	}
	if status != http.StatusOK || json.Unmarshal(body, &about) != nil {
		return fmt.Sprintf("(unknown: HTTP %d %s)", status, compact(body))
	}
	return about.User.EmailAddress
}

func (c *check) do(method, target string, payload []byte, contentType string) (int, []byte) {
	var reader io.Reader
	if payload != nil {
		reader = bytes.NewReader(payload)
	}
	request, err := http.NewRequest(method, target, reader)
	if err != nil {
		return 0, []byte(err.Error())
	}
	request.Header.Set("Authorization", "Bearer "+c.accessToken)
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return 0, []byte(err.Error())
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	return response.StatusCode, body
}

func compact(body []byte) string {
	var out bytes.Buffer
	if json.Compact(&out, body) == nil {
		return out.String()
	}
	return strings.TrimSpace(string(body))
}

func randomString() string {
	buffer := make([]byte, 32)
	_, _ = rand.Read(buffer)
	return base64.RawURLEncoding.EncodeToString(buffer)
}

func openBrowser(target string) {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	case "darwin":
		command = exec.Command("open", target)
	default:
		command = exec.Command("xdg-open", target)
	}
	_ = command.Start()
}
