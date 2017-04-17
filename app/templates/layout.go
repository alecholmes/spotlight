package templates

type LayoutData struct {
	SignedIn bool
}

var PageLayout = parse("layout")
