package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/alecholmes/spotlight/app/jobs"
	"github.com/alecholmes/spotlight/app/model"
	"github.com/alecholmes/spotlight/app/notifiers"
	"github.com/alecholmes/spotlight/app/oauth"
	"github.com/alecholmes/spotlight/app/requests"
	"github.com/alecholmes/spotlight/app/templates"
	"github.com/alecholmes/spotlight/spotify"
	"github.com/alecholmes/spotlight/util"

	"github.com/go-errors/errors"
	"github.com/golang/glog"
	"github.com/gorilla/mux"
)

var (
	emailRegexp = regexp.MustCompile(`^(([^<>()\[\]\.,;:\s@\"]+(\.[^<>()\[\]\.,;:\s@\"]+)*)|(\".+\"))@(([^<>()[\]\.,;:\s@\"]+\.)+[^<>()[\]\.,;:\s@\"]{2,})$`)
)

type ShareRequest struct {
	PlaylistOwnerID model.UserID     `json:"playlistOwnerId"`
	PlaylistID      model.PlaylistID `json:"playlistId"`
	Email           string           `json:"email"`
}

type Subscriptions struct {
	oauth           *oauth.OAuth
	spotifyClientFn func(user *model.User) (*spotify.SpotifyClient, error)
	playlistStore   model.PlaylistStore
	notifier        *notifiers.Notifier
	errorHandler    func(http.ResponseWriter, error)
	clock           util.Clock
}

func NewSubscriptionsController(
	oauth *oauth.OAuth,
	spotifyClientFn func(user *model.User) (*spotify.SpotifyClient, error),
	playlistStore model.PlaylistStore,
	notifier *notifiers.Notifier,
	errorHandler func(http.ResponseWriter, error)) *Subscriptions {

	return &Subscriptions{
		oauth:           oauth,
		spotifyClientFn: spotifyClientFn,
		playlistStore:   playlistStore,
		notifier:        notifier,
		errorHandler:    errorHandler,
		clock:           util.WallClock,
	}
}

func (s *Subscriptions) BindToMux(mux *mux.Router) {
	mux.HandleFunc("/subscriptions",
		requests.WithContext(s.oauth.MustBeAuthed(s.View, s.errorHandler))).
		Methods(http.MethodGet)
	mux.HandleFunc("/subscriptions/create",
		requests.WithContext(s.oauth.MustBeAuthed(s.Create, s.errorHandler))).
		Methods(http.MethodGet)
	mux.HandleFunc("/subscriptions/delete",
		requests.WithContext(s.oauth.OptionallyAuthed(s.Delete, s.errorHandler))).
		Methods(http.MethodGet)

	mux.HandleFunc("/subscriptions/share",
		requests.WithContext(s.oauth.OptionallyAuthed(s.ShareView, s.errorHandler))).
		Methods(http.MethodGet)

	// REST API for sharing a subscription from the subscriptions view
	mux.HandleFunc("/subscriptions/share",
		requests.WithContext(s.oauth.MustBeAuthed(s.ShareCreate, s.errorHandler))).
		Methods(http.MethodPost)
}

func (s *Subscriptions) View(rw http.ResponseWriter, req *http.Request) {
	user := requests.MustUserFromContext(req.Context())

	client, err := s.spotifyClientFn(user)
	if err != nil {
		s.errorHandler(rw, err)
		return
	}

	allPlaylists, err := client.ListMyPlaylists()
	if err != nil {
		s.errorHandler(rw, err)
		return
	}

	subs, err := s.playlistStore.ListSubscriptionsForUser(user.ID)
	if err != nil {
		s.errorHandler(rw, err)
		return
	}

	subTokens := make(map[model.PlaylistID]model.SubscriptionToken)
	for _, sub := range subs {
		subTokens[sub.PlaylistID] = sub.Token
	}

	playlists := make([]*templates.Playlist, 0, len(allPlaylists))
	for _, playlist := range allPlaylists {
		if playlist.Collaborative {
			playlists = append(playlists, templates.NewPlaylist(playlist, subTokens[model.PlaylistID(playlist.ID)]))
		}
	}

	activities, err := s.playlistStore.ListActivityForUser(user.ID, model.LatestActivityID, 25)
	if err != nil {
		s.errorHandler(rw, err)
		return
	}

	templatedActivities, err := s.toTemplateActivities(activities, client)
	if err != nil {
		s.errorHandler(rw, err)
		return
	}

	// TODO: include deleted playlists in view

	data := &templates.SubscriptionsViewData{
		LayoutData: templates.LayoutData{SignedIn: true},
		Activities: templatedActivities,
		Playlists:  playlists,
	}

	if err := templates.SubscriptionsView.Execute(rw, data); err != nil {
		glog.Errorf("Unable to render template: %v", err)
	}
}

func (s *Subscriptions) toTemplateActivities(activities []*model.Activity, client *spotify.SpotifyClient) ([]*templates.Activity, error) {
	templated := make([]*templates.Activity, len(activities))

	type playlistLookup struct {
		ownerID    model.UserID
		playlistID model.PlaylistID
	}

	users := make(map[model.UserID]*spotify.PublicProfile)
	playlistLookups := make(map[playlistLookup]interface{})
	playlists := make(map[model.PlaylistID]*spotify.Playlist)

	for _, activity := range activities {
		users[activity.Data.ActorUserID] = nil

		playlistLookup := playlistLookup{
			ownerID:    activity.Data.PlaylistOwnerID,
			playlistID: activity.Data.PlaylistID,
		}
		playlistLookups[playlistLookup] = nil
	}

	for userID := range users {
		user, err := client.GetProfile(string(userID))
		if err != nil {
			return nil, errors.Wrap(err, 0)
		}
		users[userID] = user
	}

	for playlistLookup := range playlistLookups {
		playlist, err := client.GetPlaylist(string(playlistLookup.ownerID), string(playlistLookup.playlistID))
		if err != nil {
			return nil, errors.Wrap(err, 0)
		}
		playlists[playlistLookup.playlistID] = playlist
	}

	for i, activity := range activities {
		templated[i] = templates.NewActivity(activity, users[activity.Data.ActorUserID], playlists[activity.Data.PlaylistID])
	}

	return templated, nil
}

func (s *Subscriptions) Create(rw http.ResponseWriter, req *http.Request) {
	user := requests.MustUserFromContext(req.Context())

	ownerID := req.URL.Query().Get("ownerId")
	playlistID := req.URL.Query().Get("playlistId")
	if len(ownerID) == 0 || len(playlistID) == 0 {
		glog.Infof("Attempt to create subscription without IDs set. ownerID=`%s` playlistID=`%s`", ownerID, playlistID)
		rw.Header().Set("Location", "/subscriptions")
		rw.WriteHeader(http.StatusFound)
		return
	}

	client, err := s.spotifyClientFn(user)
	if err != nil {
		s.errorHandler(rw, err)
		return
	}

	playlist, err := client.GetPlaylist(ownerID, playlistID)
	if err != nil {
		s.errorHandler(rw, err)
		return
	} else if playlist == nil {
		// TODO: 404
		s.errorHandler(rw, fmt.Errorf("Playlist not found"))
		return
	}

	nextCheckAt := s.clock.Now().Add(jobs.SubscriptionCheckPeriod)
	sub := newSubscription(user.ID, playlist, nextCheckAt)

	if _, err := s.playlistStore.CreateSubscription(sub); err != nil {
		s.errorHandler(rw, fmt.Errorf("Playlist not found"))
		return
	}

	if playlist.Owner.ID != string(user.ID) {
		if _, err := client.FollowPlaylist(playlist.Owner.ID, playlist.ID, true); err != nil {
			s.errorHandler(rw, err)
		}
	}

	rw.Header().Set("Location", "/subscriptions")
	rw.WriteHeader(http.StatusFound)
}

func (s *Subscriptions) Delete(rw http.ResponseWriter, req *http.Request) {
	subToken := model.SubscriptionToken(req.URL.Query().Get("token"))
	if len(subToken) == 0 {
		glog.Info("Attempt to delete subscription without token set")
		rw.Header().Set("Location", "/subscriptions")
		rw.WriteHeader(http.StatusFound)
		return
	}

	// TODO this is racy w.r.t the update job. Need to serialize on subscription or something like that.
	deleted, err := s.playlistStore.DeleteSubscription(subToken)
	if err != nil {
		s.errorHandler(rw, err)
		return
	}
	if !deleted {
		glog.Infof("Attempt to delete subscription that does not exist. subscriptionToken=`%s`", subToken)
	}

	if authed := requests.UserFromContext(req.Context()) != nil; authed {
		rw.Header().Set("Location", "/subscriptions")
	} else {
		rw.Header().Set("Location", "/")
	}

	rw.WriteHeader(http.StatusFound)
}

func (s *Subscriptions) ShareCreate(rw http.ResponseWriter, req *http.Request) {
	user := requests.MustUserFromContext(req.Context())

	defer req.Body.Close()
	shareReq := new(ShareRequest)
	if err := json.NewDecoder(req.Body).Decode(&shareReq); err != nil {
		glog.Infof("Error decoding share request: %v", err)
		rw.WriteHeader(http.StatusBadRequest)
		return
	}

	if len(shareReq.PlaylistOwnerID) == 0 || len(shareReq.PlaylistID) == 0 || len(shareReq.Email) == 0 {
		glog.Infof("Invalid request: %+v", shareReq)
		rw.WriteHeader(http.StatusBadRequest)
		return
	}

	if !emailRegexp.MatchString(shareReq.Email) {
		glog.Infof("Invalid email in request: `%+v`", shareReq)
		rw.WriteHeader(http.StatusBadRequest)
		return
	}

	client, err := s.spotifyClientFn(user)
	if err != nil {
		s.errorHandler(rw, err)
		return
	}

	playlist, err := client.GetPlaylist(string(shareReq.PlaylistOwnerID), string(shareReq.PlaylistID))
	if err != nil {
		s.errorHandler(rw, err)
		return
	}
	if playlist == nil {
		glog.Infof("Playlist not found: %+v", shareReq)
		rw.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := s.notifier.SharePlaylist(client, shareReq.Email, playlist); err != nil {
		glog.Errorf("Error sharing playlist: %v", err)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	rw.Write([]byte(`{}`))
}

func (s *Subscriptions) ShareView(rw http.ResponseWriter, req *http.Request) {
	user := requests.UserFromContext(req.Context())

	inviterName := req.URL.Query().Get("inviterName")
	inviterEmail := req.URL.Query().Get("inviterEmail")
	ownerID := req.URL.Query().Get("ownerId")
	playlistID := req.URL.Query().Get("playlistId")
	playlistName := req.URL.Query().Get("playlistName")

	if len(inviterName) == 0 || len(inviterEmail) == 0 || len(ownerID) == 0 || len(playlistID) == 0 || len(playlistName) == 0 {
		glog.Infof("Attempt to view share without IDs set. inviterName=`%s` inviterEmail=`%s` ownerID=`%s` playlistID=`%s` playlistName=`%s`",
			inviterName, inviterEmail, ownerID, playlistID, playlistName)
		rw.Header().Set("Location", "/")
		rw.WriteHeader(http.StatusFound)
		return
	}

	// If authenticated, use the normal subscribe endpoint
	if user != nil {
		s.Create(rw, req)
		return
	}

	query := make(url.Values)
	query.Set("ownerId", ownerID)
	query.Set("playlistId", playlistID)
	subscribeURL := fmt.Sprintf("/subscriptions/create?%s", query.Encode())

	viewData := &templates.ShareViewData{
		LayoutData:   templates.LayoutData{SignedIn: user != nil},
		InviterName:  inviterName,
		InviterEmail: inviterEmail,
		PlaylistName: playlistName,
		SubscribeURL: subscribeURL,
	}

	if err := templates.ShareView.Execute(rw, viewData); err != nil {
		glog.Errorf("Unable to render template: %v", err)
	}
}

func newSubscription(userID model.UserID, playlist *spotify.Playlist, nextCheckAt time.Time) *model.Subscription {
	sub := &model.Subscription{
		UserID:          userID,
		PlaylistID:      model.PlaylistID(playlist.ID),
		PlaylistOwnerID: model.UserID(playlist.Owner.ID),
		PlaylistName:    playlist.Name,
	}

	updateSubscription(sub, playlist, nextCheckAt)

	return sub
}

func updateSubscription(sub *model.Subscription, playlist *spotify.Playlist, nextCheckAt time.Time) {
	sub.PlaylistVersion = playlist.SnapshotID
	sub.PlaylistTracks = []byte(strings.Join(spotify.PlaylistTrackIDs(playlist), ","))
	sub.NextCheckAt = &nextCheckAt
}
