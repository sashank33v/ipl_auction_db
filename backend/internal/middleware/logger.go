package middleware

import (
	"log"
	"net/http"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		ww := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)

		next.ServeHTTP(ww, r)

		duration := time.Since(start)

		log.Printf(
			"Method=%s Path=%s Status=%d Duration=%s",
			r.Method,
			r.URL.Path,
			ww.Status(),
			duration,
		)
	})
}
