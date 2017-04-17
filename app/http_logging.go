package app

import (
	"net/http"
	"time"

	"github.com/golang/glog"
)

type wrappedResponseWriter struct {
	http.ResponseWriter
	status int
}

var _ http.ResponseWriter = &wrappedResponseWriter{}

func (w *wrappedResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func loggingHandler(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		wrappedRW := &wrappedResponseWriter{ResponseWriter: rw}
		start := time.Now()

		defer func() {
			status := wrappedRW.status
			if status == 0 {
				status = http.StatusOK
			}

			glog.Infof("Handled HTTP request. method=%s url=%v status=%d elapsedMs=%d",
				req.Method, req.URL, status, int(time.Since(start)/time.Millisecond))
		}()

		handler.ServeHTTP(wrappedRW, req)
	})
}
