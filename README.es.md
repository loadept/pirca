# Pirca

**Pirca** es un middleware HTTP liviano para Go que extiende `net/http` con un `Context` rico, helpers de respuesta, binders de request, manejo de formularios y archivos, y más — todo sin reemplazar la biblioteca estándar ni agregar dependencias externas.

Basado en **Gin**, Pirca sigue las mismas convenciones y lógica, adaptado para funcionar directamente con `net/http`. Muchos métodos reflejan el API de Gin, haciéndolo familiar si ya has usado Gin, pero sin el lock-in de un framework.

- **Cero dependencias** — solo la stdlib de Go.
- **Basado en Gin** — mismos patrones, misma sensación, sin framework.
- **Control total** — `ctx.Request` y `ctx.Writer` son el `*http.Request` y `http.ResponseWriter` originales. Úsalos directamente cuando necesites.
- **Implementa `context.Context`** — pasa `ctx` directamente a bases de datos, HTTP clients, trazadores, etc.
- **Captura status code y bytes escritos** — ideal para middlewares de logging, métricas y observabilidad.
- **Acelera el desarrollo** — binders JSON/XML, escritura de respuestas, subida de archivos, cookies, query params, formularios — todo listo para usar.

## Requisitos

- Go 1.22 o superior

## Instalación

```bash
go get github.com/loadept/pirca
```

## Primeros pasos

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
            "message": "Hola mundo",
        })
    })

    log.Fatal(http.ListenAndServe(":8080", handler))
}
```

### Cómo funciona

1. `pirca.New()` retorna un middleware que crea un `Context` por cada request.
2. Dentro del handler, `pirca.Ctx(r)` recupera el `Context` del request.
3. Usa `ctx` para bindear bodies, escribir respuestas, manejar cookies, query params, formularios, archivos y más.

## API Central

### `New(cfg ...*Config) func(http.Handler) http.Handler`

Crea el middleware. Debe ser la capa más externa de tu cadena de handlers. Acepta un `*Config` opcional.

```go
// Valores por defecto
handler := pirca.New()(mux)

// Con configuración personalizada
handler := pirca.New(&pirca.Config{
    MaxBodySize:        1 << 20,       // 1MB máximo de body
    MaxMultipartMemory: 64 << 20,      // 64MB para multipart
})(mux)
```

### `Config`

| Campo | Tipo | Default | Descripción |
|-------|------|---------|-------------|
| `MaxBodySize` | `int64` | `0` (sin límite) | Tamaño máximo del body en bytes |
| `MaxMultipartMemory` | `int64` | `32 MB` | Memoria máxima para forms multipart |

### `Ctx(r *http.Request) *Context`

Recupera el `Context` del request. Debe llamarse dentro de un handler envuelto por `New()`.

```go
ctx := pirca.Ctx(r)
```

## Referencia de la API

### 📦 Body Binding

Lee y deserializa el body del request.

| Método | Stream | Cache | Mejor para |
|--------|--------|-------|------------|
| `BindJSON(obj)` | decoder | ❌ | JSON, una lectura, eficiente |
| `BindJSONStrict(obj)` | decoder | ❌ | JSON + rechazar campos desconocidos |
| `BindXML(obj)` | decoder | ❌ | XML, una lectura |
| `Bind(obj, binder)` | `[]byte` | ❌ | Formatos custom (TOML, YAML, etc.) |
| `BindBodyWith(obj, binder)` | `[]byte` | ✅ | Formatos custom + múltiples lecturas |
| `BindJSONWith(obj)` | `[]byte` | ✅ | JSON + múltiples lecturas |
| `BindJSONStrictWith(obj)` | `[]byte` | ✅ | JSON estricto + múltiples lecturas |
| `BindXMLWith(obj)` | `[]byte` | ✅ | XML + múltiples lecturas |

```go
ctx := pirca.Ctx(r)

// JSON desde stream (una sola lectura)
var user User
if err := ctx.BindJSON(&user); err != nil { ... }

// JSON estricto — rechaza campos desconocidos
if err := ctx.BindJSONStrict(&user); err != nil { ... }

// Formato custom (TOML, YAML, MessagePack, etc.)
var cfg Config
if err := ctx.Bind(&cfg, toml.Unmarshal); err != nil { ... }

// Body cacheado — puede ser leído de nuevo por otros middlewares
var product Product
if err := ctx.BindBodyWith(&product, json.Unmarshal); err != nil { ... }
ctx.Set("product", product) // compartir con otros handlers

// Variantes con cache por conveniencia
ctx.BindJSONWith(&user)       // json.Unmarshal
ctx.BindJSONStrictWith(&user) // json con DisallowUnknownFields
ctx.BindXMLWith(&doc)         // xml.Unmarshal
```

#### Acceso al body crudo

```go
// Leer body como bytes (una sola lectura)
body, err := ctx.GetBodyBytes()

// Acceso directo al reader
raw, err := io.ReadAll(ctx.Request.Body)
```

### 📤 Response Writers

Escribe respuestas en varios formatos.

| Método | Content-Type | Descripción |
|--------|-------------|-------------|
| `JSON(code, obj)` | `application/json` | Serializa como JSON |
| `XML(code, obj)` | `application/xml` | Serializa como XML |
| `String(code, msg)` | no se setea | Texto plano |
| `Data(code, data)` | no se setea | Bytes crudos |
| `Redirect(code, location)` | — | Redirección HTTP |
| `File(filepath)` | auto | Sirve un archivo |
| `FileFromFS(filepath, fs)` | auto | Sirve desde `http.FileSystem` |
| `FileAttachment(filepath, filename)` | auto | Fuerza descarga |

```go
ctx := pirca.Ctx(r)

ctx.JSON(http.StatusOK, map[string]string{"message": "ok"})
ctx.XML(http.StatusCreated, miStruct)
ctx.String(http.StatusOK, "<h1>Hola</h1>")
ctx.Data(http.StatusOK, pdfBytes)
ctx.Redirect(http.StatusMovedPermanently, "/nueva-url")

// Archivos
ctx.File("./static/index.html")
ctx.FileFromFS("static/style.css", http.FS(embedFS))
ctx.FileAttachment("./docs/reporte.pdf", "reporte_2026.pdf")
```

### 🔗 Query Parameters

Accede a los parámetros de la URL.

```go
ctx := pirca.Ctx(r)

// GET /search?q=golang&page=1&color=red&color=blue

q := ctx.Query("q")                 // "golang"
page := ctx.DefaultQuery("page", "1") // "1" (default)
limit := ctx.DefaultQuery("limit", "10") // "10" (no está en URL)

value, exists := ctx.GetQuery("q")  // ("golang", true)
value, exists := ctx.GetQuery("wtf") // ("", false)

colors := ctx.QueryArray("color")     // ["red", "blue"]
values, ok := ctx.GetQueryArray("color") // (["red", "blue"], true)
```

| Método | Retorna | Descripción |
|--------|---------|-------------|
| `Query(key)` | `string` | Valor o `""` |
| `DefaultQuery(key, default)` | `string` | Valor o default si no existe |
| `GetQuery(key)` | `(string, bool)` | Valor + verificación de existencia |
| `QueryArray(key)` | `[]string` | Todos los valores |
| `GetQueryArray(key)` | `([]string, bool)` | Todos los valores + existencia |

### 📋 Path Parameters (Go 1.22+)

```go
// Patrón: GET /user/{id}
ctx := pirca.Ctx(r)
id := ctx.Param("id") // "123"
```

### 📝 Form Values

Accede a campos de formulario `application/x-www-form-urlencoded` y `multipart/form-data`.

```go
ctx := pirca.Ctx(r)

name := ctx.FormValue("name")                  // "jesus" o ""
name := ctx.DefaultFormValue("name", "invitado") // "jesus" o "invitado" si falta
name, exists := ctx.GetFormValue("name")        // ("jesus", true) o ("", false)
```

| Método | Retorna | Descripción |
|--------|---------|-------------|
| `FormValue(key)` | `string` | Valor o `""` |
| `DefaultFormValue(key, default)` | `string` | Valor o default si falta |
| `GetFormValue(key)` | `(string, bool)` | Valor + verificación de existencia |

### 📎 File Uploads

Manejo de subida de archivos multipart.

```go
ctx := pirca.Ctx(r)

// Archivo único
file, err := ctx.FormFile("avatar")
if err != nil { ... }
ctx.SaveUploadedFile(file, "./uploads/"+file.Filename)

// Múltiples archivos
form, err := ctx.MultipartForm()
if err != nil { ... }
for _, file := range form.File["images"] {
    ctx.SaveUploadedFile(file, "./uploads/"+file.Filename)
}
```

| Método | Descripción |
|--------|-------------|
| `FormFile(name)` | Retorna el primer archivo del campo indicado |
| `MultipartForm()` | Retorna el formulario multipart completo |
| `SaveUploadedFile(file, dst, perm...)` | Guarda en disco (crea directorios, permisos opcionales) |

### 🍪 Cookies

```go
ctx := pirca.Ctx(r)

// Setear
ctx.SetSameSite(http.SameSiteLaxMode)
ctx.SetCookie("token", "abc123", 3600, "/", "example.com", true, true)

// Setear con cookie pre-construida
ctx.SetCookieData(&http.Cookie{
    Name:  "session",
    Value: sessionID,
})

// Obtener
val, err := ctx.Cookie("token") // http.ErrNoCookie si no existe
```

| Método | Descripción |
|--------|-------------|
| `SetSameSite(samesite)` | Configura SameSite para cookies posteriores |
| `SetCookie(name, value, maxAge, path, domain, secure, httpOnly)` | Escribe header Set-Cookie |
| `SetCookieData(cookie)` | Escribe usando un `*http.Cookie` pre-construido |
| `Cookie(name)` | Lee una cookie del request (URL-decodeada) |

### 📋 Headers

```go
ctx := pirca.Ctx(r)

ctx.Header("X-Custom", "valor")     // setear header de respuesta
ctx.Header("X-Custom", "")          // eliminar header de respuesta
val := ctx.GetHeader("Content-Type") // leer header del request
```

| Método | Descripción |
|--------|-------------|
| `Header(key, value)` | Setea o elimina un header de respuesta |
| `GetHeader(key)` | Retorna el valor de un header del request |

### 📊 Status y Métricas de Respuesta

```go
ctx := pirca.Ctx(r)

ctx.Status(http.StatusCreated)
fmt.Println(ctx.GetStatus())    // 201
fmt.Println(ctx.BytesWritten()) // bytes totales escritos al body
```

| Método | Descripción |
|--------|-------------|
| `Status(code)` | Escribe el código de estado HTTP |
| `GetStatus()` | Retorna el código de estado escrito |
| `BytesWritten()` | Retorna el total de bytes escritos al body |

### 🔑 Key-Value Store

Comparte datos entre middlewares y handlers dentro del mismo request.

```go
ctx := pirca.Ctx(r)

// Setear
ctx.Set("userID", "123")
ctx.Set("role", "admin")

// Obtener
if userID, ok := ctx.Get("userID"); ok {
    fmt.Println(userID)
}

// Eliminar
ctx.Delete("tempData")
```

Todos los métodos son seguros para uso concurrente.

| Método | Descripción |
|--------|-------------|
| `Set(key, value)` | Almacena un valor (cualquier tipo) |
| `Get(key)` | Recupera un valor + bool de existencia |
| `Delete(key)` | Elimina un valor |

### 🌐 Implementación de `context.Context`

`*Context` implementa `context.Context`, por lo que puede pasarse directamente a cualquier función que acepte uno.

```go
ctx := pirca.Ctx(r)

// Pasar a base de datos, HTTP client, trazador, etc.
user, err := db.FindUser(ctx, id)
resp, err := http.NewRequestWithContext(ctx, "GET", url, nil)
span, _ := tracer.Start(ctx, "handler")
```

- `Deadline()` — delega al contexto padre del request
- `Done()` — se cierra cuando el cliente se desconecta
- `Err()` — retorna el error de cancelación
- `Value(key)` — las keys string buscan en el store local primero, luego delegan al contexto padre

## Integración con Middlewares

Como `New()` captura el status code y bytes escritos a través de un `responseWriter` interno, puedes construir middlewares sin wrappear el ResponseWriter tú mismo.

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

## Flexibilidad

Como Pirca trabaja directamente con `net/http`, siempre tienes acceso completo a los tipos subyacentes:

```go
ctx := pirca.Ctx(r)

// ctx.Request es el *http.Request original
// ctx.Writer es el http.ResponseWriter original (wrapeado)
// Son las mismas referencias que los parámetros del handler

fmt.Fprintf(ctx.Writer, "escritura directa")
ctx.Request.Header.Get("Authorization")
r.Method // también funciona — r es el mismo que ctx.Request
```

Nunca estás encerrado por el middleware. Usa `ctx.Request` directamente, `ctx.Writer` directamente, o los `r` y `w` originales — todos son los mismos objetos.

## Ejemplo Completo

```go
func handler(w http.ResponseWriter, r *http.Request) {
    ctx := pirca.Ctx(r)

    var payload struct {
        Name string `json:"name"`
        Age  int    `json:"age"`
    }

    if err := ctx.BindJSON(&payload); err != nil {
        ctx.JSON(http.StatusBadRequest, map[string]string{
            "error": "cuerpo de request inválido",
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

## Licencia

MIT
