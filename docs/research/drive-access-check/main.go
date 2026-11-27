// Command drive-access-check is a throwaway feasibility check, not part of the
// eventlog library. It answers one question: can a second Google account,
// signed in with only the non-sensitive drive.file permission, use a folder
// that another account owns and shared with it, after picking that folder once
// in Google Picker?
//
// It uses Google's desktop Picker flow: the ordinary OAuth sign-in URL with
// trigger_onepick=true, where the user picks the folder on the consent page and
// the redirect returns picked_file_ids with the authorization code. It then
// looks up the folder, lists it, reads another account's file in it, and
// creates a file of its own. See README.md. Standard library only.
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

// The desktop Picker permits only drive.file, and not combined with any other
// scope.
const scope = "https://www.googleapis.com/auth/drive.file"

type oauthClient struct {
	Installed struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	} `json:"installed"`
}

type tokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type check struct {
	client   oauthClient
	folderID string
	redirect string
	verifier string
	state    string
	tokens   tokens
	email    string
	report   strings.Builder
	done     chan struct{}
}

func main() {
	clientFile := flag.String("client", "oauth-client.json", "OAuth client JSON (Desktop app) downloaded from Google Cloud")
	folderID := flag.String("folder", "", "ID of the folder the owner shared")
	port := flag.Int("port", 8765, "fixed local port the browser returns to after sign-in")
	flag.Parse()
	if *folderID == "" {
		log.Fatal("-folder is required; see README.md")
	}
	raw, err := os.ReadFile(*clientFile)
	if err != nil {
		log.Fatal(err)
	}
	c := &check{folderID: *folderID, done: make(chan struct{})}
	if err := json.Unmarshal(raw, &c.client); err != nil || c.client.Installed.ClientID == "" {
		log.Fatal("the client file must be a Desktop app OAuth client (it has an \"installed\" section)")
	}
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		log.Fatal(err)
	}
	// Google accepts any loopback port for Desktop-app clients; return to the
	// root path, as its loopback examples do.
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
		"client_id":              {c.client.Installed.ClientID},
		"redirect_uri":           {c.redirect},
		"response_type":          {"code"},
		"scope":                  {scope},
		"code_challenge":         {base64.RawURLEncoding.EncodeToString(challenge[:])},
		"code_challenge_method":  {"S256"},
		"state":                  {c.state},
		"access_type":            {"offline"},
		"prompt":                 {"consent"},
		"trigger_onepick":        {"true"},
		"allow_folder_selection": {"true"},
	}.Encode()
	fmt.Println("Opening the browser for sign-in and Picker. If it does not open, visit:")
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
	query := r.URL.Query()
	if query.Get("state") != c.state {
		http.Error(w, "state mismatch", http.StatusBadRequest)
		return
	}
	if e := query.Get("error"); e != "" {
		c.logf("Sign-in or Picker failed: %s", e)
		fmt.Fprint(w, c.report.String())
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
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusOK || json.Unmarshal(body, &c.tokens) != nil {
		c.logf("Token exchange failed: %s", compact(body))
		fmt.Fprint(w, c.report.String())
		return
	}
	c.email = c.signedInEmail()
	picked := query.Get("picked_file_ids")
	c.logf("Signed in as: %s (refresh token issued: %t)", c.email, c.tokens.RefreshToken != "")
	c.logf("Step 1, picked IDs: %q (shared folder %s picked: %t)", picked, c.folderID, containsID(picked, c.folderID))
	c.logf("Step 2, folder lookup -> %s", c.describeFolder())
	c.logf("Step 3, list folder contents -> %s", c.listFolder())
	c.logf("Step 4, create a file of my own in the folder -> %s", c.createFile())
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprint(w, c.report.String())
}

func containsID(list, id string) bool {
	for _, candidate := range strings.Split(list, ",") {
		if strings.TrimSpace(candidate) == id {
			return true
		}
	}
	return false
}

func (c *check) driveGet(target string) (int, []byte) {
	request, _ := http.NewRequest(http.MethodGet, target, nil)
	request.Header.Set("Authorization", "Bearer "+c.tokens.AccessToken)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return 0, []byte(err.Error())
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	return response.StatusCode, body
}

// signedInEmail asks Drive who the token belongs to, since the desktop Picker
// flow cannot also request the email scope.
func (c *check) signedInEmail() string {
	status, body := c.driveGet("https://www.googleapis.com/drive/v3/about?fields=user(emailAddress)")
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

func (c *check) describeFolder() string {
	status, body := c.driveGet("https://www.googleapis.com/drive/v3/files/" + url.PathEscape(c.folderID) +
		"?fields=id,name,owners(emailAddress),capabilities(canAddChildren,canListChildren)&supportsAllDrives=true")
	return fmt.Sprintf("HTTP %d %s", status, compact(body))
}

func (c *check) listFolder() string {
	query := url.Values{
		"q":                 {fmt.Sprintf("'%s' in parents and trashed = false", c.folderID)},
		"fields":            {"files(id,name,owners(emailAddress))"},
		"supportsAllDrives": {"true"},
	}
	status, body := c.driveGet("https://www.googleapis.com/drive/v3/files?" + query.Encode())
	result := fmt.Sprintf("HTTP %d %s", status, compact(body))
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
		return result
	}
	for _, file := range listing.Files {
		if len(file.Owners) > 0 && file.Owners[0].EmailAddress != c.email {
			readStatus, content := c.driveGet("https://www.googleapis.com/drive/v3/files/" + url.PathEscape(file.ID) + "?alt=media")
			if len(content) > 120 {
				content = content[:120]
			}
			return result + fmt.Sprintf("\n  read %q owned by %s -> HTTP %d %q", file.Name, file.Owners[0].EmailAddress, readStatus, content)
		}
	}
	return result + "\n  (no file owned by another account was visible to read)"
}

func (c *check) createFile() string {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	metadata, _ := json.Marshal(map[string]any{
		"name":    fmt.Sprintf("drive-access-check-%s.txt", time.Now().UTC().Format("20060102T150405Z")),
		"parents": []string{c.folderID},
	})
	part, _ := writer.CreatePart(textproto.MIMEHeader{"Content-Type": {"application/json; charset=UTF-8"}})
	_, _ = part.Write(metadata)
	part, _ = writer.CreatePart(textproto.MIMEHeader{"Content-Type": {"text/plain"}})
	_, _ = part.Write([]byte("written by drive-access-check as " + c.email + "\n"))
	_ = writer.Close()
	request, _ := http.NewRequest(http.MethodPost,
		"https://www.googleapis.com/upload/drive/v3/files?uploadType=multipart&supportsAllDrives=true&fields=id,name,owners(emailAddress)", &body)
	request.Header.Set("Authorization", "Bearer "+c.tokens.AccessToken)
	request.Header.Set("Content-Type", "multipart/related; boundary="+writer.Boundary())
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err.Error()
	}
	defer response.Body.Close()
	reply, _ := io.ReadAll(response.Body)
	return fmt.Sprintf("HTTP %d %s", response.StatusCode, compact(reply))
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
