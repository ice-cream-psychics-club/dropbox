package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"

	"github.com/ice-cream-psychics/dropbox/internal/pkg/api"
	"github.com/ice-cream-psychics/dropbox/pkg/dropbox"
)

var developmentTimeout = 15 * time.Minute

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), developmentTimeout)
	defer cancel()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	var (
		clientID    = getEnvOrElse("DROPBOX_ACCESS_KEY")
		host        = getEnvOrElse("HOST")
		port        = getEnvOrElse("PORT")
		redirectURL = "http://" + host + ":" + port + "/oauth2/callback"
	)

	// setup dependencies
	oauth2, err := api.NewOAuth2(clientID, redirectURL)
	if err != nil {
		panic(err)
	}

	dbx := &api.Dropbox{}

	// start server
	router := newRouter(dbx, oauth2)
	shutdown := make(chan error)
	go func() {
		if err := http.ListenAndServe(":"+port, api.LogRequests(logger, router)); err != nil {
			shutdown <- err
		}
	}()

	logger.Debug("client initialized")
	client := <-oauth2.Client()
	dbx.Client = &dropbox.Client{HTTPClient: client}

	logger.Debug("ready to make dropbox requests")

	// shutdown
	select {
	case err := <-shutdown:
		logger.Error("error listening and serving: %v", err)
	case <-ctx.Done():
		logger.Error("context done")
	}
}

func newRouter(dbx *api.Dropbox, oauth2 *api.OAuth2) *mux.Router {
	// TODO: implement missing routes
	var todo http.HandlerFunc

	router := mux.NewRouter()
	router.HandleFunc("/", oauth2.AuthorizeHandle)
	router.HandleFunc("/file/metadata", dbx.DescribeFile).Methods("GET")
	router.HandleFunc("/folder", dbx.ListFolder).Methods("GET")
	router.HandleFunc("/webhook", todo).Methods("POST")
	router.HandleFunc("/webhook", todo).Methods("GET")
	router.HandleFunc("/oauth2/callback", oauth2.ExchangeHandle)

	return router
}

func getEnvOrElse(k string) string {
	v := os.Getenv(k)
	if v == "" {
		panic(fmt.Errorf("missing %s", v))
	}

	return v
}
