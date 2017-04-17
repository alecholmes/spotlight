package app

import (
	"database/sql"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/alecholmes/spotlight/app/controllers"
	"github.com/alecholmes/spotlight/app/jobs"
	"github.com/alecholmes/spotlight/app/model"
	"github.com/alecholmes/spotlight/app/notifiers"
	"github.com/alecholmes/spotlight/app/oauth"
	"github.com/alecholmes/spotlight/app/requests"
	"github.com/alecholmes/spotlight/spotify"

	"github.com/braintree/manners"
	"github.com/go-errors/errors"
	"github.com/golang/glog"
	"github.com/gorilla/mux"
)

var (
	requiredScopes = []spotify.Scope{
		spotify.ScopePlaylistReadCollaborative,
		spotify.ScopePlaylistModifyPublic,
		spotify.ScopePlaylistReadPrivate,
		spotify.ScopePlaylistModifyPrivate,
		spotify.ScopeUserFollowRead,
		spotify.ScopeUserFollowModify,
		spotify.ScopeUserReadPrivate,
		spotify.ScopeUserReadEmail,
	}
)

type App struct {
	config *AppConfig
}

func NewApp(config *AppConfig) *App {
	return &App{config: config}
}

func (a *App) Run(stopCh <-chan struct{}) {
	// Initialize the database
	glog.Info("Initializing DB")
	db, stopDB, err := a.initDB()
	if err != nil {
		glog.Errorf("Error initializing DB: %v", err)
		return
	}
	defer stopDB()

	store := model.NewDBStore(db)
	sessions, err := requests.NewSessions(a.config.HTTPSession)
	if err != nil {
		glog.Errorf("Error initializing HTTP sessions: %v", err)
		return
	}

	// Create notifier
	mailer := notifiers.NewMailerFromConfig(a.config.Email)
	notifier := notifiers.NewNotifier(a.config.AppBaseURL, a.config.AppEmail, mailer)

	// Create controllers
	router := mux.NewRouter()
	router.NotFoundHandler = http.HandlerFunc(controllers.Render404)

	oauth := oauth.NewOAuth(oauth.SpotifyConfig(a.config.AppBaseURL, a.config.OAuth, requiredScopes), sessions, store, controllers.Render500)
	oauth.BindToMux(router)

	controllers.NewHome(sessions).BindToMux(router, oauth, controllers.Render500)

	spotifyClientFn := func(user *model.User) (*spotify.SpotifyClient, error) { return newSpotifyClient(oauth, user) }
	controllers.NewSubscriptionsController(
		oauth, spotifyClientFn, store, notifier, controllers.Render500).
		BindToMux(router)

	controllers.NewPlaylistsController(oauth, spotifyClientFn, store, controllers.Render500).
		BindToMux(router)

	// Serve HTTP endpoints
	glog.Info("Initializing HTTP")
	stopHTTP, httpErrCh, err := a.initHTTP(a.config.HTTPServer.Port, loggingHandler(router))
	if err != nil {
		glog.Errorf("Error initializing endpoints: %v", err)
	}
	defer stopHTTP()

	// Start jobs
	glog.Info("Initializing jobs")
	stopUpdatePlaylistJob := a.initUpdatePlaylistJob(oauth, store, store, notifier)
	defer stopUpdatePlaylistJob()

	// Wait for the app to stop or a fatal HTTP error to occur
	select {
	case <-stopCh:
	case err := <-httpErrCh:
		glog.Errorf("HTTP server error: %v", err)
	}

	glog.Info("Shutting down")
}

func (a *App) initDB() (*sql.DB, func(), error) {
	db, err := model.NewDB(a.config.Database)
	if err != nil {
		return nil, nil, err
	}

	return db, func() {
		glog.Info("Shutting down DB")
		logError(db.Close)
	}, nil
}

func (a *App) initHTTP(port int, router http.Handler) (func(), <-chan error, error) {
	var wg sync.WaitGroup
	wg.Add(1)

	errCh := make(chan error, 1)
	go func() {
		if err := manners.ListenAndServe(fmt.Sprintf(":%d", port), router); err != nil {
			errCh <- err
		}

		wg.Done()
	}()

	return func() {
		glog.Info("Shutting down HTTP")
		manners.Close()
		wg.Wait()
		close(errCh)
	}, errCh, nil
}

func (a *App) initUpdatePlaylistJob(oauth *oauth.OAuth, userStore model.UserStore,
	playlistStore model.PlaylistStore, notifier *notifiers.Notifier) func() {

	stopCh := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(1)

	job := jobs.NewUpdatePlaylistsJob(oauth, userStore, playlistStore, notifier)
	go func() {
		for done := false; !done; {
			next := time.After(10 * time.Second)
			select {
			case <-next:
				if err := job.Run(); err != nil {
					glog.Errorf("Error updating playlists. %v", err)
				}
			case <-stopCh:
				done = true
			}
		}

		wg.Done()
	}()

	return func() {
		glog.Info("Shutting down update playlist job")
		close(stopCh)
		wg.Wait()
	}
}

func logError(fn func() error) func() {
	return func() {
		if err := fn(); err != nil {
			glog.Error(err)
		}
	}
}

func newSpotifyClient(oauth *oauth.OAuth, user *model.User) (*spotify.SpotifyClient, error) {
	accessToken, err := oauth.AccessToken(user)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	return spotify.NewSpotifyClient(accessToken), nil
}
