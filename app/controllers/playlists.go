package controllers

import (
	"encoding/json"
	"net/http"

	"github.com/alecholmes/spotlight/app/jobs"
	"github.com/alecholmes/spotlight/app/model"
	"github.com/alecholmes/spotlight/app/oauth"
	"github.com/alecholmes/spotlight/app/requests"
	"github.com/alecholmes/spotlight/spotify"
	"github.com/alecholmes/spotlight/util"

	"github.com/golang/glog"
	"github.com/gorilla/mux"
)

type CreatePlaylistRequest struct {
	PlaylistName string `json:"playlist_name`
}

type Playlists struct {
	oauth           *oauth.OAuth
	spotifyClientFn func(user *model.User) (*spotify.SpotifyClient, error)
	playlistStore   model.PlaylistStore
	errorHandler    func(http.ResponseWriter, error)
	clock           util.Clock
}

func NewPlaylistsController(
	oauth *oauth.OAuth,
	spotifyClientFn func(user *model.User) (*spotify.SpotifyClient, error),
	playlistStore model.PlaylistStore,
	errorHandler func(http.ResponseWriter, error)) *Playlists {

	return &Playlists{
		oauth:           oauth,
		spotifyClientFn: spotifyClientFn,
		playlistStore:   playlistStore,
		errorHandler:    errorHandler,
		clock:           util.WallClock,
	}
}

func (p *Playlists) BindToMux(mux *mux.Router) {
	mux.HandleFunc("/playlists",
		requests.WithContext(p.oauth.MustBeAuthed(p.Create, p.errorHandler))).
		Methods(http.MethodPost)
}

func (p *Playlists) Create(rw http.ResponseWriter, req *http.Request) {
	// TODO: idempotency

	user := requests.MustUserFromContext(req.Context())

	defer req.Body.Close()
	createReq := new(CreatePlaylistRequest)
	if err := json.NewDecoder(req.Body).Decode(&createReq); err != nil {
		glog.Infof("Error decoding share request: %v", err)
		rw.WriteHeader(http.StatusBadRequest)
		return
	}

	if len(createReq.PlaylistName) == 0 {
		glog.Infof("Invalid request: %+v", createReq)
		rw.WriteHeader(http.StatusBadRequest)
		return
	}

	client, err := p.spotifyClientFn(user)
	if err != nil {
		p.errorHandler(rw, err)
		return
	}

	playlist, err := client.CreatePlaylist(string(user.ID), createReq.PlaylistName, spotify.PlaylistCollaborative)
	if err != nil {
		p.errorHandler(rw, err)
		return
	}
	glog.Infof("Created playlist. userID=`%s` playlistID=`%s` playlistName=`%s`", user.ID, playlist.ID, playlist.Name)

	sub := newSubscription(user.ID, playlist, p.clock.Now().Add(jobs.SubscriptionCheckPeriod))
	if _, err := p.playlistStore.CreateSubscription(sub); err != nil {
		p.errorHandler(rw, err)
		return
	}

	rw.Write([]byte(`{}`))
}
