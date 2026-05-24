package pirca

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const BodyBytesKey = "_pirca_body_bytes"

// Context holds the request and response state for a single HTTP request.
// It implements context.Context, so it can be passed directly to any function
// that accepts a context, such as database drivers, HTTP clients, or tracers.
//
// Context is created by Wrap for each incoming request and retrieved via Ctx.
type Context struct {
	// Writer is the underlying http.ResponseWriter, wrapped to capture
	// status code and bytes written. It can be used directly if needed.
	Writer http.ResponseWriter

	// Request is the incoming HTTP request. It can be used directly
	// alongside Context methods interchangeably.
	Request *http.Request

	mu       sync.RWMutex
	keys     map[any]any
	sameSite http.SameSite
	config   *Config
}

var _ context.Context = (*Context)(nil)

// FormValue returns the value of a form field from the request body.
// Works with application/x-www-form-urlencoded and multipart/form-data.
// For query params use Query instead.
func (c *Context) FormValue(key string) string {
	return c.Request.PostFormValue(key)
}

// DefaultFormValue returns the value of a form field from the request body,
// or defaultValue if the key is not present.
func (c *Context) DefaultFormValue(key, defaultValue string) string {
	if val := c.FormValue(key); val != "" {
		return val
	}
	return defaultValue
}

// FormFile returns the first file uploaded with the given form key.
// Parses the multipart form if not already parsed, using MaxMultipartMemory from Config.
//
// Use SaveUploadedFile to save the file to disk, or call file.Open()
// to read its contents directly.
//
// Example:
//
//	file, err := ctx.FormFile("avatar")
//	if err != nil {
//		_ = ctx.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
//		return
//	}
//	ctx.SaveUploadedFile(file, "./uploads/"+file.Filename)
func (c *Context) FormFile(name string) (*multipart.FileHeader, error) {
	if c.Request.MultipartForm == nil {
		if err := c.Request.ParseMultipartForm(c.config.MaxMultipartMemory); err != nil {
			return nil, err
		}
	}
	f, fh, err := c.Request.FormFile(name)
	if err != nil {
		return nil, err
	}
	f.Close()
	return fh, nil
}

// MultipartForm parses and returns the full multipart form data, including
// all fields and uploaded files. Uses MaxMultipartMemory from Config.
//
// Use FormFile for single file uploads. Use MultipartForm when you need
// access to multiple files or all form fields at once.
//
// Note: if reading file contents manually via file.Open(), always close
// the returned reader after use.
//
// Example:
//
//	form, err := ctx.MultipartForm()
//	if err != nil {
//		_ = ctx.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
//		return
//	}
//	files := form.File["avatars"]
//	for _, f := range files {
//		ctx.SaveUploadedFile(f, "./uploads/"+f.Filename)
//	}
func (c *Context) MultipartForm() (*multipart.Form, error) {
	err := c.Request.ParseMultipartForm(c.config.MaxMultipartMemory)
	return c.Request.MultipartForm, err
}

// SaveUploadedFile saves a multipart file to the given destination path.
// Creates any necessary parent directories automatically.
// The optional perm parameter sets the directory permissions, defaulting to 0o750.
//
// The uploaded file is opened and closed internally — the caller does not
// need to manage the file lifecycle.
//
// Example:
//
//	file, _ := ctx.FormFile("avatar")
//	if err := ctx.SaveUploadedFile(file, "./uploads/"+file.Filename); err != nil {
//		_ = ctx.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
//		return
//	}
func (c *Context) SaveUploadedFile(file *multipart.FileHeader, dst string, perm ...fs.FileMode) error {
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	var mode os.FileMode = 0o750
	if len(perm) > 0 {
		mode = perm[0]
	}
	dir := filepath.Dir(dst)
	if err = os.MkdirAll(dir, mode); err != nil {
		return err
	}
	if err = os.Chmod(dir, mode); err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, src)
	return err
}

// BindJSON decodes the request body as JSON into obj.
// Returns an error if the body is nil, the JSON is malformed,
// or the types are incompatible with obj.
//
// The body can only be read once. Use BindJSONWith if the body
// needs to be read multiple times across middlewares and handlers.
//
// Example:
//
//	var payload MyStruct
//	if err := ctx.BindJSON(&payload); err != nil {
//		_ = ctx.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
//		return
//	}
func (c *Context) BindJSON(obj any) error {
	if c.Request.Body == nil {
		return errors.New("invalid request")
	}
	return json.NewDecoder(c.Request.Body).Decode(obj)
}

// BindJSONStrict is like BindJSON but returns an error if the request body
// contains fields that are not present in obj.
// Useful for strict API validation where unknown fields should be rejected.
//
// Example:
//
//	var payload MyStruct
//	if err := ctx.BindJSONStrict(&payload); err != nil {
//		_ = ctx.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
//		return
//	}
func (c *Context) BindJSONStrict(obj any) error {
	if c.Request.Body == nil {
		return errors.New("invalid request")
	}
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(obj)
}

// BindXML decodes the request body as XML into obj.
// Returns an error if the body is nil, the XML is malformed,
// or the types are incompatible with obj.
//
// The body can only be read once. Use BindXMLWith if the body
// needs to be read multiple times across middlewares and handlers.
//
// Example:
//
//	var payload MyStruct
//	if err := ctx.BindXML(&payload); err != nil {
//		_ = ctx.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
//		return
//	}
func (c *Context) BindXML(obj any) error {
	if c.Request.Body == nil {
		return errors.New("invalid request")
	}
	return xml.NewDecoder(c.Request.Body).Decode(obj)
}

// BindBodyWith reads the request body once and caches it in the Context store,
// allowing subsequent calls to reuse the same body bytes across middlewares
// and handlers without hitting EOF.
//
// bind is a function that deserializes the body bytes into obj.
// Use BindJSONWith or BindXMLWith for the most common cases.
//
// Example:
//
//	ctx.BindBodyWith(&obj, func(b []byte, v any) error {
//		return json.Unmarshal(b, v)
//	})
func (c *Context) BindBodyWith(obj any, bind func(b []byte, obj any) error) error {
	var body []byte
	if cb, ok := c.Get(BodyBytesKey); ok {
		body = cb.([]byte)
	} else {
		var err error
		body, err = io.ReadAll(c.Request.Body)
		if err != nil {
			return err
		}
		c.Set(BodyBytesKey, body)
	}
	return bind(body, obj)
}

// BindJSONWith decodes the request body as JSON into obj, caching the body
// so it can be read multiple times. See BindBodyWith for details.
//
// Example:
//
//	var payload MyStruct
//	if err := ctx.BindJSONWith(&payload); err != nil {
//		_ = ctx.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
//		return
//	}
func (c *Context) BindJSONWith(obj any) error {
	return c.BindBodyWith(obj, json.Unmarshal)
}

// BindJSONStrictWith is like BindJSONWith but returns an error if the request
// body contains fields not present in obj. The body is cached for reuse.
//
// Example:
//
//	var payload MyStruct
//	if err := ctx.BindJSONStrictWith(&payload); err != nil {
//		_ = ctx.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
//		return
//	}
func (c *Context) BindJSONStrictWith(obj any) error {
	return c.BindBodyWith(obj, func(b []byte, obj any) error {
		decoder := json.NewDecoder(bytes.NewReader(b))
		decoder.DisallowUnknownFields()
		return decoder.Decode(obj)
	})
}

// BindXMLWith decodes the request body as XML into obj, caching the body
// so it can be read multiple times. See BindBodyWith for details.
//
// Example:
//
//	var payload MyStruct
//	if err := ctx.BindXMLWith(&payload); err != nil {
//		_ = ctx.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
//		return
//	}
func (c *Context) BindXMLWith(obj any) error {
	return c.BindBodyWith(obj, xml.Unmarshal)
}

// Status writes the HTTP status code to the response header.
// Must be called before writing the response body.
func (c *Context) Status(code int) {
	c.Writer.WriteHeader(code)
}

// Header sets a response header key to value.
// If value is empty, the header is deleted.
func (c *Context) Header(key, value string) {
	if value == "" {
		c.Writer.Header().Del(key)
		return
	}
	c.Writer.Header().Set(key, value)
}

// GetHeader returns the value of the request header with the given key.
func (c *Context) GetHeader(key string) string {
	return c.Request.Header.Get(key)
}

// GetRawData reads and returns the raw request body as bytes.
// Returns an error if the body is nil.
func (c *Context) GetRawData() ([]byte, error) {
	if c.Request.Body == nil {
		return nil, errors.New("cannot read nil body")
	}
	return io.ReadAll(c.Request.Body)
}

// SetSameSite sets the SameSite attribute used for cookies set via
// SetCookie. Defaults to http.SameSiteDefaultMode if not called.
func (c *Context) SetSameSite(samesite http.SameSite) {
	c.sameSite = samesite
}

// SetCookie writes a Set-Cookie header to the response.
// The value is automatically URL-encoded and decoded by Cookie.
// If path is empty, it defaults to "/".
func (c *Context) SetCookie(name, value string, maxAge int, path, domain string, secure, httpOnly bool) {
	if path == "" {
		path = "/"
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    url.QueryEscape(value),
		MaxAge:   maxAge,
		Path:     path,
		Domain:   domain,
		SameSite: c.sameSite,
		Secure:   secure,
		HttpOnly: httpOnly,
	})
}

// SetCookieData writes a Set-Cookie header using a pre-built http.Cookie.
// If Path is empty, it defaults to "/".
// If SameSite is http.SameSiteDefaultMode, it uses the value set by SetSameSite.
func (c *Context) SetCookieData(cookie *http.Cookie) {
	if cookie.Path == "" {
		cookie.Path = "/"
	}
	if cookie.SameSite == http.SameSiteDefaultMode {
		cookie.SameSite = c.sameSite
	}
	http.SetCookie(c.Writer, cookie)
}

// Cookie returns the value of the named cookie from the request.
// The value is automatically URL-decoded to reverse the encoding applied by SetCookie.
// Returns http.ErrNoCookie if the cookie is not present.
func (c *Context) Cookie(name string) (string, error) {
	cookie, err := c.Request.Cookie(name)
	if err != nil {
		return "", err
	}
	val, _ := url.QueryUnescape(cookie.Value)
	return val, nil
}

// JSON serializes obj as JSON and writes it to the response with the given
// status code. Sets Content-Type to "application/json".
//
// JSON uses encoding/json Marshal, which means it will fail for types that
// cannot be serialized such as channels, functions, or circular references.
// For well-defined structs and maps with basic types, it will never fail.
//
// Example:
//
//	_ = ctx.JSON(http.StatusOK, map[string]string{"message": "ok"})
func (c *Context) JSON(code int, obj any) error {
	jsonBytes, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Status(code)
	_, err = c.Writer.Write(jsonBytes)
	return err
}

// XML serializes obj as XML and writes it to the response with the given
// status code. Sets Content-Type to "application/xml".
//
// Example:
//
//	_ = ctx.XML(http.StatusOK, myStruct)
func (c *Context) XML(code int, obj any) error {
	xmlBytes, err := xml.Marshal(obj)
	if err != nil {
		return err
	}
	c.Writer.Header().Set("Content-Type", "application/xml")
	c.Status(code)
	_, err = c.Writer.Write(xmlBytes)
	return err
}

// String writes a plain string message to the response with the given status code.
// Unlike JSON or XML, it does not set a Content-Type header — the caller is
// responsible for setting it beforehand if needed.
//
// Example:
//
//	ctx.Header("Content-Type", "text/html; charset=utf-8")
//	_ = ctx.String(http.StatusOK, "<h1>Hello</h1>")
func (c *Context) String(code int, message string) (err error) {
	c.Status(code)
	_, err = c.Writer.Write([]byte(message))
	return
}

// Data writes raw bytes to the response with the given status code.
// Set the Content-Type header beforehand with Header if needed.
//
// Example:
//
//	ctx.Header("Content-Type", "application/pdf")
//	ctx.Data(http.StatusOK, pdfBytes)
func (c *Context) Data(code int, data []byte) (err error) {
	c.Status(code)
	_, err = c.Writer.Write(data)
	return
}

// Redirect replies to the request with a redirect to the given location.
// The code must be a valid HTTP redirect status code (301-308) or 201.
//
// Panics if an invalid status code is provided, as this indicates a bug
// in the caller's code rather than a runtime error.
func (c *Context) Redirect(code int, location string) {
	if (code < http.StatusMultipleChoices || code > http.StatusPermanentRedirect) && code != http.StatusCreated {
		panic(fmt.Sprintf("cannot redirect with status code %d", code))
	}
	http.Redirect(c.Writer, c.Request, location, code)
}

// File serves the file at the given filepath using http.ServeFile.
// It handles Range requests, ETags, and Last-Modified headers automatically.
func (c *Context) File(filepath string) {
	http.ServeFile(c.Writer, c.Request, filepath)
}

// FileFromFS serves a file from the given http.FileSystem at the given filepath.
// Unlike File, it allows serving from any FileSystem implementation, including
// embedded files via embed.FS.
//
// Example:
//
//	//go:embed static
//	var staticFiles embed.FS
//
//	ctx.FileFromFS("static/style.css", http.FS(staticFiles))
func (c *Context) FileFromFS(filepath string, fs http.FileSystem) {
	defer func(old string) {
		c.Request.URL.Path = old
	}(c.Request.URL.Path)

	c.Request.URL.Path = filepath
	http.FileServer(fs).ServeHTTP(c.Writer, c.Request)
}

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

// FileAttachment serves the file at filepath as a downloadable attachment.
// The filename parameter sets the suggested filename in the browser's save dialog.
//
// ASCII filenames are quoted and escaped per RFC 2183.
// Non-ASCII filenames are encoded using RFC 5987 (UTF-8 with URL encoding)
// to support characters such as accents, ñ, or CJK characters.
//
// Example:
//
//	ctx.FileAttachment("./files/report.pdf", "reporte_2026.pdf")
//	ctx.FileAttachment("./files/report.pdf", "reporte_año_2026.pdf")
func (c *Context) FileAttachment(filepath, filename string) {
	if isASCII(filename) {
		escapeQuotes := quoteEscaper.Replace(filename)
		c.Writer.Header().Set("Content-Disposition", `attachment; filename="`+escapeQuotes+`"`)
	} else {
		c.Writer.Header().Set("Content-Disposition", `attachment; filename*=UTF-8''`+url.QueryEscape(filename))
	}
	http.ServeFile(c.Writer, c.Request, filepath)
}

// GetStatus returns the HTTP status code written to the response.
func (c *Context) GetStatus() int {
	return c.Writer.(*responseWriter).statusCode
}

// BytesWritten returns the total number of bytes written to the response body.
func (c *Context) BytesWritten() int {
	return c.Writer.(*responseWriter).bytesWritten
}

// Set stores a key-value pair in the Context, making it available to
// subsequent handlers and middlewares within the same request lifecycle.
// It is safe for concurrent use.
//
// Example:
//
//	ctx.Set("userID", "123")
//	ctx.Set("role", "admin")
func (c *Context) Set(key any, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.keys == nil {
		c.keys = make(map[any]any)
	}
	c.keys[key] = value
}

// Get retrieves a value previously stored with Set.
// Returns the value and true if the key exists, or nil and false otherwise.
// It is safe for concurrent use.
//
// Example:
//
//	if userID, ok := ctx.Get("userID"); ok {
//		fmt.Println(userID)
//	}
func (c *Context) Get(key any) (value any, exists bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	value, exists = c.keys[key]
	return
}

func (c *Context) Delete(key any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.keys != nil {
		delete(c.keys, key)
	}
}

// Deadline implements context.Context. It delegates to the underlying
// request context, which is canceled when the client disconnects.
func (c *Context) Deadline() (deadline time.Time, ok bool) {
	return c.Request.Context().Deadline()
}

// Done implements context.Context. The returned channel is closed when the
// request context is canceled — typically when the client disconnects.
func (c *Context) Done() <-chan struct{} {
	return c.Request.Context().Done()
}

// Err implements context.Context. It returns the error from the underlying
// request context, or nil if the context has not been canceled.
func (c *Context) Err() error {
	return c.Request.Context().Err()
}

// Value implements context.Context. It looks up key in the following order:
//  1. If key is a string, searches in the keys stored via Set.
//  2. Delegates to the underlying request context.
//
// This allows *Context to be passed directly to any function that accepts
// a context.Context, including database drivers, HTTP clients, and tracers.
//
// Note: only string keys are searched in the local store. Non-string keys
// are delegated to the request context, where external libraries store
// their own values.
func (c *Context) Value(key any) any {
	if keyAsString, ok := key.(string); ok {
		if val, exists := c.Get(keyAsString); exists {
			return val
		}
	}
	return c.Request.Context().Value(key)
}
