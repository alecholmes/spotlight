package spotify

import (
	"sort"
)

// PlaylistTrackIDs returns all track IDs in a playlist, lexically sorted.
func PlaylistTrackIDs(playlist *Playlist) []string {
	trackIDs := make([]string, len(playlist.PlaylistTracks))
	for i, track := range playlist.PlaylistTracks {
		trackIDs[i] = track.Track.ID
	}
	sort.Strings(trackIDs)

	return trackIDs
}
