package spotify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/go-errors/errors"
	"github.com/golang/glog"
)

type PlaylistVisibility int

const (
	PlaylistPrivate PlaylistVisibility = iota
	PlaylistPublic
	PlaylistCollaborative

	spotifyAPIURL = "https://api.spotify.com"
)

type PrivateProfile struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name,omitempty"`
	Email       string `json:"email,omitempty"`
	URI         string `json:"uri,omitempty"`
}

type Artist struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	ExternalURLs map[string]string `json:"external_urls"`
}

type Album struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	ExternalURLs map[string]string `json:"external_urls"`
}

type Track struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Arists       []*Artist         `json:"artists"`
	Album        *Album            `json:"album"`
	ExternalURLs map[string]string `json:"external_urls"`
	URI          string            `json:"uri"`
}

type PublicProfile struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

type PlaylistTrack struct {
	Track   *Track         `json:"track"`
	AddedAt time.Time      `json:"added_at"`
	AddedBy *PublicProfile `json:"added_by"`
}

type listPlaylistTracks struct {
	PlaylistTracks []*PlaylistTrack `json:"items"`
	Next           string           `json:"next,omitempty"`
}

type Playlist struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Owner          *PublicProfile    `json:"owner"`
	SnapshotID     string            `json:"snapshot_id"`
	Collaborative  bool              `json:"collaborative"`
	ExternalURLs   map[string]string `json:"external_urls"`
	PlaylistTracks []*PlaylistTrack
	RawTracks      listPlaylistTracks `json:"tracks"`
}

type listPlaylists struct {
	Playlists []*Playlist `json:"items"`
	Next      string      `json:"next,omitempty"`
}

type NotFoundError struct {
	url *url.URL
}

var _ error = &NotFoundError{}

func (n *NotFoundError) Error() string {
	return fmt.Sprintf("Resource not found: %v", n.url)
}

type SpotifyClient struct {
	accessToken string
}

func NewSpotifyClient(accessToken string) *SpotifyClient {
	return &SpotifyClient{accessToken: accessToken}
}

func (s *SpotifyClient) GetMyProfile() (*PrivateProfile, error) {
	profile := new(PrivateProfile)
	_, err := s.get("/v1/me", nil, false, &profile)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	return profile, nil
}

func (s *SpotifyClient) GetProfile(userID string) (*PublicProfile, error) {
	profile := new(PublicProfile)
	_, err := s.get(fmt.Sprintf("/v1/users/%s", userID), nil, false, &profile)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	return profile, nil
}

func (s *SpotifyClient) ListMyPlaylists() ([]*Playlist, error) {
	wrappedPlaylists := new(listPlaylists)
	_, err := s.get("/v1/me/playlists", map[string]string{"limit": "50"}, false, &wrappedPlaylists)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	allPlaylists := wrappedPlaylists.Playlists
	for next := wrappedPlaylists.Next; len(next) > 0; {
		playlists := new(listPlaylists)
		if _, err := s.get(next, nil, false, playlists); err != nil {
			return nil, err
		}

		allPlaylists = append(allPlaylists, playlists.Playlists...)
		next = playlists.Next
	}

	return allPlaylists, nil
}

func (s *SpotifyClient) GetPlaylist(userID, playlistID string) (*Playlist, error) {
	playlist := new(Playlist)
	resp, err := s.get(fmt.Sprintf("/v1/users/%s/playlists/%s", userID, playlistID), nil, true, &playlist)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	} else if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	// Load all additional pages for paginated results.
	allTracks := playlist.RawTracks.PlaylistTracks
	for next := playlist.RawTracks.Next; len(next) > 0; {
		tracks := new(listPlaylistTracks)
		if _, err := s.get(next, nil, false, tracks); err != nil {
			return nil, err
		}

		allTracks = append(allTracks, tracks.PlaylistTracks...)
		next = tracks.Next
	}

	playlist.PlaylistTracks = allTracks

	return playlist, nil
}

func (s *SpotifyClient) CreatePlaylist(userID, name string, visibility PlaylistVisibility) (*Playlist, error) {
	req := map[string]interface{}{
		"name":          name,
		"public":        visibility == PlaylistPublic,
		"collaborative": visibility == PlaylistCollaborative,
	}

	playlist := new(Playlist)
	if _, err := s.post(fmt.Sprintf("/v1/users/%s/playlists", userID), req, playlist); err != nil {
		return nil, err
	}

	return playlist, nil
}

func (s *SpotifyClient) FollowPlaylist(ownerID, playlistID string, public bool) (*http.Response, error) {
	path := fmt.Sprintf("/v1/users/%s/playlists/%s/followers", ownerID, playlistID)
	queryParams := map[string]string{
		"public": fmt.Sprintf("%t", public),
	}
	resp, err := s.put(path, queryParams, nil)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	return resp, nil
}

// path may be either a relative ("/foo/bar") or absoluate ("http://example.com/foo/bar").
// If path is relative then it will be prefixed with spotifyAPIURL.
func (s *SpotifyClient) newRequest(method, path string, queryParams map[string]string, reqBody interface{}) (*http.Request, error) {
	u, err := url.Parse(path)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	} else if len(u.Scheme) == 0 {
		return s.newRequest(method, fmt.Sprintf("%s%s", spotifyAPIURL, path), queryParams, reqBody)
	}

	for k, v := range queryParams {
		u.Query().Set(k, v)
	}

	var body io.Reader
	if reqBody != nil {
		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(reqBody); err != nil {
			return nil, errors.Wrap(err, 0)
		}
		body = &buf
	}

	req, err := http.NewRequest(method, u.String(), body)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.accessToken))

	return req, nil
}

func (s *SpotifyClient) get(path string, queryParams map[string]string, optional bool, data interface{}) (*http.Response, error) {
	req, err := s.newRequest(http.MethodGet, path, queryParams, nil)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	glog.Infof("Spotify GET: %v", req.URL.String())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	if resp.StatusCode == http.StatusNotFound {
		if !optional {
			return nil, &NotFoundError{url: req.URL}
		}
	} else if resp.StatusCode == http.StatusOK {
		if err := json.NewDecoder(resp.Body).Decode(data); err != nil {
			return nil, errors.WrapPrefix(err, "Unable to decode body", 0)
		}
	} else {
		return nil, errors.Errorf("Unexpected status code %d", resp.StatusCode)
	}

	return resp, nil
}

func (s *SpotifyClient) post(path string, reqBody, respData interface{}) (*http.Response, error) {
	req, err := s.newRequest(http.MethodPost, path, nil, reqBody)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	glog.Infof("Spotify POST: %v `%v`", req.URL.String(), reqBody)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, errors.Errorf("Unexpected status code %d", resp.StatusCode)
	}

	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(respData); err != nil {
		return nil, errors.WrapPrefix(err, "Unable to decode response", 0)
	}

	return resp, nil
}

func (s *SpotifyClient) put(path string, queryParams map[string]string, reqBody interface{}) (*http.Response, error) {
	req, err := s.newRequest(http.MethodPut, path, nil, reqBody)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	glog.Infof("Spotify PUT: %v `%v`", req.URL.String(), reqBody)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, errors.Errorf("Unexpected status code %d", resp.StatusCode)
	}

	return resp, nil
}
