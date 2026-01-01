package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync/atomic"

	"github.com/ice-cream-psychics-club/dropbox/pkg/dropbox"
	"github.com/ice-cream-psychics-club/dropbox/pkg/store"
)

// TODO: DRY

type Dropbox struct {
	Client       *dropbox.Client
	Cursors      *store.MemoryStore
	Logger       *slog.Logger
	ClientSecret string
	ready        atomic.Bool

	subscribers []Subscriber
}

func (dbx *Dropbox) SetClient(client *dropbox.Client) {
	dbx.Client = client
	dbx.ready.Store(true)
}

type Subscriber interface {
	Handle(account string, files []dropbox.File) error
}

func (dbx *Dropbox) Subscribe(subscribers ...Subscriber) {
	dbx.subscribers = append(dbx.subscribers, subscribers...)
}

type Folder struct {
	Files []dropbox.File `json:"files"`
}

func (dbx *Dropbox) DescribeFolder(w http.ResponseWriter, r *http.Request) {
	if !dbx.ready.Load() {
		WriteErrResponse(w, http.StatusServiceUnavailable, fmt.Errorf("server is still starting up"))
		return
	}

	folderName := r.URL.Query().Get("name")
	cursor := r.URL.Query().Get("cursor")

	folder, err := dbx.Client.ListFolder(folderName, cursor)
	if err != nil {
		WriteErrResponse(w, http.StatusInternalServerError, &Error{
			Type:    "BackendError",
			Message: err.Error(),
		})
		return
	}

	body, err := json.Marshal(&folder)
	if err != nil {
		WriteErrResponse(w, http.StatusInternalServerError, &Error{
			Type:    "JSONError",
			Message: err.Error(),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(body)
}

func (dbx *Dropbox) DescribeFile(w http.ResponseWriter, r *http.Request) {
	if !dbx.ready.Load() {
		WriteErrResponse(w, http.StatusServiceUnavailable, fmt.Errorf("server is still starting up"))
		return
	}

	path := r.URL.Query().Get("path")
	if len(path) == 0 {
		WriteErrResponse(w, http.StatusBadRequest, &Error{
			Type:    "MissingInfo",
			Message: "missing `path` parameter in request URL",
		})
		return
	}

	metadata, err := dbx.Client.DescribeFile(path)
	if err != nil {
		WriteErrResponse(w, http.StatusInternalServerError, &Error{
			Type:    "BackendError",
			Message: err.Error(),
		})
		return
	}

	body, err := json.Marshal(&metadata)
	if err != nil {
		WriteErrResponse(w, http.StatusInternalServerError, &Error{
			Type:    "JSONError",
			Message: err.Error(),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(body)
}

func (dbx *Dropbox) VerifyWebhook(w http.ResponseWriter, r *http.Request) {
	challenge := r.URL.Query().Get("challenge")
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	w.Write([]byte(challenge))
	w.WriteHeader(http.StatusOK)
}

type Update struct {
	ListFolder struct {
		Accounts []Account `json:"accounts"`
	} `json:"list_folder"`
}

type Account string

func (dbx *Dropbox) ReceiveUpdate(w http.ResponseWriter, r *http.Request) {
	if !dbx.ready.Load() {
		WriteErrResponse(w, http.StatusServiceUnavailable, fmt.Errorf("server is still starting up"))
		return
	}

	received := r.Header.Get("X-Dropbox-Signature")
	if len(received) == 0 {
		w.WriteHeader(http.StatusUnauthorized)
		dbx.Logger.Error("missing header `X-Dropbox-Signature`")
		return
	}

	mac := hmac.New(sha256.New, []byte(dbx.ClientSecret))
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		dbx.Logger.Error(fmt.Sprintf("error reading request body: %v", err))
		return
	}

	if _, err := mac.Write(body); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		dbx.Logger.Error(fmt.Sprintf("error calculating expected MAC: %v", err))
		return
	}
	expected := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(received), []byte(expected)) {
		w.WriteHeader(http.StatusForbidden)
		dbx.Logger.Error(fmt.Sprintf("MACs did not match: expected %s, received %s", expected, received))
		return
	}

	var update Update
	if err := json.Unmarshal(body, &update); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		dbx.Logger.Error(fmt.Sprintf("error parsing request body %s: %w", string(body), err))
		return
	}

	w.WriteHeader(http.StatusAccepted)

	if err := dbx.processUpdate(update.ListFolder.Accounts); err != nil {
		dbx.Logger.Error(fmt.Sprintf("error processing update: %v", err))
	}
}

func (dbx *Dropbox) processUpdate(accounts []Account) error {
	latest := ""
	for _, a := range accounts {
		account := string(a)

		cursor, err := dbx.Cursors.Get(account)
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			return err
		}
		if errors.Is(err, store.ErrNotFound) && latest == "" {
			cursor, err := dbx.Client.GetLatestCursor("")
			if err != nil {
				return fmt.Errorf("error getting latest cursor: %w", err)
			}

			latest = cursor
		}
		if errors.Is(err, store.ErrNotFound) {
			cursor = latest
			dbx.Cursors.Set(account, latest)
		}

		folder, err := dbx.Client.ListFolder("", cursor)
		if err != nil {
			return fmt.Errorf("error listing folder for %s @ cursor %s: %w", account, cursor, err)
		}

		for _, subscriber := range dbx.subscribers {
			if err := subscriber.Handle(account, folder.Entries); err != nil {
				return fmt.Errorf("error handling %s @ cursor %s: %w",
					account, folder.Cursor, err,
				)
			}
		}

		dbx.Cursors.Set(account, folder.Cursor)
	}

	return nil
}
