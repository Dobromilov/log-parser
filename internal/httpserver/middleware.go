package httpserver

import (
	"log/slog"
	"net/http"
	"time"
)

type responseWriter struct {
	http.ResponseWriter
	statusCode int
	bytes      int
}

func (w *responseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *responseWriter) Write(data []byte) (int, error) {
	if w.statusCode == 0 {
		w.WriteHeader(http.StatusOK)
	}

	n, err := w.ResponseWriter.Write(data)
	w.bytes += n
	return n, err
}

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		writer := &responseWriter{ResponseWriter: w}

		next.ServeHTTP(writer, r)

		statusCode := writer.statusCode
		if statusCode == 0 {
			statusCode = http.StatusOK
		}

		slog.Info("http request completed",
			"method", r.Method,
			"path", r.URL.Path,
			"status", statusCode,
			"bytes", writer.bytes,
			"duration_ms", time.Since(startedAt).Milliseconds(),
			"remote_addr", r.RemoteAddr,
		)
	})
}
