package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
)

type Error struct {
	Type    string
	Message string
}

func (e *Error) Error() string {
	return e.Type + ": " + e.Message
}

type ErrHandler struct {
	Logger *slog.Logger
}

func (erw *ErrHandler) Write(w http.ResponseWriter, statusCode int, err error) {
	erw.Logger.Error(fmt.Sprintf("status code %d: %w", statusCode, err))

	apiErr, ok := err.(*Error)
	if !ok {
		w.Header().Add("Content-Type", "text/plain")
		w.WriteHeader(statusCode)
		w.Write([]byte(err.Error()))
		return
	}

	body, err := json.Marshal(apiErr)
	if err != nil {
		w.Header().Add("Content-Type", "text/plain")
		w.WriteHeader(statusCode)
		w.Write([]byte(err.Error()))
	}

	w.Header().Add("Content-Type", "text/json")
	w.WriteHeader(statusCode)
	w.Write(body)
}
