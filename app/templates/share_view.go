package templates

// TODO: include playlist URL
type ShareViewData struct {
	LayoutData
	InviterName  string
	InviterEmail string
	PlaylistName string
	SubscribeURL string
}

var ShareView = extend(PageLayout, "share_view")
