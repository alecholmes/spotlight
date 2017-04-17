package notifiers

import (
	"bytes"
	"fmt"

	"github.com/alecholmes/spotlight/app/model"
	"github.com/alecholmes/spotlight/app/templates"
	"github.com/alecholmes/spotlight/spotify"

	"github.com/go-errors/errors"
)

type Notifier struct {
	mailer     Mailer
	appBaseURL string
	fromEmail  string
}

func NewNotifier(appBaseURL, fromEmail string, mailer Mailer) *Notifier {
	return &Notifier{
		mailer:     mailer,
		appBaseURL: appBaseURL,
		fromEmail:  fromEmail,
	}
}

func (n *Notifier) SubscriptionUpdate(spotifyClient *spotify.SpotifyClient, activities []*model.Activity) error {
	cachedClient := spotify.NewCachingClient(spotifyClient)

	loggedInUser, err := n.getLoggedInUser(spotifyClient)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	templateData := templates.UpdateSubscriptionEmailData{
		AppBaseURL: n.appBaseURL,
	}

	var body bytes.Buffer
	for _, activity := range activities {
		actor, err := cachedClient.GetProfile(string(activity.Data.ActorUserID))
		if err != nil {
			return errors.Wrap(err, 0)
		}
		playlist, err := cachedClient.GetPlaylist(string(activity.Data.PlaylistOwnerID), string(activity.Data.PlaylistID))
		if err != nil {
			return errors.Wrap(err, 0)
		}
		if templateData.Playlist == nil {
			templateData.Playlist = templates.NewPlaylist(playlist, activity.SubscriptionToken)
		}

		templateData.Activities = append(templateData.Activities, templates.NewActivity(activity, actor, playlist))
	}

	templateData.ActorsDescription = templates.PrettyActorNames(templateData.Activities, 3)

	if err := templates.UpdateSubscriptionEmailHTML.Execute(&body, &templateData); err != nil {
		return errors.Wrap(err, 0)
	}

	subject := "Updates to your Spotify playlist"

	if err := n.mailer.SendHTML(n.fromEmail, []string{loggedInUser.Email}, nil, []string{n.fromEmail}, subject, body.String()); err != nil {
		return errors.WrapPrefix(err, "Error sending email", 0)
	}

	return nil
}

func (n *Notifier) SharePlaylist(spotifyClient *spotify.SpotifyClient, inviteeEmail string, playlist *spotify.Playlist) error {
	loggedInUser, err := n.getLoggedInUser(spotifyClient)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	templateData := templates.NewShareEmailData(loggedInUser, playlist, n.appBaseURL)

	var body bytes.Buffer
	if err := templates.ShareEmailHTML.Execute(&body, &templateData); err != nil {
		return errors.Wrap(err, 0)
	}

	subject := fmt.Sprintf("Follow some music with %s", loggedInUser.DisplayName)

	if err := n.mailer.SendHTML(n.fromEmail, []string{inviteeEmail}, nil, []string{n.fromEmail}, subject, body.String()); err != nil {
		return errors.WrapPrefix(err, "Error sending email", 0)
	}

	return nil
}

func (n *Notifier) getLoggedInUser(spotifyClient *spotify.SpotifyClient) (*spotify.PrivateProfile, error) {
	loggedInUser, err := spotifyClient.GetMyProfile()
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	if len(loggedInUser.Email) == 0 {
		return nil, errors.Errorf("No email found for user %s", loggedInUser.ID)
	}

	return loggedInUser, nil
}
