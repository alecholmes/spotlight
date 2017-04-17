package spotify

type playlistLookup struct {
	userID     string
	playlistID string
}

type CachingClient struct {
	client    *SpotifyClient
	profiles  map[string]*PublicProfile
	playlists map[playlistLookup]*Playlist
}

func NewCachingClient(client *SpotifyClient) *CachingClient {
	return &CachingClient{
		client:    client,
		profiles:  make(map[string]*PublicProfile),
		playlists: make(map[playlistLookup]*Playlist),
	}
}

func (c *CachingClient) GetProfile(userID string) (*PublicProfile, error) {
	if profile, ok := c.profiles[userID]; ok {
		return profile, nil
	} else if profile, err := c.client.GetProfile(userID); err != nil {
		return nil, err
	} else {
		c.profiles[userID] = profile
		return profile, nil
	}
}

func (c *CachingClient) GetPlaylist(userID, playlistID string) (*Playlist, error) {
	lookup := playlistLookup{userID: userID, playlistID: playlistID}
	if playlist, ok := c.playlists[lookup]; ok {
		return playlist, nil
	} else if playlist, err := c.client.GetPlaylist(userID, playlistID); err != nil {
		return nil, err
	} else {
		c.playlists[lookup] = playlist
		return playlist, nil
	}
}
