/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"
)

// maxLogBodyBytes bounds how much of a request/response body is retained for
// debug logging. Bodies can be attacker-controlled and arbitrarily large (and
// may carry sensitive data), so we never buffer or emit more than this; the log
// is a truncated preview, not a verbatim copy.
const maxLogBodyBytes = 4096

// responseWriter wraps http.ResponseWriter to capture the status code and, when
// body capture is enabled, a bounded prefix of the response body for logging.
type responseWriter struct {
	http.ResponseWriter
	statusCode  int
	captureBody bool
	body        bytes.Buffer
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(p []byte) (int, error) {
	// Only buffer a bounded prefix, and only when the caller intends to log it.
	if rw.captureBody {
		if remaining := maxLogBodyBytes - rw.body.Len(); remaining > 0 {
			if len(p) < remaining {
				remaining = len(p)
			}
			rw.body.Write(p[:remaining])
		}
	}
	return rw.ResponseWriter.Write(p)
}

// withLogging wraps the handler with request logging
func (s *Server) withLogging(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.verbose == 0 {
			handler.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		captureBody := s.verbose >= 2
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK, captureBody: captureBody}

		// At highest verbosity, capture a bounded prefix of the request body for
		// logging while streaming the full body through to downstream handlers.
		var reqBodyData []byte
		if captureBody && r.Body != nil {
			preview, _ := io.ReadAll(io.LimitReader(r.Body, maxLogBodyBytes))
			// Restore the body (preview + untouched remainder) for handlers.
			r.Body = io.NopCloser(io.MultiReader(bytes.NewReader(preview), r.Body))
			reqBodyData = preview
		}

		handler.ServeHTTP(wrapped, r)

		duration := time.Since(start)
		log.Printf("%s %s -> %d in %.1fms",
			r.Method,
			r.URL.Path,
			wrapped.statusCode,
			float64(duration.Microseconds())/1000.0,
		)

		if s.verbose >= 2 {
			if len(reqBodyData) > 0 {
				log.Printf("Request body:%s", formatMaybeJSON(reqBodyData))
			}

			respBody := wrapped.body.Bytes()
			if len(respBody) > 0 {
				log.Printf("Response body:%s", formatMaybeJSON(respBody))
			}
		}
	})
}

func formatMaybeJSON(data []byte) string {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return ""
	}
	if trimmed[0] == '{' || trimmed[0] == '[' {
		var v any
		if err := json.Unmarshal(trimmed, &v); err == nil {
			pretty, err := json.MarshalIndent(v, "", "  ")
			if err == nil {
				return "\n" + string(pretty)
			}
		}
	}
	return " " + string(data)
}
