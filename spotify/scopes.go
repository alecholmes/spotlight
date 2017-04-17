package spotify

type Scope string

const (
	ScopePlaylistReadCollaborative Scope = "playlist-read-collaborative"
	ScopePlaylistModifyPublic            = "playlist-modify-public"
	ScopePlaylistReadPrivate             = "playlist-read-private"
	ScopePlaylistModifyPrivate           = "playlist-modify-private"
	ScopeUserReadPrivate                 = "user-read-private"
	ScopeUserReadEmail                   = "user-read-email"
	ScopeUserFollowRead                  = "user-follow-read"
	ScopeUserFollowModify                = "user-follow-modify"
)
