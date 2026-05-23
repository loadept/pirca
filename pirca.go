package pirca

import (
	"net/http"
)

type responseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func (r *responseWriter) Write(b []byte) (n int, err error) {
	n, err = r.ResponseWriter.Write(b)
	r.bytesWritten += n
	return
}

func (r *responseWriter) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func Pirca(next http.Handler) http.Handler {
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

func PircaCtx(r *http.Request) *Context {
	return r.Context().(*Context)
}
