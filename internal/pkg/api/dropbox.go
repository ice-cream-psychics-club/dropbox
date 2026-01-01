package api

import (
	"encoding/json"
	"net/http"

	"github.com/ice-cream-psychics/dropbox/pkg/dropbox"
)

// TODO: DRY

type Dropbox struct {
	Client *dropbox.Client
}

type ListFolderResponse struct {
	Files []dropbox.File `json:"files"`
}

func (dbx *Dropbox) ListFolder(w http.ResponseWriter, r *http.Request) {
	folderName := r.URL.Query().Get("name")

	files, err := dbx.Client.ListFolder(folderName)
	if err != nil {
		WriteErrResponse(w, http.StatusInternalServerError, &Error{
			Type:    "BackendError",
			Message: err.Error(),
		})
		return
	}

	body, err := json.Marshal(&files)
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
