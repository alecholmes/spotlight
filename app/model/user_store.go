package model

import (
	"sync"
	"time"
)

type UserID string

type User struct {
	ID                 UserID      `db:"id"`
	AccessToken        string      `db:"access_token"`
	RefreshToken       string      `db:"refresh_token"`
	ExpiresAt          time.Time   `db:"expires_at"`
	Name               string      `db:"name"`
	Email              string      `db:"email"`
	LastSeenActivityID *ActivityID `db:"last_seen_activity_id"`
	CreatedAt          time.Time   `db:"created_at"`
	UpdatedAt          time.Time   `db:"updated_at"`
}

type UserStore interface {
	GetUser(userID UserID) (*User, error)
	UpsertUser(user *User) (*User, error)
}

type InMemoryUserStore struct {
	mu    sync.Mutex
	users map[UserID]*User
	nowFn func() time.Time
}

var _ UserStore = &InMemoryUserStore{}

func NewInMemoryUserStore() UserStore {
	return &InMemoryUserStore{
		users: make(map[UserID]*User),
		nowFn: time.Now,
	}
}

func (i *InMemoryUserStore) GetUser(userID UserID) (*User, error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	return i.users[userID], nil
}

func (i *InMemoryUserStore) UpsertUser(user *User) (*User, error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	now := i.nowFn()
	if u, ok := i.users[user.ID]; ok {
		u.AccessToken = user.AccessToken
		u.RefreshToken = user.RefreshToken
		u.ExpiresAt = user.ExpiresAt
		u.UpdatedAt = now
		return u, nil
	}

	user.CreatedAt = now
	user.UpdatedAt = now
	i.users[user.ID] = user

	return user, nil
}
