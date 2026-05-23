package pirca

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type Context struct {
	Writer  http.ResponseWriter
	Request *http.Request

	mu   sync.RWMutex
	keys map[any]any

	sameSite http.SameSite
}

func (c *Context) Status(code int) {
	c.Writer.WriteHeader(code)
}

func (c *Context) GetStatus() int {
	return c.Writer.(*responseWriter).statusCode
}

func (c *Context) Header(key, value string) {
	if value == "" {
		c.Writer.Header().Del(key)
		return
	}
	c.Writer.Header().Set(key, value)
}

func (c *Context) GetHeader(key string) string {
	return c.Request.Header.Get(key)
}

func (c *Context) GetRawData() ([]byte, error) {
	if c.Request.Body == nil {
		return nil, errors.New("cannot read nil body")
	}
	return io.ReadAll(c.Request.Body)
}

func (c *Context) SetSameSite(samesite http.SameSite) {
	c.sameSite = samesite
}

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

func (c *Context) SetCookieData(cookie *http.Cookie) {
	if cookie.Path == "" {
		cookie.Path = "/"
	}
	if cookie.SameSite == http.SameSiteDefaultMode {
		cookie.SameSite = c.sameSite
	}
	http.SetCookie(c.Writer, cookie)
}

func (c *Context) Cookie(name string) (string, error) {
	cookie, err := c.Request.Cookie(name)
	if err != nil {
		return "", err
	}
	val, _ := url.QueryUnescape(cookie.Value)
	return val, nil
}

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

func (c *Context) Write(code int, message string) error {
	c.Status(code)
	_, err := c.Writer.Write([]byte(message))
	return err
}

func (c *Context) Redirect(code int, location string) {
	if (code < http.StatusMultipleChoices || code > http.StatusPermanentRedirect) && code != http.StatusCreated {
		panic(fmt.Sprintf("cannot redirect with status code %d", code))
	}
	http.Redirect(c.Writer, c.Request, location, code)
}

func (c *Context) File(filepath string) {
	http.ServeFile(c.Writer, c.Request, filepath)
}

func (c *Context) FileFromFS(filepath string, fs http.FileSystem) {
	defer func(old string) {
		c.Request.URL.Path = old
	}(c.Request.URL.Path)

	c.Request.URL.Path = filepath
	http.FileServer(fs).ServeHTTP(c.Writer, c.Request)
}

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

func (c *Context) FileAttachment(filepath, filename string) {
	if isASCII(filename) {
		escapeQuotes := quoteEscaper.Replace(filename)
		c.Writer.Header().Set("Content-Disposition", `attachment; filename="`+escapeQuotes+`"`)
	} else {
		c.Writer.Header().Set("Content-Disposition", `attachment; filename*=UTF-8''`+url.QueryEscape(filename))
	}
	http.ServeFile(c.Writer, c.Request, filepath)
}

func (c *Context) BytesWritten() int {
	return c.Writer.(*responseWriter).bytesWritten
}

func (c *Context) Set(key any, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.keys == nil {
		c.keys = make(map[any]any)
	}
	c.keys[key] = value
}

func (c *Context) Get(key any) (value any, exists bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	value, exists = c.keys[key]
	return
}

func (c *Context) Deadline() (deadline time.Time, ok bool) {
	return c.Request.Context().Deadline()
}

func (c *Context) Done() <-chan struct{} {
	return c.Request.Context().Done()
}

func (c *Context) Err() error {
	return c.Request.Context().Err()
}

func (c *Context) Value(key any) any {
	if key == 0 {
		return c.Request
	}
	if keyAsString, ok := key.(string); ok {
		if val, exists := c.keys[keyAsString]; exists {
			return val
		}
	}
	return c.Request.Context().Value(key)
}
