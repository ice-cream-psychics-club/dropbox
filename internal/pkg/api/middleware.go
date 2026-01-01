package api

import (
	"log/slog"
	"net/http"
)

func LogRequests(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.Info(r.Method + " " + r.URL.String())
		next.ServeHTTP(w, r)
	})
}
