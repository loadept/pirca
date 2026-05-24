package pirca

import "net/http"

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
