package oauth

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/alecholmes/spotlight/app/model"
	"github.com/alecholmes/spotlight/app/requests"
	"github.com/alecholmes/spotlight/spotify"
	"github.com/alecholmes/spotlight/util"

	"github.com/go-errors/errors"
	"github.com/golang/glog"
	"github.com/gorilla/mux"
	"golang.org/x/oauth2"
)

func SpotifyConfig(appBaseURL string, config *Config, scopes []spotify.Scope) *oauth2.Config {
	scopeStrs := make([]string, len(scopes))
	for i, scope := range scopes {
		scopeStrs[i] = string(scope)
	}

	return &oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.spotify.com/authorize",
			TokenURL: "https://accounts.spotify.com/api/token",
		},
		RedirectURL: fmt.Sprintf("%s/authed", appBaseURL),
		Scopes:      scopeStrs,
	}
}

type Config struct {
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
}

type OAuth struct {
	config       *oauth2.Config
	sessions     *requests.Sessions
	userStore    model.UserStore
	errorHandler func(http.ResponseWriter, error)
	clock        util.Clock
}

func NewOAuth(config *oauth2.Config, sessions *requests.Sessions, userStore model.UserStore,
	errorHandler func(http.ResponseWriter, error)) *OAuth {
	return &OAuth{
		config:       config,
		sessions:     sessions,
		userStore:    userStore,
		errorHandler: errorHandler,
		clock:        util.WallClock,
	}
}

func (o *OAuth) BindToMux(mux *mux.Router) {
	mux.HandleFunc("/authed", requests.WithContext(o.CallbackHandler)).Methods(http.MethodGet)
}

func (o *OAuth) AccessToken(user *model.User) (string, error) {
	token := &oauth2.Token{
		AccessToken:  user.AccessToken,
		RefreshToken: user.RefreshToken,
		Expiry:       user.ExpiresAt.In(time.Local),
	}
	source := o.config.TokenSource(context.Background(), token)

	// Will this not refresh tokens that are expiring imminently?
	newToken, err := source.Token()
	if err != nil {
		return "", errors.Wrap(err, 0)
	}

	if newToken.AccessToken != user.AccessToken || newToken.RefreshToken != user.RefreshToken {
		glog.Infof("Saving new access token. userId=%s", user.ID)
		user.AccessToken = newToken.AccessToken
		user.RefreshToken = newToken.RefreshToken
		user.ExpiresAt = newToken.Expiry.In(time.UTC)
		if _, err := o.userStore.UpsertUser(user); err != nil {
			return "", errors.Wrap(err, 0)
		}
	}

	return newToken.AccessToken, nil
}

func (o *OAuth) MustBeAuthed(handler http.HandlerFunc, errorHandler func(http.ResponseWriter, error)) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		session, err := o.sessions.GetSession(req)
		if err != nil {
			errorHandler(rw, err)
			return
		}

		requiresAuth := true
		if len(session.SpotifyUserID()) > 0 {
			glog.Infof("Session with spotify user id `%s`", session.SpotifyUserID())
			user, err := o.userStore.GetUser(model.UserID(session.SpotifyUserID()))
			if err != nil {
				errorHandler(rw, err)
				return
			} else if user == nil {
				glog.Warningf("No persisted credentials found - restarting auth flow. userId=%s", session.SpotifyUserID())
			} else if _, err := o.AccessToken(user); err != nil {
				// TODO should handle unauthorized case
				glog.Infof("Unable to get access token. userId=%s error=`%v`", user.ID, err)
				errorHandler(rw, err)
				return
			} else {
				requiresAuth = false

				req = req.WithContext(context.WithValue(req.Context(), requests.ContextUser{}, user))
			}
		}

		if requiresAuth {
			glog.Infof("Redirecting to oauth flow. requestedURL=`%v`", req.URL)

			state := fmt.Sprintf("%s?%s", req.URL.Path, req.URL.RawQuery) // TODO: better

			rw.Header().Set("Location", o.config.AuthCodeURL(state, oauth2.AccessTypeOffline))
			rw.WriteHeader(http.StatusFound)
			return
		}

		handler(rw, req)
	}
}

func (o *OAuth) OptionallyAuthed(handler http.HandlerFunc, errorHandler func(http.ResponseWriter, error)) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		session, err := o.sessions.GetSession(req)
		if err != nil {
			errorHandler(rw, err)
			return
		}

		if len(session.SpotifyUserID()) > 0 {
			glog.Infof("Session with spotify user id `%s`", session.SpotifyUserID())

			if user, err := o.userStore.GetUser(model.UserID(session.SpotifyUserID())); err != nil {
				errorHandler(rw, err)
				return
			} else if user != nil {
				if _, err := o.AccessToken(user); err == nil {
					req = req.WithContext(context.WithValue(req.Context(), requests.ContextUser{}, user))
				}
			}
		}

		handler(rw, req)
	}
}

func (o *OAuth) CallbackHandler(rw http.ResponseWriter, req *http.Request) {
	session, err := o.sessions.GetSession(req)
	if err != nil {
		o.errorHandler(rw, err)
		return
	}

	query := req.URL.Query()
	if authErr := query.Get("error"); len(authErr) > 0 {
		glog.Infof("Auth denied: `%v`", req.URL) // TODO: better error handling, especially for access_denied
		rw.Header().Set("Location", "/")
		rw.WriteHeader(http.StatusFound)
		return
	}

	tokens, err := o.config.Exchange(context.Background(), query.Get("code"))
	if err != nil {
		o.errorHandler(rw, errors.Wrap(err, 0))
		return
	}

	client := spotify.NewSpotifyClient(tokens.AccessToken)
	profile, err := client.GetMyProfile()
	if err != nil {
		o.errorHandler(rw, errors.Wrap(err, 0))
		return
	}

	if _, err := o.userStore.UpsertUser(&model.User{
		ID:           model.UserID(profile.ID),
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		Name:         profile.DisplayName,
		Email:        profile.Email,
		ExpiresAt:    tokens.Expiry,
	}); err != nil {
		o.errorHandler(rw, errors.Wrap(err, 0))
		return
	}

	session.SetSpotifyUserID(profile.ID)
	session.Save(req, rw)

	// TODO: better validation and logging
	redirectLocation := req.FormValue("state")
	if len(redirectLocation) == 0 {
		redirectLocation = "/"
	}

	rw.Header().Set("Location", redirectLocation)
	rw.WriteHeader(http.StatusFound)
}
