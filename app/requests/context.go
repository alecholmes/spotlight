package requests

import (
	"context"
	"net/http"

	"github.com/alecholmes/spotlight/app/model"

	"github.com/gorilla/mux"
)

type ContextPathVars struct{}
type ContextUser struct{}

func WithContext(handler http.HandlerFunc) func(rw http.ResponseWriter, req *http.Request) {
	return func(rw http.ResponseWriter, req *http.Request) {
		handler(rw, req.WithContext(context.WithValue(req.Context(), ContextPathVars{}, mux.Vars(req))))
	}
}

func UserFromContext(ctx context.Context) *model.User {
	user := ctx.Value(ContextUser{})
	if user == nil {
		return nil
	}

	return user.(*model.User)
}

func MustUserFromContext(ctx context.Context) *model.User {
	user := UserFromContext(ctx)
	if user == nil {
		panic("Expected user in request context")
	}

	return user
}

func MustPathVarsFromContext(ctx context.Context) map[string]string {
	vars := ctx.Value(ContextPathVars{})
	if vars == nil {
		panic("Expected path vars in request context")
	}

	return vars.(map[string]string)
}
