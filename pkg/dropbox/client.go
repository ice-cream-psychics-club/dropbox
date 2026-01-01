package dropbox

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
)

// TODO: DRY

const (
	BaseURL        = "https://api.dropboxapi.com/2"
	BaseNotifyURL  = "https://notify.dropboxapi.com/2"
	BaseContentURL = "https://content.dropboxapi.com/2"

	RootFolder = "/apps/content-selection/"
)

type Client struct {
	HTTPClient *http.Client
	Logger     *slog.Logger
}

func (c *Client) DescribeFile(path string) (*File, error) {
	params := map[string]any{
		"path": path,
	}

	body := &bytes.Buffer{}
	encoder := json.NewEncoder(body)
	if err := encoder.Encode(params); err != nil {
		return nil, fmt.Errorf("error forming request body: %w", err)
	}

	url := BaseURL + "/files/get_metadata"
	c.Logger.Debug("client.DescribeFile: " + url)

	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, fmt.Errorf("error forming http request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error calling get_metadata: %w", err)
	}

	if resp.StatusCode > 299 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("received status code %d from get_metadata: error reading body: %w", resp.StatusCode, err)
		}

		return nil, fmt.Errorf("received status code %d from get_metadata: %v", resp.StatusCode, string(body))
	}

	var fileMetadata File
	if err := json.NewDecoder(resp.Body).Decode(&fileMetadata); err != nil {
		return nil, fmt.Errorf("error parsing get_metadata response body: %w", err)
	}

	return &fileMetadata, nil
}

func (c *Client) GetLatestCursor(path string) (string, error) {
	if _, found := strings.CutPrefix(path, "/"); !found {
		path = RootFolder + path
	}

	params := map[string]any{
		"path":      path,
		"recursive": true,
	}

	body := &bytes.Buffer{}
	encoder := json.NewEncoder(body)
	if err := encoder.Encode(params); err != nil {
		return "", fmt.Errorf("error forming request body: %w", err)
	}

	url := BaseURL + "/files/list_folder/get_latest_cursor"
	c.Logger.Debug("client.GetLatestCursor: " + url)

	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return "", fmt.Errorf("error forming http request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("error calling get_latest_cursor: %w", err)
	}

	if resp.StatusCode > 299 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("received status code %d from get_metadata: error reading body: %w", resp.StatusCode, err)
		}

		return "", fmt.Errorf("received status code %d from get_metadata: %v", resp.StatusCode, string(body))
	}

	var cursor Cursor
	if err := json.NewDecoder(resp.Body).Decode(&cursor); err != nil {
		return "", fmt.Errorf("error parsing get_latest_cursor response body: %w", err)
	}

	return cursor.Cursor, nil
}

func (c *Client) ListFolder(path, cursor string) (*Folder, error) {
	if _, found := strings.CutPrefix(path, "/"); !found {
		path = RootFolder + path
	}

	url := BaseURL + "/files/list_folder"
	params := map[string]any{
		"path":      path,
		"recursive": true,
	}

	if len(cursor) != 0 {
		url = url + "/continue"
		params = map[string]any{
			"cursor": cursor,
		}
	}

	c.Logger.Debug("client.ListFolder: " + url)

	body := &bytes.Buffer{}
	encoder := json.NewEncoder(body)
	if err := encoder.Encode(params); err != nil {
		return nil, fmt.Errorf("error forming request body: %w", err)
	}

	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, fmt.Errorf("error forming http request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error calling list_folder: %w", err)
	}

	if resp.StatusCode > 299 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("received status code %d from list_folder: error reading body: %w", resp.StatusCode, err)
		}

		return nil, fmt.Errorf("received status code %d from list_folder: %v", resp.StatusCode, string(body))
	}

	var folder Folder
	if err := json.NewDecoder(resp.Body).Decode(&folder); err != nil {
		return nil, fmt.Errorf("error parsing list_folder response body: %w", err)
	}

	return &folder, nil
}

func (c *Client) Download(path string) (io.Reader, error) {
	if _, found := strings.CutPrefix(path, "/"); !found {
		path = RootFolder + path
	}

	params := map[string]any{
		"path": path,
	}

	body := &bytes.Buffer{}
	encoder := json.NewEncoder(body)
	if err := encoder.Encode(params); err != nil {
		return nil, fmt.Errorf("error forming request body: %w", err)
	}

	query := url.Values{
		"arg": []string{body.String()},
	}

	url := BaseContentURL + "/files/download?" + strings.TrimSpace(query.Encode())
	c.Logger.Debug("client.Download: " + url)

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error forming http request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream; charset=utf-8")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error calling download: %w", err)
	}
	if resp.StatusCode > 299 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("received status code %d from download: error reading body: %w", resp.StatusCode, err)
		}

		return nil, fmt.Errorf("received status code %d from download: %v", resp.StatusCode, string(body))
	}

	return resp.Body, nil
}

func (c *Client) Upload(path string, r io.Reader) error {
	if _, found := strings.CutPrefix(path, "/"); !found {
		path = RootFolder + path
	}

	params := map[string]any{
		"path": path,
		"mode": "overwrite",
	}

	body := &bytes.Buffer{}
	encoder := json.NewEncoder(body)
	if err := encoder.Encode(params); err != nil {
		return fmt.Errorf("error forming request body: %w", err)
	}

	query := url.Values{
		"arg": []string{body.String()},
	}

	url := BaseContentURL + "/files/upload?" + strings.TrimSpace(query.Encode())
	c.Logger.Debug("client.Upload: " + url)

	req, err := http.NewRequest("POST", url, r)
	if err != nil {
		return fmt.Errorf("error forming http request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("error calling upload: %w", err)
	}
	if resp.StatusCode > 299 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("received status code %d from upload: error reading body: %w", resp.StatusCode, err)
		}

		return fmt.Errorf("received status code %d from upload: %v", resp.StatusCode, string(body))
	}

	return nil
}
