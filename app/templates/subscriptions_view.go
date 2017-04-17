package templates

import (
	"fmt"

	"github.com/alecholmes/spotlight/app/model"
	"github.com/alecholmes/spotlight/spotify"
)

var SubscriptionsView = extend(PageLayout, "subscriptions_view")

type Activity struct {
	ActorName    string
	Description  string
	PlaylistName string
	PlaylistURL  string
	EmbedURL     string
	TrackName    string
	TrackURL     string
}

type Playlist struct {
	ID                string
	Name              string
	OwnerID           string
	ExternalURL       string
	SubscriptionToken model.SubscriptionToken
}

func NewPlaylist(playlist *spotify.Playlist, subToken model.SubscriptionToken) *Playlist {
	return &Playlist{
		ID:                playlist.ID,
		Name:              playlist.Name,
		OwnerID:           playlist.Owner.ID,
		ExternalURL:       playlist.ExternalURLs["spotify"], // TODO fix
		SubscriptionToken: subToken,
	}
}

func NewActivity(activity *model.Activity, actor *spotify.PublicProfile, playlist *spotify.Playlist) *Activity {
	var description string
	if activity.Data.TrackAdded != nil {
		description = "added"
	} else {
		description = "did something mysterious to"
	}

	return &Activity{
		ActorName:    actor.DisplayName,
		Description:  description,
		PlaylistName: playlist.Name,
		PlaylistURL:  playlist.ExternalURLs["spotify"], // TODO fix
		EmbedURL:     fmt.Sprintf("https://embed.spotify.com/?uri=%s&theme=white", activity.Data.TrackMetadata.URI),
		TrackName:    activity.Data.TrackMetadata.Name,
		TrackURL:     activity.Data.TrackMetadata.URL,
	}
}

type SubscriptionsViewData struct {
	LayoutData
	Activities []*Activity
	Playlists  []*Playlist
}
