package templates

import (
	"fmt"
	"net/url"

	"github.com/alecholmes/spotlight/app/model"
	"github.com/alecholmes/spotlight/spotify"
)

type FullUser struct {
	Name  string
	Email string
}

type ShareEmailData struct {
	Inviter      *FullUser
	Playlist     *Playlist
	SubscribeURL string
	AppBaseURL   string
}

func NewShareEmailData(inviter *spotify.PrivateProfile, playlist *spotify.Playlist, appBaseURL string) *ShareEmailData {
	query := make(url.Values)
	query.Set("inviterName", inviter.DisplayName)
	query.Set("inviterEmail", inviter.Email)
	query.Set("ownerId", playlist.Owner.ID)
	query.Set("playlistId", playlist.ID)
	query.Set("playlistName", playlist.Name)

	subscribeURL := fmt.Sprintf("%s/subscriptions/share?%s", appBaseURL, query.Encode())

	return &ShareEmailData{
		Inviter:      NewFullUser(inviter),
		Playlist:     NewPlaylist(playlist, model.SubscriptionToken("")),
		SubscribeURL: subscribeURL,
		AppBaseURL:   appBaseURL,
	}
}

func NewFullUser(user *spotify.PrivateProfile) *FullUser {
	return &FullUser{
		Name:  user.DisplayName,
		Email: user.Email,
	}
}

var ShareEmailHTML = parse("share_email")
