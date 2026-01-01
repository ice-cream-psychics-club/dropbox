package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"

	"github.com/ice-cream-psychics-club/dropbox/internal/pkg/api"
	"github.com/ice-cream-psychics-club/dropbox/internal/pkg/subscriber"
	"github.com/ice-cream-psychics-club/dropbox/pkg/csv"
	"github.com/ice-cream-psychics-club/dropbox/pkg/dropbox"
	"github.com/ice-cream-psychics-club/dropbox/pkg/store"
)

var developmentTimeout = 15 * time.Minute

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), developmentTimeout)
	defer cancel()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	var (
		clientID     = getEnvOrElse("DROPBOX_ACCESS_KEY")
		clientSecret = getEnvOrElse("DROPBOX_ACCESS_SECRET")
		host         = getEnvOrElse("HOST")
		port         = getEnvOrElse("PORT")
		redirectURL  = "http://" + host + ":" + port + "/oauth2/callback"
	)

	// setup dependencies
	oauth2, err := api.NewOAuth2(clientID, redirectURL)
	if err != nil {
		panic(err)
	}

	propagator := &subscriber.Propagator{
		Source: "responses.xlsx",
		Targets: []subscriber.Target{
			{
				Name: "responses.csv",
				Transform: func(r io.Reader) (io.Reader, error) {
					filePath := "./tmp/responses.xlsx"
					return xlsxToCSV(filePath, r)
				},
			},
		},
		Logger: logger,
	}

	dbx := &api.Dropbox{
		Logger:       logger,
		Cursors:      &store.MemoryStore{},
		ClientSecret: clientSecret,
	}

	// start server
	router := newRouter(dbx, oauth2)
	shutdown := make(chan error)
	go func() {
		if err := http.ListenAndServe(":"+port, api.LogRequests(logger, router)); err != nil {
			shutdown <- err
		}
	}()

	logger.Debug("client initialized")
	authClient := <-oauth2.Client()

	// TODO: messy, find a better way to block requests until client has arrived
	client := &dropbox.Client{
		HTTPClient: authClient,
		Logger:     logger,
	}
	dbx.SetClient(client)

	propagator.Client = client
	dbx.Subscribe(&subscriber.Logger{Logger: logger}, propagator)

	logger.Debug("ready to make dropbox requests")

	// shutdown
	select {
	case err := <-shutdown:
		logger.Error(fmt.Sprintf("done listening and serving: %v", err))
	case <-ctx.Done():
		logger.Error("context done")
	}
}

func newRouter(dbx *api.Dropbox, oauth2 *api.OAuth2) *mux.Router {
	router := mux.NewRouter()
	router.HandleFunc("/", oauth2.AuthorizeHandle)
	router.HandleFunc("/file/metadata", dbx.DescribeFile).Methods("GET")
	router.HandleFunc("/folder", dbx.DescribeFolder).Methods("GET")
	router.HandleFunc("/update", dbx.VerifyWebhook).Methods("GET")
	router.HandleFunc("/update", dbx.ReceiveUpdate).Methods("POST")
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

func xlsxToCSV(filePath string, in io.Reader) (io.Reader, error) {
	f, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("error opening temporary file: %w", err)
	}
	defer f.Close()

	if _, err := f.ReadFrom(in); err != nil {
		return nil, fmt.Errorf("error writing to %s: %w", filePath, err)
	}

	buff := &bytes.Buffer{}
	if err := csv.FromXLSX(filePath, buff); err != nil {
		return nil, fmt.Errorf("error converting xlsx to csv: %w", err)
	}

	return buff, nil
}
