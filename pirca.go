// Package pirca provides a lightweight HTTP middleware that enhances
// net/http with a rich Context, response helpers, and request utilities
// without replacing the standard library.
package pirca

import (
	"net/http"
)

// responseWriter wraps http.ResponseWriter to capture the status code
// and the number of bytes written during the request lifecycle.
type responseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

var _ http.ResponseWriter = (*responseWriter)(nil)

// Write writes the data to the connection and tracks the number of bytes written.
func (r *responseWriter) Write(b []byte) (n int, err error) {
	n, err = r.ResponseWriter.Write(b)
	r.bytesWritten += n
	return
}

// WriteHeader sends an HTTP response header with the provided status code
// and stores it for later retrieval via Context.GetStatus.
func (r *responseWriter) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

// Wrap returns an HTTP handler that initializes a pirca Context for each
// incoming request and makes it available via Ctx. It must wrap the
// outermost handler in the middleware chain.
//
// Example:
//
//	mux := http.NewServeMux()
//	http.ListenAndServe(":8080", pirca.Wrap(mux))
func Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		ctx := &Context{
			Writer:  rw,
			Request: r,
		}
		r = r.WithContext(ctx)
		next.ServeHTTP(rw, r)
	})
}

// Ctx retrieves the pirca Context from the request. It must be called
// within a handler wrapped by Wrap, otherwise it will panic.
//
// Example:
//
//	mux.HandleFunc("GET /ip/{ip}", func(w http.ResponseWriter, r *http.Request) {
//	    ctx := pirca.Ctx(r)
//	    _ = ctx.JSON(http.StatusOK, map[string]string{"msg": "hello, world!"})
//	}
func Ctx(r *http.Request) *Context {
	return r.Context().(*Context)
}
