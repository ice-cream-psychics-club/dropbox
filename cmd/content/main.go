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
	"github.com/ice-cream-psychics-club/dropbox/internal/pkg/content"
	"github.com/ice-cream-psychics-club/dropbox/internal/pkg/subscriber"
	"github.com/ice-cream-psychics-club/dropbox/pkg/csv"
	"github.com/ice-cream-psychics-club/dropbox/pkg/dropbox"
)

var developmentTimeout = 15 * time.Minute

func main() {
	var (
		clientID     = getEnvOrElse("DROPBOX_ACCESS_KEY")
		clientSecret = getEnvOrElse("DROPBOX_ACCESS_SECRET")
		host         = getEnvOrElse("HOST")
		port         = getEnvOrElse("PORT")
		redirectURL  = "http://" + host + ":" + port + "/oauth2/callback"
	)

	// setup dependencies
	ctx, cancel := context.WithTimeout(context.Background(), developmentTimeout)
	defer cancel()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// build APIs
	oauth2, err := api.NewOAuth2(clientID, redirectURL, logger)
	if err != nil {
		panic(err)
	}

	dbx := api.NewDropbox(clientSecret, logger)

	// build subscribers
	debugger := &subscriber.Logger{
		Logger: logger,
	}
	convertSubmissionsToCSV := &subscriber.Propagator{
		Source: "responses.xlsx",
		Targets: []subscriber.Target{
			{
				Name: "submissions.csv",
				Transform: func(r io.Reader) (io.Reader, error) {
					// TODO: add transformations
					filePath := "./tmp/responses.xlsx"
					return xlsxToCSV(filePath, r)
				},
			},
		},
		Logger: logger,
	}
	parseSubmissions := &subscriber.Propagator{
		Source: "ratings.csv",
		Targets: []subscriber.Target{
			{
				Name: "submissions.csv",
				Transform: func(r io.Reader) (io.Reader, error) {
					// import current ratings
					ratingsReader, err := dbx.Client.Download("ratings.csv")
					if err != nil {
						return nil, fmt.Errorf("error downloading ratings: %w", err)
					}
					ratings, members, err := content.ImportRatings(ratingsReader)
					if err != nil {
						return nil, fmt.Errorf("error importing ratings: %w", err)
					}

					// import previous submissions
					prevReader, err := dbx.Client.Download("prev_responses.csv")
					if err != nil {
						return nil, fmt.Errorf("error downloading previous responses: %w", err)
					}
					prev, err := content.ImportSubmissions(prevReader)
					if err != nil {
						return nil, fmt.Errorf("error importing previous submissions: %w", err)
					}

					// import current submissions
					curr, err := content.ImportSubmissions(r)
					if err != nil {
						return nil, fmt.Errorf("error importing submissions: %w", err)
					}

					// cal
					delta := content.CalculateDelta(prev, curr)
					if len(delta) == 0 {
						return r, nil
					}

					for _, s := range delta {
						id := content.SubmissionID{
							Title:     s.Title,
							Submitter: s.Member,
						}

						for _, m := range members {
							ratings[id] = append(ratings[id], content.Rating{
								Rater:        m,
								Interest:     -1,
								SubmissionID: id,
							})
						}
					}

					buff := &bytes.Buffer{}
					if err := content.ExportRatings(ratings, members, buff); err != nil {
						return nil, fmt.Errorf("error exporting ratings: %w", err)
					}

					return buff, nil
				},
			},
		},
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
	client := &dropbox.Client{
		HTTPClient: authClient,
		Logger:     logger,
	}

	convertSubmissionsToCSV.Client = client
	dbx.Subscribe(debugger, convertSubmissionsToCSV, parseSubmissions)
	dbx.SetClient(client)

	logger.Debug("ready to make dropbox requests")

	// handle shutdown
	select {
	case err := <-shutdown:
		logger.Error(fmt.Sprintf("done listening and serving: %v", err))
	case <-ctx.Done():
		logger.Error("context done")
	}
}

func newRouter(dbx *api.Dropbox, oauth2 *api.OAuth2) *mux.Router {
	base := mux.NewRouter()
	base.HandleFunc("/", oauth2.AuthorizeHandle)
	base.HandleFunc("/oauth2/callback", oauth2.ExchangeHandle)

	dropbox := base.Path("dropbox").Subrouter()
	dropbox.HandleFunc("/file", dbx.DescribeFile).Methods("GET")
	dropbox.HandleFunc("/folder", dbx.DescribeFolder).Methods("GET")
	dropbox.HandleFunc("/update", dbx.VerifyWebhook).Methods("GET")
	dropbox.HandleFunc("/update", dbx.ReceiveUpdate).Methods("POST")

	return base
}

func getEnvOrElse(k string) string {
	v := os.Getenv(k)
	if v == "" {
		panic(fmt.Errorf("missing %s", v))
	}

	return v
}

func xlsxToCSV(filePath string, in io.Reader) (io.Reader, error) {
	// create xlsx temp file
	f, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("error opening temporary file: %w", err)
	}
	defer f.Close()

	if _, err := f.ReadFrom(in); err != nil {
		return nil, fmt.Errorf("error writing to %s: %w", filePath, err)
	}

	// convert to csv
	buff := &bytes.Buffer{}
	if err := csv.FromXLSX(filePath, buff); err != nil {
		return nil, fmt.Errorf("error converting xlsx to csv: %w", err)
	}

	return buff, nil
}
