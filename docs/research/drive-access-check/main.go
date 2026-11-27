// Command drive-access-check is a throwaway feasibility check, not part of the
// eventlog library. It answers one question: can a second Google account,
// signed in with only the non-sensitive drive.file permission, use a folder
// that another account owns and shared with it, after picking that folder once
// in Google Picker?
//
// It signs in through a browser, tries the folder before and after picking it,
// lists it, reads another account's file in it, and creates a file of its own.
// See README.md for setup. It uses only the Go standard library.
package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
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

const scopes = "openid email https://www.googleapis.com/auth/drive.file"

type oauthClient struct {
	Installed struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	} `json:"installed"`
}

type tokens struct {
	AccessToken  string `json:"access_token"`
	IDToken      string `json:"id_token"`
	RefreshToken string `json:"refresh_token"`
}

type check struct {
	client   oauthClient
	apiKey   string
	appID    string
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
	apiKey := flag.String("apikey", "", "API key allowed to use the Google Picker API")
	appID := flag.String("appid", "", "Google Cloud project number")
	folderID := flag.String("folder", "", "ID of the folder the owner shared")
	port := flag.Int("port", 8765, "fixed local port the browser returns to after sign-in")
	flag.Parse()
	if *apiKey == "" || *appID == "" || *folderID == "" {
		log.Fatal("-apikey, -appid, and -folder are required; see README.md")
	}
	raw, err := os.ReadFile(*clientFile)
	if err != nil {
		log.Fatal(err)
	}
	c := &check{apiKey: *apiKey, appID: *appID, folderID: *folderID, done: make(chan struct{})}
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
	mux.HandleFunc("/picker", c.picker)
	mux.HandleFunc("/picked", c.picked)
	go func() {
		_ = http.Serve(listener, mux)
	}()

	challenge := sha256.Sum256([]byte(c.verifier))
	authURL := "https://accounts.google.com/o/oauth2/v2/auth?" + url.Values{
		"client_id":             {c.client.Installed.ClientID},
		"redirect_uri":          {c.redirect},
		"response_type":         {"code"},
		"scope":                 {scopes},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(challenge[:])},
		"code_challenge_method": {"S256"},
		"state":                 {c.state},
		"access_type":           {"offline"},
		"prompt":                {"consent select_account"},
	}.Encode()
	fmt.Println("Opening the browser for sign-in. If it does not open, visit:")
	fmt.Println(authURL)
	openBrowser(authURL)
	select {
	case <-c.done:
	case <-time.After(10 * time.Minute):
		log.Fatal("timed out after 10 minutes")
	}
	fmt.Println(c.report.String())
}

func (c *check) logf(format string, args ...any) {
	line := fmt.Sprintf(format, args...)
	c.report.WriteString(line + "\n")
	fmt.Println(line)
}

func (c *check) callback(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("state") != c.state {
		http.Error(w, "state mismatch", http.StatusBadRequest)
		return
	}
	if e := r.URL.Query().Get("error"); e != "" {
		c.logf("sign-in failed: %s", e)
		http.Error(w, "sign-in failed: "+e, http.StatusBadRequest)
		close(c.done)
		return
	}
	response, err := http.PostForm("https://oauth2.googleapis.com/token", url.Values{
		"client_id":     {c.client.Installed.ClientID},
		"client_secret": {c.client.Installed.ClientSecret},
		"code":          {r.URL.Query().Get("code")},
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
		http.Error(w, "token exchange failed: "+string(body), http.StatusBadGateway)
		return
	}
	c.email = emailFromIDToken(c.tokens.IDToken)
	c.logf("Signed in as: %s (refresh token issued: %t)", c.email, c.tokens.RefreshToken != "")
	c.logf("Step 1, before picking: folder lookup -> %s", c.describeFolder())
	http.Redirect(w, r, "/picker", http.StatusFound)
}

var pickerPage = template.Must(template.New("picker").Parse(`<!doctype html>
<html><head><meta charset="utf-8"><title>Pick the shared folder</title></head>
<body style="font-family:sans-serif;margin:2em">
<p>Pick the folder the owner shared with you, then wait for the result.</p>
<pre id="out"></pre>
<script src="https://apis.google.com/js/api.js"></script>
<script>
gapi.load('picker', function () {
  var view = new google.picker.DocsView(google.picker.ViewId.FOLDERS)
    .setIncludeFolders(true).setSelectFolderEnabled(true).setOwnedByMe(false);
  new google.picker.PickerBuilder()
    .addView(view)
    .setOAuthToken({{.Token}})
    .setDeveloperKey({{.APIKey}})
    .setAppId({{.AppID}})
    .setCallback(function (data) {
      if (data.action !== google.picker.Action.PICKED) { return; }
      fetch('/picked?id=' + encodeURIComponent(data.docs[0].id))
        .then(function (r) { return r.text(); })
        .then(function (t) { document.getElementById('out').textContent = t; });
    })
    .build().setVisible(true);
});
</script></body></html>`))

func (c *check) picker(w http.ResponseWriter, r *http.Request) {
	_ = pickerPage.Execute(w, map[string]string{"Token": c.tokens.AccessToken, "APIKey": c.apiKey, "AppID": c.appID})
}

func (c *check) picked(w http.ResponseWriter, r *http.Request) {
	picked := r.URL.Query().Get("id")
	c.logf("Picked folder ID: %s (expected %s, match: %t)", picked, c.folderID, picked == c.folderID)
	c.logf("Step 2, after picking: folder lookup -> %s", c.describeFolder())
	c.logf("Step 3, list folder contents -> %s", c.listFolder())
	c.logf("Step 4, create a file of my own in the folder -> %s", c.createFile())
	fmt.Fprint(w, c.report.String())
	close(c.done)
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

func emailFromIDToken(idToken string) string {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return "(unknown)"
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "(unknown)"
	}
	var claims struct {
		Email string `json:"email"`
	}
	_ = json.Unmarshal(payload, &claims)
	return claims.Email
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
