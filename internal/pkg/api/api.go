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

var ErrStartup = errors.New("server is still starting up")

func NewDropbox(clientSecret string, logger *slog.Logger) *Dropbox {
	return &Dropbox{
		Logger:       logger,
		ClientSecret: clientSecret,
		errHandler: ErrHandler{
			Logger: logger,
		},
		cursors: &store.MemoryStore{},
	}
}

type Dropbox struct {
	Client       *dropbox.Client
	Logger       *slog.Logger
	ClientSecret string

	ready       atomic.Bool
	errHandler  ErrHandler
	cursors     *store.MemoryStore
	subscribers []Subscriber
}

type Subscriber interface {
	Handle(account string, files []dropbox.File) error
}

func (d *Dropbox) SetClient(client *dropbox.Client) {
	d.Client = client
	d.ready.Store(true)
}

func (d *Dropbox) Subscribe(subscribers ...Subscriber) {
	d.subscribers = append(d.subscribers, subscribers...)
}

func (d *Dropbox) DescribeFolder(w http.ResponseWriter, r *http.Request) {
	if !d.ready.Load() {
		d.errHandler.Write(w, http.StatusServiceUnavailable, ErrStartup)
		return
	}

	folderName := r.URL.Query().Get("name")
	cursor := r.URL.Query().Get("cursor")

	folder, err := d.Client.ListFolder(folderName, cursor)
	if err != nil {
		d.errHandler.Write(w, http.StatusInternalServerError, &Error{
			Type:    "BackendError",
			Message: err.Error(),
		})
		return
	}

	body, err := json.Marshal(&folder)
	if err != nil {
		d.errHandler.Write(w, http.StatusInternalServerError, &Error{
			Type:    "JSONError",
			Message: err.Error(),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(body)
}

func (d *Dropbox) DescribeFile(w http.ResponseWriter, r *http.Request) {
	if !d.ready.Load() {
		d.errHandler.Write(w, http.StatusServiceUnavailable, ErrStartup)
		return
	}

	path := r.URL.Query().Get("path")
	if len(path) == 0 {
		d.errHandler.Write(w, http.StatusBadRequest, &Error{
			Type:    "MissingField",
			Message: "missing `path` parameter in request URL",
		})
		return
	}

	file, err := d.Client.DescribeFile(path)
	if err != nil {
		d.errHandler.Write(w, http.StatusInternalServerError, &Error{
			Type:    "BackendError",
			Message: err.Error(),
		})
		return
	}

	body, err := json.Marshal(&file)
	if err != nil {
		d.errHandler.Write(w, http.StatusInternalServerError, &Error{
			Type:    "JSONError",
			Message: err.Error(),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(body)
}

func (d *Dropbox) VerifyWebhook(w http.ResponseWriter, r *http.Request) {
	if !d.ready.Load() {
		d.errHandler.Write(w, http.StatusServiceUnavailable, ErrStartup)
		return
	}

	challenge := r.URL.Query().Get("challenge")
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	w.Write([]byte(challenge))
	w.WriteHeader(http.StatusOK)
}

func (d *Dropbox) ReceiveUpdate(w http.ResponseWriter, r *http.Request) {
	if !d.ready.Load() {
		d.errHandler.Write(w, http.StatusServiceUnavailable, ErrStartup)
		return
	}

	// verify signature
	received := r.Header.Get("X-Dropbox-Signature")
	if len(received) == 0 {
		d.errHandler.Write(w, http.StatusBadRequest, &Error{
			Type:    "MissingField",
			Message: "missing header `X-Dropbox-Signature`",
		})
		return
	}

	mac := hmac.New(sha256.New, []byte(d.ClientSecret))
	body, err := io.ReadAll(r.Body)
	if err != nil {
		d.errHandler.Write(w, http.StatusBadRequest,
			fmt.Errorf("error reading request body: %v", err),
		)
		return
	}

	if _, err := mac.Write(body); err != nil {
		d.errHandler.Write(w, http.StatusInternalServerError,
			fmt.Errorf("error calculating expected MAC: %w", err),
		)
		return
	}
	expected := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(received), []byte(expected)) {
		d.errHandler.Write(w, http.StatusForbidden,
			fmt.Errorf("MACs did not match: expected %s, received %s", expected, received),
		)
		return
	}

	// decode the update
	var update dropbox.Update
	if err := json.Unmarshal(body, &update); err != nil {
		d.errHandler.Write(w, http.StatusBadRequest, &Error{
			Type:    "JSONError",
			Message: err.Error(),
		})
		return
	}

	// return response before processing the update
	w.WriteHeader(http.StatusAccepted)

	if err := d.processUpdate(update.ListFolder.Accounts); err != nil {
		d.Logger.Error(fmt.Sprintf("error processing update: %v", err))
	}
}

func (d *Dropbox) processUpdate(accounts []dropbox.Account) error {
	latest := ""
	for _, a := range accounts {
		account := string(a)

		cursor, err := d.cursors.Get(account)
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			return err
		}
		if errors.Is(err, store.ErrNotFound) && latest == "" {
			// no cursor in store yet; go get the latest
			cursor, err := d.Client.GetLatestCursor("")
			if err != nil {
				return fmt.Errorf("error getting latest cursor: %w", err)
			}

			latest = cursor
		}
		if errors.Is(err, store.ErrNotFound) {
			// store latest cursor so we can reference it on future calls
			cursor = latest
			d.cursors.Set(account, latest)
			continue
		}

		// get the delta from the previous cursor
		folder, err := d.Client.ListFolder("", cursor)
		if err != nil {
			return fmt.Errorf("error listing folder for %s @ cursor %s: %w", account, cursor, err)
		}

		// fan out the update to subscribers
		for _, subscriber := range d.subscribers {
			if err := subscriber.Handle(account, folder.Entries); err != nil {
				return fmt.Errorf("error handling %s @ cursor %s: %w",
					account, folder.Cursor, err,
				)
			}
		}

		// store current cursor
		d.cursors.Set(account, folder.Cursor)
	}

	return nil
}
