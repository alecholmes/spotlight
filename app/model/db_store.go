package model

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/alecholmes/spotlight/util"

	"github.com/go-errors/errors"
	"github.com/go-sql-driver/mysql"
	"github.com/satori/go.uuid"
	"github.com/square/squalor"
)

type DBConfig struct {
	HostName string
	Port     int
	User     string
	Password string
	Database string `yaml:"db_name"`
}

func NewDB(config *DBConfig) (*sql.DB, error) {
	db, err := sql.Open("mysql", mySQLConnectionString(config))
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	if err := db.Ping(); err != nil {
		return nil, errors.Wrap(err, 0)
	}

	return db, nil
}

func mySQLConnectionString(config *DBConfig) string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?strict=true&parseTime=true",
		config.User, config.Password, config.HostName, config.Port, config.Database)
}

type DBStore struct {
	db *squalor.DB
}

var _ UserStore = &DBStore{}
var _ PlaylistStore = &DBStore{}

func NewDBStore(db *sql.DB) *DBStore {
	squalorDB := squalor.NewDB(db)

	squalorDB.MustBindModel("users", &User{})
	squalorDB.MustBindModel("subscriptions", &Subscription{})
	squalorDB.MustBindModel("activities", &Activity{})

	return &DBStore{
		db: squalorDB,
	}
}

func (d *DBStore) GetUser(userID UserID) (*User, error) {
	user := new(User)
	if err := d.db.Get(user, userID); err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	return user, nil
}

func (d *DBStore) UpsertUser(user *User) (*User, error) {
	now := util.WallClock.Now()

	tx, err := d.db.Begin()
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	defer tx.Rollback()

	var updated *User
	var existing []User
	if tx.Select(&existing, "SELECT * FROM users WHERE id = ? FOR UPDATE", user.ID); err != nil {
		return nil, errors.Wrap(err, 0)
	} else if len(existing) == 0 {
		updated = user
		updated.CreatedAt = now
		updated.UpdatedAt = now

		if err := tx.Insert(updated); err != nil {
			return nil, errors.Wrap(err, 0)
		}
	} else {
		updated := existing[0]
		updated.AccessToken = user.AccessToken
		updated.RefreshToken = user.RefreshToken
		updated.ExpiresAt = user.ExpiresAt
		updated.UpdatedAt = now

		if _, err := tx.Update(&updated); err != nil {
			return nil, errors.Wrap(err, 0)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, errors.Wrap(err, 0)
	}

	return updated, nil
}

func (d *DBStore) CreateSubscription(sub *Subscription) (*Subscription, error) {
	now := util.WallClock.Now()

	sub.Token = SubscriptionToken(strings.Replace(uuid.NewV4().String(), "-", "", -1))
	sub.CreatedAt = now
	sub.UpdatedAt = now

	if err := d.db.Insert(sub); duplicateKeyErr(err) {
		var loaded []*Subscription
		if err := d.db.Select(&loaded, "SELECT * FROM subscriptions WHERE user_id = ? AND playlist_id = ?",
			sub.UserID, sub.PlaylistID); err != nil {
			return nil, errors.Wrap(err, 0)
		}
		return loaded[0], nil
	} else if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	return sub, nil
}
func (d *DBStore) UpdateSubscriptions(subs []*Subscription) error {
	now := util.WallClock.Now()

	// NB: ideally optimistic locking instead of last-write-wins
	updates := make([]interface{}, len(subs))
	for i, sub := range subs {
		sub.UpdatedAt = now
		updates[i] = sub
	}

	if _, err := d.db.Update(updates...); err != nil {
		return errors.Wrap(err, 0)
	}

	return nil
}

func (d *DBStore) DeleteSubscription(token SubscriptionToken) (bool, error) {
	tx, err := d.db.Begin()
	if err != nil {
		return false, errors.Wrap(err, 0)
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM activities WHERE subscription_token = ?", token); err != nil {
		return false, errors.Wrap(err, 0)
	}

	res, err := tx.Exec("DELETE FROM subscriptions WHERE token = ?", token)
	if err != nil {
		return false, errors.Wrap(err, 0)
	} else if deletedCount, err := res.RowsAffected(); err != nil {
		return false, errors.Wrap(err, 0)
	} else if deletedCount == 0 {
		return false, nil
	}

	if err := tx.Commit(); err != nil {
		return false, errors.Wrap(err, 0)
	}

	return true, nil
}

func (d *DBStore) ListSubscriptionsForUser(userID UserID) ([]*Subscription, error) {
	var subs []*Subscription
	if err := d.db.Select(&subs, "SELECT * FROM subscriptions WHERE user_id = ? ORDER BY token", userID); err != nil {
		return nil, errors.Wrap(err, 0)
	}

	return subs, nil
}

func (d *DBStore) ListSubscriptionsToCheck(from time.Time, limit int) ([]*Subscription, error) {
	var subs []*Subscription
	if err := d.db.Select(&subs, "SELECT * FROM subscriptions WHERE next_check_at <= ? ORDER BY next_check_at LIMIT ?",
		from, limit); err != nil {
		return nil, errors.Wrap(err, 0)
	}

	return subs, nil
}

func (d *DBStore) AppendActivities(sub *Subscription, data []*ActivityData) ([]*Activity, error) {
	now := util.WallClock.Now()

	tx, err := d.db.Begin()
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	defer tx.Rollback()

	activities := make([]*Activity, len(data))
	for i, d := range data {
		activity := &Activity{
			UniqueID:          d.UniqueID(),
			SubscriptionToken: sub.Token,
			UserID:            sub.UserID,
			Data:              d,
			CreatedAt:         now,
		}
		activities[i] = activity

		if err := tx.Insert(activity); err != nil && !duplicateKeyErr(err) {
			return nil, errors.Wrap(err, 0)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, errors.Wrap(err, 0)
	}

	return activities, nil
}

func (d *DBStore) ListActivityForUser(userID UserID, to ActivityID, limit int) ([]*Activity, error) {
	var activities []*Activity
	if err := d.db.Select(&activities, "SELECT * FROM activities WHERE user_id = ? AND id <= ? ORDER BY id DESC LIMIT ?",
		userID, to, limit); err != nil {
		return nil, errors.Wrap(err, 0)
	}

	return activities, nil
}

func duplicateKeyErr(err error) bool {
	if err == nil {
		return false
	}
	if mysqlErr, ok := err.(*mysql.MySQLError); ok {
		if mysqlErr.Number == 1062 {
			return true
		}
	}

	return false
}
