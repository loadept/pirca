// Package pirca provides a lightweight HTTP middleware that enhances
// net/http with a rich Context, response helpers, and request utilities
// without replacing the standard library.
package pirca

import (
	"net/http"
)

const (
	defaultMultipartMemory = 32 << 20 // 32 MB
	defaultMaxBodySize     = 0
)

// Config holds the configuration for the Pirca middleware.
// All fields are optional — unset fields use their default values.
//
// Defaults:
//
//	MaxBodySize:        0 (no limit)
//	MaxMultipartMemory: 32MB
type Config struct {
	// MaxBodySize sets the maximum size of the request body in bytes.
	// Requests exceeding this limit will return an error on read.
	// Defaults to 0 (no limit).
	MaxBodySize int64

	// MaxMultipartMemory sets the maximum memory used when parsing multipart forms.
	// The rest is written to temporary files on disk.
	// Defaults to 32MB.
	MaxMultipartMemory int64
}

// New returns an HTTP middleware that initializes a pirca Context for each
// incoming request and makes it available via Ctx. It must wrap the
// outermost handler in the middleware chain.
// Optionally accepts a *Config to override default settings.
//
// Example:
//
//	mux := http.NewServeMux()
//
//	// With defaults
//	http.ListenAndServe(":8080", pirca.New()(mux))
//
//	// With custom config
//	http.ListenAndServe(":8080", pirca.New(&pirca.Config{
//	    MaxBodySize: 1 << 20,
//	})(mux))
func New(cfg ...*Config) func(http.Handler) http.Handler {
	conf := &Config{
		MaxMultipartMemory: defaultMultipartMemory,
		MaxBodySize:        defaultMaxBodySize,
	}
	if len(cfg) > 0 && cfg[0] != nil {
		if cfg[0].MaxMultipartMemory > 0 {
			conf.MaxMultipartMemory = cfg[0].MaxMultipartMemory
		}
		if cfg[0].MaxBodySize > 0 {
			conf.MaxBodySize = cfg[0].MaxBodySize
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if conf.MaxBodySize > 0 {
				r.Body = http.MaxBytesReader(w, r.Body, conf.MaxBodySize)
			}
			rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			ctx := &Context{
				Writer:    rw,
				Request:   r,
				parentCtx: r.Context(),
				config:    conf,
			}
			r = r.WithContext(ctx)
			next.ServeHTTP(rw, r)
		})
	}
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
