package requests

import (
	"encoding/base64"
	"net/http"

	"github.com/go-errors/errors"
	"github.com/golang/glog"
	"github.com/gorilla/sessions"
)

const (
	sessionName          = "spotifySession"
	sessionSpotifyUserID = "spotifyUserID"
)

type SessionConfig struct {
	Base64AuthenticationKey string `yaml:"authentication_key"`
	Base64EncryptionKey     string `yaml:"encryption_key"`
}

func (s *SessionConfig) AuthenticationKey() ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(s.Base64AuthenticationKey)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	return key, nil
}

func (s *SessionConfig) EncryptionKey() ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(s.Base64EncryptionKey)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	return key, nil
}

type Sessions struct {
	store sessions.Store
}

func NewSessions(config *SessionConfig) (*Sessions, error) {
	authKey, err := config.AuthenticationKey()
	if err != nil {
		return nil, err
	}
	encryptionKey, err := config.EncryptionKey()
	if err != nil {
		return nil, err
	}

	store := sessions.NewCookieStore(authKey, encryptionKey)
	return &Sessions{store: store}, nil
}

func (s *Sessions) GetSession(req *http.Request) (*Session, error) {
	session, err := s.store.Get(req, sessionName)
	if err != nil {
		glog.Errorf("Could not decode session: %v", err)
	}

	return newSession(session), nil
}

type Session struct {
	underlying    *sessions.Session
	spotifyUserID string
}

func newSession(underlying *sessions.Session) *Session {
	session := &Session{
		underlying: underlying,
	}

	if spotifyUserID, ok := underlying.Values[sessionSpotifyUserID]; ok {
		session.spotifyUserID = spotifyUserID.(string)
	}

	return session
}

func (s *Session) SpotifyUserID() string {
	if userID := s.underlying.Values[sessionSpotifyUserID]; userID != nil {
		return userID.(string)
	}
	return ""
}

func (s *Session) SetSpotifyUserID(userID string) {
	s.underlying.Values[sessionSpotifyUserID] = userID
}

func (s *Session) Delete(req *http.Request, rw http.ResponseWriter) {
	delete(s.underlying.Values, sessionSpotifyUserID)
	s.underlying.Options.MaxAge = -1
	s.underlying.Save(req, rw)
}

func (s *Session) Save(req *http.Request, rw http.ResponseWriter) {
	s.underlying.Save(req, rw)
}
