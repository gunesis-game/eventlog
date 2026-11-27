// Command drive-ownership-check is a throwaway feasibility check, not part of
// the eventlog library. It answers one question: when a second Google account
// creates a file in a folder the owner shared with it, can that file's
// ownership be handed to the owner automatically through the Drive API, and
// what can the second account still do with the file afterwards?
//
// It runs in three steps, each with its own browser sign-in:
//
//	offer   (second account) creates a file in the folder and offers its
//	        ownership to the owner (pending owner).
//	accept  (owner) finds files in the folder with a pending ownership offer
//	        to it and accepts each one.
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
	scope    = "https://www.googleapis.com/auth/drive"
	driveAPI = "https://www.googleapis.com/drive/v3"
	upload   = "https://www.googleapis.com/upload/drive/v3"
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

type check struct {
	client      oauthClient
	step        string
	folderID    string
	owner       string
	fileID      string
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
	step := flag.String("step", "", "offer, accept, or verify")
	folderID := flag.String("folder", "", "ID of the folder the owner shared (offer and accept)")
	owner := flag.String("owner", "", "the owner's Google account email (offer and accept)")
	fileID := flag.String("file", "", "ID of the file the offer step created (verify)")
	port := flag.Int("port", 8765, "fixed local port the browser returns to after sign-in")
	flag.Parse()
	switch *step {
	case "offer", "accept":
		if *folderID == "" || *owner == "" {
			log.Fatalf("-step %s needs -folder and -owner; see README.md", *step)
		}
	case "verify":
		if *fileID == "" {
			log.Fatal("-step verify needs -file; see README.md")
		}
	default:
		log.Fatal("-step must be offer, accept, or verify; see README.md")
	}
	raw, err := os.ReadFile(*clientFile)
	if err != nil {
		log.Fatal(err)
	}
	c := &check{step: *step, folderID: *folderID, owner: strings.ToLower(*owner), fileID: *fileID, done: make(chan struct{})}
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
	if c.folderID != "" {
		// Drive calls folders files too, so a missing folder is reported
		// as "File not found". Look it up first to say so plainly.
		status, body := c.do(http.MethodGet, driveAPI+"/files/"+url.PathEscape(c.folderID)+"?fields=id,name,mimeType,owners(emailAddress),capabilities(canAddChildren)", nil, "")
		c.logf("0. Look up the shared folder -> HTTP %d %s", status, compact(body))
		if status != http.StatusOK {
			c.logf("   This account cannot see the folder: it is not shared with this account, or the ID is wrong.")
			return
		}
	}
	switch c.step {
	case "offer":
		c.offer()
	case "accept":
		c.accept()
	case "verify":
		c.verify()
	}
}

// offer creates a file in the shared folder and offers its ownership to the
// owner.
func (c *check) offer() {
	status, body := c.createFile()
	c.logf("1. Create a file in the folder -> HTTP %d %s", status, compact(body))
	var created struct {
		ID string `json:"id"`
	}
	if status != http.StatusOK || json.Unmarshal(body, &created) != nil {
		return
	}
	c.fileID = created.ID
	c.logf("2. Permissions before the offer -> %s", c.describePermissions())

	offer, _ := json.Marshal(map[string]any{"type": "user", "role": "writer", "emailAddress": c.owner, "pendingOwner": true})
	status, body = c.do(http.MethodPost, driveAPI+"/files/"+url.PathEscape(c.fileID)+"/permissions?fields=id,role,emailAddress,pendingOwner", offer, "application/json")
	c.logf("3. Offer ownership to %s (create permission, pendingOwner) -> HTTP %d %s", c.owner, status, compact(body))
	if status != http.StatusOK {
		if existing := c.findPermission(c.owner); existing != nil {
			update, _ := json.Marshal(map[string]any{"role": "writer", "pendingOwner": true})
			status, body = c.do(http.MethodPatch, driveAPI+"/files/"+url.PathEscape(c.fileID)+"/permissions/"+url.PathEscape(existing.ID)+"?fields=id,role,emailAddress,pendingOwner", update, "application/json")
			c.logf("   Retry by updating the owner's existing permission %s -> HTTP %d %s", existing.ID, status, compact(body))
		}
	}
	c.logf("4. Permissions after the offer -> %s", c.describePermissions())
	c.logf("File ID for the verify step: %s", c.fileID)
}

// accept finds files in the folder with a pending ownership offer to the
// signed-in account and accepts each one, as an owner's device would at
// start.
func (c *check) accept() {
	if c.email != c.owner {
		c.logf("Note: signed in as %s, not the owner %s; acceptance is expected to fail.", c.email, c.owner)
	}
	query := url.Values{
		"q":      {fmt.Sprintf("'%s' in parents and trashed = false", c.folderID)},
		"fields": {"files(id,name,owners(emailAddress))"},
	}
	status, body := c.do(http.MethodGet, driveAPI+"/files?"+query.Encode(), nil, "")
	c.logf("1. List the folder -> HTTP %d %s", status, compact(body))
	var listing struct {
		Files []struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Owners []struct {
				EmailAddress string `json:"emailAddress"`
			} `json:"owners"`
		} `json:"files"`
	}
	if status != http.StatusOK || json.Unmarshal(body, &listing) != nil {
		return
	}
	accepted := 0
	for _, file := range listing.Files {
		if len(file.Owners) == 0 || strings.EqualFold(file.Owners[0].EmailAddress, c.email) {
			continue
		}
		c.fileID = file.ID
		mine := c.findPermission(c.email)
		if mine == nil || !mine.PendingOwner {
			c.logf("2. %q owned by %s: no pending offer to me", file.Name, file.Owners[0].EmailAddress)
			continue
		}
		update, _ := json.Marshal(map[string]any{"role": "owner"})
		status, body = c.do(http.MethodPatch, driveAPI+"/files/"+url.PathEscape(file.ID)+"/permissions/"+url.PathEscape(mine.ID)+"?transferOwnership=true&fields=id,role,emailAddress", update, "application/json")
		c.logf("2. %q owned by %s: accept ownership -> HTTP %d %s", file.Name, file.Owners[0].EmailAddress, status, compact(body))
		c.logf("   Permissions after acceptance -> %s", c.describePermissions())
		accepted++
	}
	c.logf("Accepted %d offer(s).", accepted)
}

// verify reports what the second account can still do with the file after
// the ownership transfer.
func (c *check) verify() {
	status, body := c.do(http.MethodGet, driveAPI+"/files/"+url.PathEscape(c.fileID)+
		"?fields=name,owners(emailAddress),capabilities(canEdit,canModifyContent,canTrash,canDelete,canShare)", nil, "")
	c.logf("1. File owner and my capabilities -> HTTP %d %s", status, compact(body))

	content := []byte("updated by drive-ownership-check as " + c.email + " at " + time.Now().UTC().Format(time.RFC3339) + "\n")
	status, body = c.do(http.MethodPatch, upload+"/files/"+url.PathEscape(c.fileID)+"?uploadType=media&fields=id,size,owners(emailAddress)", content, "text/plain")
	c.logf("2. Update the file's content -> HTTP %d %s", status, compact(body))

	trash, _ := json.Marshal(map[string]any{"trashed": true})
	status, body = c.do(http.MethodPatch, driveAPI+"/files/"+url.PathEscape(c.fileID)+"?fields=id,trashed", trash, "application/json")
	c.logf("3. Move the file to the trash -> HTTP %d %s", status, compact(body))
	if status == http.StatusOK {
		restore, _ := json.Marshal(map[string]any{"trashed": false})
		status, body = c.do(http.MethodPatch, driveAPI+"/files/"+url.PathEscape(c.fileID)+"?fields=id,trashed", restore, "application/json")
		c.logf("   Restore it from the trash -> HTTP %d %s", status, compact(body))
	}
}

func (c *check) findPermission(email string) *permission {
	for _, p := range c.permissions() {
		if strings.EqualFold(p.EmailAddress, email) {
			found := p
			return &found
		}
	}
	return nil
}

func (c *check) permissions() []permission {
	status, body := c.do(http.MethodGet, driveAPI+"/files/"+url.PathEscape(c.fileID)+"/permissions?fields=permissions(id,type,role,emailAddress,pendingOwner)", nil, "")
	var list struct {
		Permissions []permission `json:"permissions"`
	}
	if status != http.StatusOK || json.Unmarshal(body, &list) != nil {
		return nil
	}
	return list.Permissions
}

func (c *check) describePermissions() string {
	status, body := c.do(http.MethodGet, driveAPI+"/files/"+url.PathEscape(c.fileID)+"?fields=owners(emailAddress),permissions(id,type,role,emailAddress,pendingOwner)", nil, "")
	return fmt.Sprintf("HTTP %d %s", status, compact(body))
}

func (c *check) createFile() (int, []byte) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	metadata, _ := json.Marshal(map[string]any{
		"name":    fmt.Sprintf("drive-ownership-check-%s.txt", time.Now().UTC().Format("20060102T150405Z")),
		"parents": []string{c.folderID},
	})
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
