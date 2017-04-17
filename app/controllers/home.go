package controllers

import (
	"net/http"

	"github.com/alecholmes/spotlight/app/oauth"
	"github.com/alecholmes/spotlight/app/requests"
	"github.com/alecholmes/spotlight/app/templates"

	"github.com/golang/glog"
	"github.com/gorilla/mux"
)

type Home struct {
	sessions *requests.Sessions
}

func NewHome(sessions *requests.Sessions) *Home {
	return &Home{sessions: sessions}
}

func (h *Home) BindToMux(mux *mux.Router, oauth *oauth.OAuth, errorHandler func(http.ResponseWriter, error)) {
	mux.HandleFunc("/",
		requests.WithContext(oauth.OptionallyAuthed(h.View, errorHandler))).
		Methods(http.MethodGet)

	mux.HandleFunc("/logout",
		requests.WithContext(h.Logout)).
		Methods(http.MethodGet)
}

func (h *Home) View(rw http.ResponseWriter, req *http.Request) {
	user := requests.UserFromContext(req.Context())
	if user != nil {
		rw.Header().Set("Location", "/subscriptions")
		rw.WriteHeader(http.StatusFound)
		return
	}

	if err := templates.HomeView.Execute(rw, &templates.LayoutData{SignedIn: false}); err != nil {
		glog.Errorf("Unable to render template: %v", err)
	}
}

func (h *Home) Logout(rw http.ResponseWriter, req *http.Request) {
	if session, err := h.sessions.GetSession(req); err != nil {
		glog.Errorf("Error decoding session: %v", err)
	} else {
		session.Delete(req, rw)
	}

	rw.Header().Set("Location", "/")
	rw.WriteHeader(http.StatusFound)
}
