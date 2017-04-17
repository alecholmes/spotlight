package controllers

import (
	"net/http"

	"github.com/alecholmes/spotlight/app/templates"

	"github.com/go-errors/errors"
	"github.com/golang/glog"
)

func Render404(rw http.ResponseWriter, req *http.Request) {
	glog.Infof("Page not found. URL=`%v`", req.URL)

	rw.WriteHeader(http.StatusNotFound)

	if err := templates.Error404.Execute(rw, nil); err != nil {
		glog.Errorf("Error rendering 404 template: %v", err)
	}
}

func Render500(rw http.ResponseWriter, err error) {
	if stackErr, ok := err.(*errors.Error); ok {
		glog.Errorf(stackErr.ErrorStack())
	} else {
		glog.Error(err)
	}

	if err := templates.Error500.Execute(rw, nil); err != nil {
		glog.Errorf("Error rendering 500 template: %v", err)
	}
}
