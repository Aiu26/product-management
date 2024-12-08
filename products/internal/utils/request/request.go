package request

import (
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

func Timer(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logrus.Info(r.Method, r.URL.Path)
		start := time.Now()
		h(w, r)
		logrus.Infof("Request took %s", time.Since(start))
	}
}