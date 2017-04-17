package jobs

import (
	"fmt"
	"strings"
	"time"

	"github.com/alecholmes/spotlight/app/model"
	"github.com/alecholmes/spotlight/app/notifiers"
	"github.com/alecholmes/spotlight/app/oauth"
	"github.com/alecholmes/spotlight/spotify"
	"github.com/alecholmes/spotlight/util"

	"github.com/go-errors/errors"
	"github.com/golang/glog"
)

const (
	SubscriptionCheckPeriod = 10 * time.Second
)

type UpdatePlaylistsJob struct {
	oauth         *oauth.OAuth
	userStore     model.UserStore
	playlistStore model.PlaylistStore
	notifier      *notifiers.Notifier
}

func NewUpdatePlaylistsJob(oauth *oauth.OAuth, userStore model.UserStore,
	playlistStore model.PlaylistStore, notifier *notifiers.Notifier) *UpdatePlaylistsJob {
	return &UpdatePlaylistsJob{
		oauth:         oauth,
		userStore:     userStore,
		playlistStore: playlistStore,
		notifier:      notifier,
	}
}

func (u *UpdatePlaylistsJob) Run() error {
	defer func() {
		if r := recover(); r != nil {
			glog.Errorf("Caught panic: %v", r)
		}
	}()

	subs, err := u.playlistStore.ListSubscriptionsToCheck(util.WallClock.Now(), 10)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	glog.Infof("Updating subs. count=%d", len(subs))

	users := make(map[model.UserID]*model.User)
	for _, sub := range subs {
		if user, ok := users[sub.UserID]; !ok {
			user, err = u.getUser(sub.UserID)
			if err != nil {
				return errors.WrapPrefix(err, fmt.Sprintf("Error getting user `%s`", sub.UserID), 0)
			} else if user == nil {
				return errors.WrapPrefix(err, fmt.Sprintf("User `%s` not found`", sub.UserID), 0)
			}
			users[user.ID] = user
		}
	}

	for _, sub := range subs {
		if err := u.updateSubscription(sub, users[sub.UserID]); err != nil {
			return err
		}
	}

	return nil
}

func (u *UpdatePlaylistsJob) getUser(userID model.UserID) (*model.User, error) {
	user, err := u.userStore.GetUser(userID)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	} else if user == nil {
		return nil, errors.Errorf("User `%s` not found", userID)
	}

	return user, nil
}

func (u *UpdatePlaylistsJob) updateSubscription(sub *model.Subscription, user *model.User) error {
	glog.Infof("Updating subscription. userID=%s subscriptionToken=%s playlistID=%s", sub.UserID, sub.Token, sub.PlaylistID)

	accessToken, err := u.oauth.AccessToken(user)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	client := spotify.NewSpotifyClient(accessToken)

	playlist, err := client.GetPlaylist(string(sub.PlaylistOwnerID), string(sub.PlaylistID))
	if err != nil {
		return errors.Wrap(err, 0)
	} else if playlist == nil {
		glog.Infof("Playlist deleted. ownerID=%s playlistID=%s", sub.PlaylistOwnerID, sub.PlaylistID)

		sub.NextCheckAt = nil
		if err := u.playlistStore.UpdateSubscriptions([]*model.Subscription{sub}); err != nil {
			return errors.Wrap(err, 0)
		}

		return nil
	}

	nextCheckAt := sub.NextCheckAt.Add(SubscriptionCheckPeriod)
	sub.NextCheckAt = &nextCheckAt
	if sub.PlaylistVersion != playlist.SnapshotID {
		prevTracks := make(map[string]bool)
		for _, trackID := range sub.PlaylistTrackIDs() {
			prevTracks[trackID] = true
		}

		var newTracks []*spotify.PlaylistTrack
		for _, track := range playlist.PlaylistTracks {
			if !prevTracks[track.Track.ID] {
				newTracks = append(newTracks, track)
			}
		}

		var newActivityData []*model.ActivityData
		for _, track := range newTracks {
			newActivityData = append(newActivityData, &model.ActivityData{
				PlaylistID:      sub.PlaylistID,
				PlaylistOwnerID: sub.PlaylistOwnerID,
				TrackAdded:      &model.TrackAdded{},
				TrackMetadata: &model.TrackMetadata{
					TrackID:     track.Track.ID,
					Name:        track.Track.Name,
					ArtistNames: artistNames(track.Track.Arists),
					AlbumName:   track.Track.Album.Name,
					URL:         track.Track.ExternalURLs["spotify"], // TODO: fix
					URI:         track.Track.URI,
				},
				ActorUserID: model.UserID(track.AddedBy.ID),
				OccuredAt:   track.AddedAt,
			})
			glog.Infof("New track. userID=%s subscriptionToken=%s playlistID=%s track=`%v`", sub.UserID, sub.Token, sub.PlaylistID, track)
		}

		var newActivities []*model.Activity
		if len(newActivityData) > 0 {
			if newActivities, err = u.playlistStore.AppendActivities(sub, newActivityData); err != nil {
				return errors.Wrap(err, 0)
			}
		}

		sub.PlaylistVersion = playlist.SnapshotID
		sub.PlaylistTracks = []byte(strings.Join(spotify.PlaylistTrackIDs(playlist), ","))

		// For notifications, filter out activities that the current user initiated
		filteredNewActivities := make([]*model.Activity, 0, len(newActivities))
		for _, activity := range newActivities {
			if activity.Data.ActorUserID != user.ID {
				filteredNewActivities = append(filteredNewActivities, activity)
			}
		}
		if len(filteredNewActivities) > 0 {
			if err := u.notifier.SubscriptionUpdate(client, newActivities); err != nil {
				glog.Errorf("Error notifying: %v", err)
			}
		} else {
			glog.Infof("Skipping notification since current user owns all activities. userID=%s playlistID=%s", sub.UserID, sub.PlaylistID)
		}
	}

	if err := u.playlistStore.UpdateSubscriptions([]*model.Subscription{sub}); err != nil {
		return errors.Wrap(err, 0)
	}

	return nil
}

func artistNames(artists []*spotify.Artist) []string {
	names := make([]string, len(artists))
	for i, artist := range artists {
		names[i] = artist.Name
	}

	return names
}
