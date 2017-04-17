package model

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"math"
	"strings"
	"time"

	"github.com/go-errors/errors"
)

type SubscriptionID int64
type SubscriptionToken string
type PlaylistID string
type ActivityID int64

const (
	LatestActivityID ActivityID = math.MaxInt64
)

type Subscription struct {
	Token           SubscriptionToken `db:"token"`
	UserID          UserID            `db:"user_id"`
	PlaylistID      PlaylistID        `db:"playlist_id"`
	PlaylistOwnerID UserID            `db:"playlist_owner_id"`
	PlaylistName    string            `db:"playlist_name"`
	PlaylistVersion string            `db:"playlist_version"`
	PlaylistTracks  []byte            `db:"playlist_tracks"`
	NextCheckAt     *time.Time        `db:"next_check_at"`
	CreatedAt       time.Time         `db:"created_at"`
	UpdatedAt       time.Time         `db:"updated_at"`
}

func (s *Subscription) PlaylistTrackIDs() []string {
	return strings.Split(string(s.PlaylistTracks), ",")
}

type TrackAdded struct {
}

type TrackMetadata struct {
	TrackID     string   `json:"track_id"`
	Name        string   `json:"name"`
	ArtistNames []string `json:"artist_names,omitempty"`
	AlbumName   string   `json:"album_name,omitempty"`
	URL         string   `json:"url,omitempty"`
	URI         string   `json:"uri,omitempty"`
}

type ActivityData struct {
	PlaylistID      PlaylistID     `json:"playlist_id"`
	PlaylistOwnerID UserID         `json:"playlist_owner_id"`
	TrackAdded      *TrackAdded    `json:"track_added,omitempty"`
	TrackMetadata   *TrackMetadata `json:"track_metadata"`
	ActorUserID     UserID         `json:"actor_user_id,omitempty"`
	OccuredAt       time.Time      `json:"occurred_at"`
}

var _ sql.Scanner = &ActivityData{}
var _ driver.Valuer = &ActivityData{}

func (a *ActivityData) UniqueID() string {
	var buf bytes.Buffer

	if a.TrackAdded != nil {
		buf.WriteString("track_added")
	}

	buf.WriteString(":")
	buf.WriteString(a.TrackMetadata.TrackID)

	return buf.String()
}

func (a *ActivityData) Value() (driver.Value, error) {
	return json.Marshal(a)
}

func (a *ActivityData) Scan(src interface{}) error {
	if bytes, ok := src.([]byte); !ok {
		return errors.Errorf("Expected []byte, not %T", src)
	} else if err := json.Unmarshal(bytes, &a); err != nil {
		return errors.Wrap(err, 0)
	}

	return nil
}

type Activity struct {
	ID                ActivityID        `db:"id"`
	UniqueID          string            `db:"unique_id"`
	SubscriptionToken SubscriptionToken `db:"subscription_token"`
	UserID            UserID            `db:"user_id"`
	Data              *ActivityData     `db:"data"`
	CreatedAt         time.Time         `db:"created_at"`
}

type PlaylistStore interface {
	CreateSubscription(sub *Subscription) (*Subscription, error)
	UpdateSubscriptions(subs []*Subscription) error
	DeleteSubscription(token SubscriptionToken) (bool, error)
	ListSubscriptionsForUser(userID UserID) ([]*Subscription, error)
	ListSubscriptionsToCheck(from time.Time, limit int) ([]*Subscription, error)

	AppendActivities(sub *Subscription, data []*ActivityData) ([]*Activity, error)
	ListActivityForUser(userID UserID, to ActivityID, limit int) ([]*Activity, error)
}
