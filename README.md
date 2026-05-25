# Pirca

**Pirca** is a lightweight HTTP middleware for Go that extends `net/http` with a rich `Context`, response helpers, request binders, form/file handling, and more — without replacing the standard library or adding external dependencies.

Built on patterns from **Gin**, Pirca follows the same conventions and logic, adapted to work directly with `net/http`. Many methods mirror Gin's API, making it familiar if you've used Gin before, but without the framework lock-in.

- **Zero dependencies** — only the Go standard library.
- **Based on Gin** — same patterns, same feel, no framework.
- **Full control** — `ctx.Request` and `ctx.Writer` are the original `*http.Request` and `http.ResponseWriter`. Use them directly whenever you need.
- **Implements `context.Context`** — pass `ctx` directly to databases, HTTP clients, tracers, etc.
- **Captures status code and bytes written** — perfect for logging, metrics, and observability middlewares.
- **Accelerates development** — JSON/XML binders, response writers, file uploads, cookies, query params, form values — all ready to use.

## Requirements

- Go 1.22 or later

## Installation

```bash
go get github.com/loadept/pirca
```

## Quick start

```go
package main

import (
    "log"
    "net/http"

    "github.com/loadept/pirca"
)

func main() {
    mux := http.NewServeMux()
    handler := pirca.New()(mux)

    mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
        ctx := pirca.Ctx(r)

        _ = ctx.JSON(http.StatusOK, map[string]string{
            "message": "Hello world",
        })
    })

    log.Fatal(http.ListenAndServe(":8080", handler))
}
```

### How it works

1. `pirca.New()` returns a middleware that creates a `Context` per request.
2. Inside the handler, `pirca.Ctx(r)` retrieves the `Context` from the request.
3. Use `ctx` to bind bodies, write responses, handle cookies, query params, forms, files, and more.

## Core API

### `New(cfg ...*Config) func(http.Handler) http.Handler`

Creates the middleware. Must be the outermost layer in your handler chain. Accepts an optional `*Config`.

```go
// Defaults
handler := pirca.New()(mux)

// With custom config
handler := pirca.New(&pirca.Config{
    MaxBodySize:        1 << 20,       // 1MB max body
    MaxMultipartMemory: 64 << 20,      // 64MB for multipart
})(mux)
```

### `Config`

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `MaxBodySize` | `int64` | `0` (no limit) | Max request body size in bytes |
| `MaxMultipartMemory` | `int64` | `32 MB` | Max memory for multipart forms |

### `Ctx(r *http.Request) *Context`

Retrieves the `Context` from the request. Must be called inside a handler wrapped by `New()`.

```go
ctx := pirca.Ctx(r)
```

## API Reference

### 📦 Body Binding

Read and deserialize the request body.

| Method | Stream | Cache | Best for |
|--------|--------|-------|----------|
| `BindJSON(obj)` | decoder | ❌ | JSON, single read, efficient |
| `BindJSONStrict(obj)` | decoder | ❌ | JSON + reject unknown fields |
| `BindXML(obj)` | decoder | ❌ | XML, single read |
| `Bind(obj, binder)` | `[]byte` | ❌ | Custom formats (TOML, YAML, etc.) |
| `BindBodyWith(obj, binder)` | `[]byte` | ✅ | Custom formats + multiple reads |
| `BindJSONWith(obj)` | `[]byte` | ✅ | JSON + multiple reads |
| `BindJSONStrictWith(obj)` | `[]byte` | ✅ | JSON strict + multiple reads |
| `BindXMLWith(obj)` | `[]byte` | ✅ | XML + multiple reads |

```go
ctx := pirca.Ctx(r)

// Stream JSON (single read)
var user User
if err := ctx.BindJSON(&user); err != nil { ... }

// Strict JSON — rejects unknown fields
if err := ctx.BindJSONStrict(&user); err != nil { ... }

// Custom format (TOML, YAML, MessagePack, etc.)
var cfg Config
if err := ctx.Bind(&cfg, toml.Unmarshal); err != nil { ... }

// Cached body — can be read again by other middlewares
var product Product
if err := ctx.BindBodyWith(&product, json.Unmarshal); err != nil { ... }
ctx.Set("product", product) // share with other handlers

// Convenience cached variants
ctx.BindJSONWith(&user)       // json.Unmarshal
ctx.BindJSONStrictWith(&user) // json with DisallowUnknownFields
ctx.BindXMLWith(&doc)         // xml.Unmarshal
```

#### Raw body access

```go
// Read body as bytes (single read)
body, err := ctx.GetBodyBytes()

// Direct reader access
raw, err := io.ReadAll(ctx.Request.Body)
```

### 📤 Response Writers

Write responses in various formats.

| Method | Content-Type | Description |
|--------|-------------|-------------|
| `JSON(code, obj)` | `application/json` | Serializes as JSON |
| `XML(code, obj)` | `application/xml` | Serializes as XML |
| `String(code, msg)` | not set | Plain text |
| `Data(code, data)` | not set | Raw bytes |
| `Redirect(code, location)` | — | HTTP redirect |
| `File(filepath)` | auto | Serves a file |
| `FileFromFS(filepath, fs)` | auto | Serves from `http.FileSystem` |
| `FileAttachment(filepath, filename)` | auto | Forces download |

```go
ctx := pirca.Ctx(r)

ctx.JSON(http.StatusOK, map[string]string{"message": "ok"})
ctx.XML(http.StatusCreated, myStruct)
ctx.String(http.StatusOK, "<h1>Hello</h1>")
ctx.Data(http.StatusOK, pdfBytes)
ctx.Redirect(http.StatusMovedPermanently, "/new-url")

// Files
ctx.File("./static/index.html")
ctx.FileFromFS("static/style.css", http.FS(embedFS))
ctx.FileAttachment("./docs/report.pdf", "report_2026.pdf")
```

### 🔗 Query Parameters

Access URL query parameters.

```go
ctx := pirca.Ctx(r)

// GET /search?q=golang&page=1&color=red&color=blue

q := ctx.Query("q")                 // "golang"
page := ctx.DefaultQuery("page", "1") // "1" (default)
limit := ctx.DefaultQuery("limit", "10") // "10" (not in URL)

value, exists := ctx.GetQuery("q")  // ("golang", true)
value, exists := ctx.GetQuery("wtf") // ("", false)

colors := ctx.QueryArray("color")     // ["red", "blue"]
values, ok := ctx.GetQueryArray("color") // (["red", "blue"], true)
```

| Method | Returns | Description |
|--------|---------|-------------|
| `Query(key)` | `string` | Value or `""` |
| `DefaultQuery(key, default)` | `string` | Value or default if missing |
| `GetQuery(key)` | `(string, bool)` | Value + existence check |
| `QueryArray(key)` | `[]string` | All values |
| `GetQueryArray(key)` | `([]string, bool)` | All values + existence |

### 📋 Path Parameters (Go 1.22+)

```go
// Pattern: GET /user/{id}
ctx := pirca.Ctx(r)
id := ctx.Param("id") // "123"
```

### 📝 Form Values

Access form fields from `application/x-www-form-urlencoded` and `multipart/form-data`.

```go
ctx := pirca.Ctx(r)

name := ctx.FormValue("name")                  // "jesus" or ""
name := ctx.DefaultFormValue("name", "guest")   // "jesus" or "guest" if missing
name, exists := ctx.GetFormValue("name")        // ("jesus", true) or ("", false)
```

| Method | Returns | Description |
|--------|---------|-------------|
| `FormValue(key)` | `string` | Value or `""` |
| `DefaultFormValue(key, default)` | `string` | Value or default if missing |
| `GetFormValue(key)` | `(string, bool)` | Value + existence check |

### 📎 File Uploads

Handle multipart file uploads.

```go
ctx := pirca.Ctx(r)

// Single file
file, err := ctx.FormFile("avatar")
if err != nil { ... }
ctx.SaveUploadedFile(file, "./uploads/"+file.Filename)

// Multiple files
form, err := ctx.MultipartForm()
if err != nil { ... }
for _, file := range form.File["images"] {
    ctx.SaveUploadedFile(file, "./uploads/"+file.Filename)
}
```

| Method | Description |
|--------|-------------|
| `FormFile(name)` | Returns the first file for the given field |
| `MultipartForm()` | Returns the full parsed multipart form |
| `SaveUploadedFile(file, dst, perm...)` | Saves to disk (creates dirs, optional permissions) |

### 🍪 Cookies

```go
ctx := pirca.Ctx(r)

// Set
ctx.SetSameSite(http.SameSiteLaxMode)
ctx.SetCookie("token", "abc123", 3600, "/", "example.com", true, true)

// Set with pre-built cookie
ctx.SetCookieData(&http.Cookie{
    Name:  "session",
    Value: sessionID,
})

// Get
val, err := ctx.Cookie("token") // http.ErrNoCookie if missing
```

| Method | Description |
|--------|-------------|
| `SetSameSite(samesite)` | Sets SameSite attribute for subsequent cookies |
| `SetCookie(name, value, maxAge, path, domain, secure, httpOnly)` | Writes a Set-Cookie header |
| `SetCookieData(cookie)` | Writes using a pre-built `*http.Cookie` |
| `Cookie(name)` | Reads a cookie from the request (URL-decoded) |

### 📋 Headers

```go
ctx := pirca.Ctx(r)

ctx.Header("X-Custom", "value")     // set response header
ctx.Header("X-Custom", "")          // delete response header
val := ctx.GetHeader("Content-Type") // read request header
```

| Method | Description |
|--------|-------------|
| `Header(key, value)` | Sets or deletes a response header |
| `GetHeader(key)` | Returns a request header value |

### 📊 Status & Response Metrics

```go
ctx := pirca.Ctx(r)

ctx.Status(http.StatusCreated)
fmt.Println(ctx.GetStatus())    // 201
fmt.Println(ctx.BytesWritten()) // total bytes written to response body
```

| Method | Description |
|--------|-------------|
| `Status(code)` | Writes the HTTP status code |
| `GetStatus()` | Returns the written status code |
| `BytesWritten()` | Returns total bytes written to the body |

### 🔑 Key-Value Store

Share data between middlewares and handlers within the same request.

```go
ctx := pirca.Ctx(r)

// Set
ctx.Set("userID", "123")
ctx.Set("role", "admin")

// Get
if userID, ok := ctx.Get("userID"); ok {
    fmt.Println(userID)
}

// Delete
ctx.Delete("tempData")
```

All methods are safe for concurrent use.

| Method | Description |
|--------|-------------|
| `Set(key, value)` | Stores a value (any type) |
| `Get(key)` | Retrieves a value + exists bool |
| `Delete(key)` | Removes a value |

### 🌐 `context.Context` Implementation

`*Context` implements `context.Context`, so it can be passed directly to any function that accepts one.

```go
ctx := pirca.Ctx(r)

// Pass to database, HTTP client, tracer, etc.
user, err := db.FindUser(ctx, id)
resp, err := http.NewRequestWithContext(ctx, "GET", url, nil)
span, _ := tracer.Start(ctx, "handler")
```

- `Deadline()` — delegates to the parent request context
- `Done()` — closed when the client disconnects
- `Err()` — returns cancellation error
- `Value(key)` — string keys search the local store first, then fall back to the parent context

## Middleware Integration

Since `New()` captures the status code and bytes written through an internal `responseWriter`, you can build middlewares without wrapping the ResponseWriter yourself.

```go
func loggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        ctx := pirca.Ctx(r)
        start := time.Now()
        next.ServeHTTP(w, r)
        log.Printf(
            "%s %s %d %d %v",
            r.Method, r.URL.Path,
            ctx.GetStatus(), ctx.BytesWritten(),
            time.Since(start),
        )
    })
}

func main() {
    mux := http.NewServeMux()
    handler := pirca.New()(loggingMiddleware(mux))
    http.ListenAndServe(":8080", handler)
}
```

## Flexibility

Because Pirca works directly with `net/http`, you always have full access to the underlying types:

```go
ctx := pirca.Ctx(r)

// ctx.Request is the original *http.Request
// ctx.Writer is the original http.ResponseWriter (wrapped)
// They are the same references as the handler parameters

fmt.Fprintf(ctx.Writer, "raw write")
ctx.Request.Header.Get("Authorization")
r.Method // also works — r is the same as ctx.Request
```

You're never locked into the middleware. Use `ctx.Request` directly, use `ctx.Writer` directly, or use the original `r` and `w` — they're all the same objects.

## Complete Example

```go
func handler(w http.ResponseWriter, r *http.Request) {
    ctx := pirca.Ctx(r)

    var payload struct {
        Name string `json:"name"`
        Age  int    `json:"age"`
    }

    if err := ctx.BindJSON(&payload); err != nil {
        ctx.JSON(http.StatusBadRequest, map[string]string{
            "error": "invalid request body",
        })
        return
    }

    page := ctx.DefaultQuery("page", "1")

    if token, err := ctx.Cookie("session"); err == nil {
        ctx.Set("session_token", token)
    }

    ctx.JSON(http.StatusOK, map[string]any{
        "name":  payload.Name,
        "age":   payload.Age,
        "page":  page,
        "agent": ctx.GetHeader("User-Agent"),
    })
}
```

## License

MIT
