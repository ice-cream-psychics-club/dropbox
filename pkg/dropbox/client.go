package dropbox

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
)

const (
	BaseURL        = "https://api.dropboxapi.com/2"
	BaseNotifyURL  = "https://notify.dropboxapi.com/2"
	BaseContentURL = "https://content.dropboxapi.com/2"

	RootFolder = "/apps/content-selection/"
)

type Header struct {
	Name  string
	Value string
}

type ClientErr struct {
	StatusCode int
	Path       string
	Cause      error
}

func (e *ClientErr) Error() string {
	return fmt.Sprintf("status code %d from %s: %v", e.StatusCode, e.Path, e.Cause)
}

type Client struct {
	HTTPClient *http.Client
	Logger     *slog.Logger
}

func (c *Client) DescribeFile(filePath string) (*File, error) {
	urlPath := "/files/get_metadata"
	url := BaseURL + urlPath
	params := map[string]any{
		"path": filePath,
	}
	c.Logger.Debug("client.DescribeFile: " + url)

	req, err := newJSONRequest("POST", url, params, Header{
		"Content-Type", "application/json",
	})
	if err != nil {
		return nil, err
	}

	var file File
	if err := c.doRequest(req, urlPath, &file); err != nil {
		return nil, err
	}

	return &file, nil
}

func (c *Client) GetLatestCursor(filePath string) (string, error) {
	if _, found := strings.CutPrefix(filePath, "/"); !found {
		filePath = RootFolder + filePath
	}

	params := map[string]any{
		"path":      filePath,
		"recursive": true,
	}
	urlPath := "/files/list_folder/get_latest_cursor"
	url := BaseURL + urlPath
	c.Logger.Debug("client.GetLatestCursor: " + url)

	req, err := newJSONRequest("POST", url, params, Header{
		"Content-Type", "application/json",
	})
	if err != nil {
		return "", err
	}

	var cursor Cursor
	if err := c.doRequest(req, urlPath, &cursor); err != nil {
		return "", err
	}

	return cursor.Cursor, nil
}

func (c *Client) ListFolder(folderPath, cursor string) (*Folder, error) {
	if _, found := strings.CutPrefix(folderPath, "/"); !found {
		folderPath = RootFolder + folderPath
	}

	urlPath := "/files/list_folder"
	params := map[string]any{
		"path":      folderPath,
		"recursive": true,
	}

	if len(cursor) != 0 {
		urlPath += "/continue"
		params = map[string]any{
			"cursor": cursor,
		}
	}

	url := BaseURL + urlPath
	c.Logger.Debug("client.ListFolder: " + url)

	req, err := newJSONRequest("POST", url, params, Header{
		"Content-Type", "application/json",
	})
	if err != nil {
		return nil, err
	}

	var folder Folder
	if err := c.doRequest(req, urlPath, &cursor); err != nil {
		return nil, err
	}

	return &folder, nil
}

func (c *Client) Download(filePath string) (io.Reader, error) {
	if _, found := strings.CutPrefix(filePath, "/"); !found {
		filePath = RootFolder + filePath
	}

	// JSON-encode params
	params := map[string]any{
		"path": filePath,
	}

	buff := &bytes.Buffer{}
	encoder := json.NewEncoder(buff)
	if err := encoder.Encode(params); err != nil {
		return nil, fmt.Errorf("error encoding request argument: %w", err)
	}

	// then encode JSON into the URL
	query := url.Values{"arg": []string{buff.String()}}
	url := BaseContentURL + "/files/download?" + strings.TrimSpace(query.Encode())
	c.Logger.Debug("client.Download: " + url)

	// do request
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error forming http request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream; charset=utf-8")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request to /files/download: %w", err)
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		// happy path
		return resp.Body, nil
	}

	// handle error responses
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &ClientErr{
			StatusCode: resp.StatusCode,
			Path:       filePath,
			Cause:      fmt.Errorf("error reading response: %w", err),
		}
	}

	return nil, &ClientErr{
		StatusCode: resp.StatusCode,
		Path:       filePath,
		Cause:      errors.New(string(body)),
	}
}

func (c *Client) Upload(filePath string, r io.Reader) error {
	if _, found := strings.CutPrefix(filePath, "/"); !found {
		filePath = RootFolder + filePath
	}

	// JSON-encode params
	params := map[string]any{
		"path": filePath,
		"mode": "overwrite",
	}

	buff := &bytes.Buffer{}
	encoder := json.NewEncoder(buff)
	if err := encoder.Encode(params); err != nil {
		return fmt.Errorf("error encoding request argument: %w", err)
	}

	// then encode JSON into the URL
	query := url.Values{"arg": []string{buff.String()}}
	url := BaseContentURL + "/files/upload?" + strings.TrimSpace(query.Encode())
	c.Logger.Debug("client.Upload: " + url)

	// do request
	req, err := http.NewRequest("POST", url, r)
	if err != nil {
		return fmt.Errorf("error forming http request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("error calling upload: %w", err)
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		// happy path
		return nil
	}

	// handle error responses
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &ClientErr{
			StatusCode: resp.StatusCode,
			Path:       filePath,
			Cause:      fmt.Errorf("error reading response: %w", err),
		}
	}

	return &ClientErr{
		StatusCode: resp.StatusCode,
		Path:       filePath,
		Cause:      errors.New(string(body)),
	}

}

func (c *Client) doRequest(req *http.Request, path string, v any) error {
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("error making request to: %s: %w", path, err)
	}

	// happy path
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
			return fmt.Errorf("error parsing get_metadata response body: %w", err)
		}

		return nil
	}

	// handle error response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &ClientErr{
			StatusCode: resp.StatusCode,
			Path:       path,
			Cause:      fmt.Errorf("error reading response: %w", err),
		}
	}

	return &ClientErr{
		StatusCode: resp.StatusCode,
		Path:       path,
		Cause:      errors.New(string(body)),
	}
}

func newJSONRequest(method, url string, v any, headers ...Header) (*http.Request, error) {
	var body io.ReadWriter
	if v != nil {
		body = &bytes.Buffer{}
		encoder := json.NewEncoder(body)
		if err := encoder.Encode(v); err != nil {
			return nil, fmt.Errorf("error encoding request body: %w", err)
		}
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("error forming http request: %w", err)
	}

	for _, header := range headers {
		req.Header.Set(header.Name, header.Value)
	}

	return req, nil
}
