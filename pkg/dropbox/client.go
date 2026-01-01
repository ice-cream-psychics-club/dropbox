package dropbox

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// TODO: DRY

const (
	BaseURL        = "https://api.dropboxapi.com/2"
	BaseNotifyURL  = "https://notify.dropboxapi.com/2"
	BaseContentURL = "https://content.dropboxapi.com/2"
)

type Client struct {
	HTTPClient *http.Client
}

func (c *Client) DescribeFile(filePath string) (*File, error) {
	params := map[string]any{
		"path": filePath,
	}

	body := &bytes.Buffer{}
	encoder := json.NewEncoder(body)
	if err := encoder.Encode(params); err != nil {
		return nil, fmt.Errorf("error forming request body: %w", err)
	}

	req, err := http.NewRequest("POST", BaseURL+"/files/get_metadata", body)
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

func (c *Client) ListFolder(folderName string) ([]File, error) {
	params := map[string]any{
		"path": folderName,
	}

	body := &bytes.Buffer{}
	encoder := json.NewEncoder(body)
	if err := encoder.Encode(params); err != nil {
		return nil, fmt.Errorf("error forming request body: %w", err)
	}

	req, err := http.NewRequest("POST", BaseURL+"/files/list_folder", body)
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

	return folder.Entries, nil
}
