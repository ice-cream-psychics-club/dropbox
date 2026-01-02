package api

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/http"

	"golang.org/x/oauth2"
)

const (
	code  = "code"
	state = "state"

	baseAuthURL  = "https://www.dropbox.com/oauth2/authorize"
	baseTokenURL = "https://www.dropbox.com/oauth2/token"
)

type OAuth2 struct {
	config            *oauth2.Config
	authURL           string
	codeVerifier      string
	state             string
	token             *oauth2.Token
	client            chan *http.Client
	errResponseWriter ErrHandler
}

func NewOAuth2(clientID, redirectURL string, logger *slog.Logger) (*OAuth2, error) {
	b := make([]byte, 96)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("error generating code verifier: %w", err)
	}

	codeVerifier := base64.RawURLEncoding.EncodeToString(b)
	hash := sha256.Sum256([]byte(codeVerifier))
	codeChallenge := base64.RawURLEncoding.EncodeToString(hash[:])

	b = b[:48]
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("error generating state: %w", err)
	}
	state := base64.RawURLEncoding.EncodeToString(b)

	config := oauth2.Config{
		ClientID:    clientID,
		RedirectURL: redirectURL,
		Endpoint: oauth2.Endpoint{
			AuthURL:  baseAuthURL,
			TokenURL: baseTokenURL,
		},
	}

	authURL := config.AuthCodeURL(state,
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
		oauth2.SetAuthURLParam("code_challenge", codeChallenge),
	)

	return &OAuth2{
		config:       &config,
		authURL:      authURL,
		codeVerifier: codeVerifier,
		state:        state,
		client:       make(chan *http.Client),
		errResponseWriter: ErrHandler{
			Logger: logger,
		},
	}, nil
}

func (o *OAuth2) AuthorizeHandle(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, o.authURL, http.StatusTemporaryRedirect)
}

func (o *OAuth2) ExchangeHandle(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get(code)
	state := r.URL.Query().Get(state)

	if subtle.ConstantTimeCompare([]byte(o.state), []byte(state)) == 0 {
		o.errResponseWriter.Write(w, http.StatusBadRequest, &Error{
			Type:    "OAuth2Error",
			Message: "states not equal",
		})
		return
	}

	token, err := o.config.Exchange(r.Context(), code, oauth2.SetAuthURLParam(
		"code_verifier", o.codeVerifier,
	))
	if err != nil {
		o.errResponseWriter.Write(w, http.StatusBadRequest, &Error{
			Type:    "OAuth2Error",
			Message: fmt.Sprintf("error exchanging token: %v", err),
		})
		return
	}

	o.token = token
	w.WriteHeader(http.StatusOK)

	o.client <- o.config.Client(r.Context(), o.token)
}

func (o *OAuth2) Client() <-chan *http.Client {
	return o.client
}
