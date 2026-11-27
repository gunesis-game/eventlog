package filesystem

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Client performs file operations on a Server's storage root. Its errors
// match the Backend contract: fs.ErrNotExist, fs.ErrExist, and ErrLocked.
type Client struct {
	base  string
	token string
	http  *http.Client
}

// NewClient returns a client for the server at base, presenting token.
func NewClient(base, token string) *Client {
	// Several store goroutines issue requests concurrently; keep enough idle
	// loopback connections that they are reused rather than reopened.
	transport := &http.Transport{MaxIdleConns: 64, MaxIdleConnsPerHost: 64}
	return &Client{base: base, token: token, http: &http.Client{Transport: transport}}
}

// do sends one request and returns the response body of a 2xx reply.
func (c *Client) do(method, route, p string, query url.Values, header http.Header, body []byte) ([]byte, error) {
	if _, err := cleanPath(p); err != nil {
		return nil, err
	}
	target := c.base + route + (&url.URL{Path: p}).EscapedPath()
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	request, err := http.NewRequest(method, target, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	for name, values := range header {
		request.Header[name] = values
	}
	request.Header.Set("Authorization", "Bearer "+c.token)
	response, err := c.http.Do(request)
	if err != nil {
		return nil, fmt.Errorf("file-system request %s %s: %w", method, p, err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("file-system response %s %s: %w", method, p, err)
	}
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return data, nil
	}
	message := strings.TrimSpace(string(data))
	switch response.StatusCode {
	case http.StatusNotFound:
		return nil, fmt.Errorf("%s %s: %s: %w", method, p, message, fs.ErrNotExist)
	case http.StatusPreconditionFailed:
		return nil, fmt.Errorf("%s %s: %s: %w", method, p, message, fs.ErrExist)
	case http.StatusConflict:
		return nil, fmt.Errorf("%s %s: %s: %w", method, p, message, ErrLocked)
	default:
		return nil, fmt.Errorf("%s %s: %s (%d)", method, p, message, response.StatusCode)
	}
}

func (c *Client) MakeDirectory(p string) error {
	_, err := c.do(http.MethodPut, "/dirs/", p, nil, nil, nil)
	return err
}

func (c *Client) CreateExclusive(p string) error {
	_, err := c.do(http.MethodPut, "/files/", p, nil, http.Header{"If-None-Match": {"*"}}, nil)
	return err
}

// Read returns length bytes at offset, or the rest of the file when length
// is negative.
func (c *Client) Read(p string, offset, length int64) ([]byte, error) {
	var header http.Header
	switch {
	case length > 0:
		header = http.Header{"Range": {fmt.Sprintf("bytes=%d-%d", offset, offset+length-1)}}
	case length == 0:
		return []byte{}, nil
	case offset > 0:
		header = http.Header{"Range": {fmt.Sprintf("bytes=%d-", offset)}}
	}
	return c.do(http.MethodGet, "/files/", p, nil, header, nil)
}

func (c *Client) Append(p string, data []byte) (int64, error) {
	body, err := c.do(http.MethodPost, "/files/", p, url.Values{"op": {"append"}}, nil, data)
	if err != nil {
		return 0, err
	}
	var reply struct {
		Size int64 `json:"size"`
	}
	if err := json.Unmarshal(body, &reply); err != nil {
		return 0, fmt.Errorf("decode append reply: %w", err)
	}
	return reply.Size, nil
}

func (c *Client) Truncate(p string, size int64) error {
	_, err := c.do(http.MethodPost, "/files/", p, url.Values{"op": {"truncate"}, "size": {strconv.FormatInt(size, 10)}}, nil, nil)
	return err
}

func (c *Client) Sync(p string) error {
	_, err := c.do(http.MethodPost, "/files/", p, url.Values{"op": {"sync"}}, nil, nil)
	return err
}

func (c *Client) Replace(p string, data []byte) error {
	_, err := c.do(http.MethodPut, "/files/", p, nil, nil, data)
	return err
}

func (c *Client) Rename(from, to string) error {
	_, err := c.do(http.MethodPost, "/files/", from, url.Values{"op": {"rename"}, "to": {to}}, nil, nil)
	return err
}

func (c *Client) Remove(p string) error {
	_, err := c.do(http.MethodDelete, "/files/", p, nil, nil, nil)
	return err
}

func (c *Client) List(p string) ([]Entry, error) {
	body, err := c.do(http.MethodGet, "/dirs/", p, nil, nil, nil)
	if err != nil {
		return nil, err
	}
	var entries []Entry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("decode listing: %w", err)
	}
	return entries, nil
}

func (c *Client) Lock(name string) error {
	_, err := c.do(http.MethodPost, "/locks/", name, nil, nil, nil)
	return err
}

func (c *Client) Unlock(name string) error {
	_, err := c.do(http.MethodDelete, "/locks/", name, nil, nil, nil)
	return err
}

// Close releases idle connections.
func (c *Client) Close() {
	c.http.CloseIdleConnections()
}
