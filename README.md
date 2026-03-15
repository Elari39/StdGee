# StdGee

> StdGee keeps Gee's learning-friendly API, but hands routing back to modern `net/http`.

[中文说明 / Chinese README](README.zh-CN.md)

StdGee is a small teaching project that rebuilds the ideas from Gee-Web on top of the Go standard library. It keeps the familiar learning surface of `Engine`, `Group`, `Context`, templates, static files, logger/recovery middleware, and graceful shutdown, while letting `http.ServeMux` own routing, method matching, path wildcards, and core HTTP semantics.

## At a Glance

- Audience: readers who learned Gee-Web and want to see what the same ideas look like after Go 1.22.
- Baseline: this repo targets `go 1.25.0`.
- Main lesson: after Go 1.22, many "framework internals" for small web projects are better delegated to the standard library.
- Optional extra: the repo also shows how Go 1.25's `http.CrossOriginProtection` can fit into the middleware chain.

## Quick Start

```bash
go run .
```

Open `http://localhost:9999` and try:

- `GET /`
- `GET /ping`
- `GET /posts/`
- `GET /v1/hello/gopher`
- `GET /v1/json`
- `GET /assets/hello.txt`
- `GET /v1/panic`

For the form example:

```bash
curl -X POST -d "username=gopher&password=123456" http://localhost:9999/v1/login
```

## Why This Matters After Go 1.22

Go 1.22 significantly upgraded `http.ServeMux`. That changed the tradeoff for small teaching frameworks and small production services.

`ServeMux` now supports:

- method-aware patterns such as `GET /posts/{id}`
- single-segment wildcards such as `{id}`
- catch-all wildcards such as `{filepath...}`
- exact trailing-slash matching with `{$}`
- path parameter access through `req.PathValue("id")`
- automatic `HEAD` support for `GET` routes
- automatic `405 Method Not Allowed` responses with an `Allow` header

That means a project like StdGee no longer needs to spend most of its complexity on:

- building a custom trie router
- parsing and storing path params in a separate map
- manually dispatching methods
- re-implementing `HEAD` and `405` behavior

The result is the real lesson of this repo: keep the friendly API, delete the no-longer-necessary router code.

## Gee-Web vs StdGee

| Dimension | Gee-Web style | StdGee |
| --- | --- | --- |
| Router core | Custom trie router | `http.ServeMux` |
| Route syntax | `:name`, `*filepath` | `{name}`, `{filepath...}`, `{$}` |
| Method matching | Framework-managed | Standard library pattern like `GET /path` |
| Path params | Custom params map | `req.PathValue` |
| Middleware model | `Context.Next()` chain | `func(http.Handler) http.Handler` |
| `HEAD` for `GET` | Usually handled manually | Built in |
| `405` and `Allow` | Usually handled manually | Built in |
| Conflict detection | Framework-specific rules | `ServeMux` panics on conflicting registrations |
| Graceful shutdown | Often omitted in tutorials | Exposed through `http.Server.Shutdown` |

## What Comes From `net/http`, What Stays in StdGee

Handled by the standard library:

- route parsing and matching
- path wildcard extraction
- `HEAD` compatibility for `GET`
- `405 Method Not Allowed`
- conflicting route panic behavior

Handled by StdGee:

- Gee-like grouping with `Group("/v1")`
- middleware collection and wrapping order
- `Context` response helpers such as `String`, `JSON`, and `HTML`
- template loading and rendering helpers
- static file mounting helpers
- graceful shutdown through the engine wrapper

## What Go 1.22 Changed in Practice

### 1. Method-aware route patterns

You can register HTTP method and path together:

```go
r.HandleFunc("GET /ping", func(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("pong\n"))
})
```

This removes the need for a separate `GET`/`POST` dispatch table inside the router.

### 2. Native path wildcards

The standard library now understands patterns like:

```go
GET /v1/hello/{name}
GET /assets/{filepath...}
GET /posts/{$}
```

- `{name}` matches one path segment.
- `{filepath...}` matches the rest of the path.
- `{$}` matches the path itself, but not child paths.

### 3. Native path parameter access

Instead of maintaining a framework-owned params map:

```go
name := req.PathValue("name")
```

StdGee's `Context.Param` is just a convenience wrapper over that standard-library behavior.

### 4. Better default HTTP semantics

If you register `GET /ping`, the standard library also handles `HEAD /ping`.
If the path matches but the method does not, `ServeMux` returns `405 Method Not Allowed` and populates `Allow`.

That is exactly the kind of behavior small frameworks should stop re-implementing.

## Two Registration Styles

StdGee intentionally supports both a standard-library-first style and a Gee-style sugar layer.

### Standard-library-first

```go
v1 := r.Group("/v1")

v1.HandleFunc("GET /json", func(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"message": "hello from stdgee",
		"path":    req.URL.Path,
		"route":   req.Pattern,
	})
})
```

This style teaches the modern `net/http` API directly.

### Gee-style sugar

```go
v1 := r.Group("/v1")

v1.GET("/hello/{name}", func(c *stdgee.Context) {
	c.String(http.StatusOK, "hello %s, you are at %s\n", c.Param("name"), c.Path)
})
```

This style keeps the original Gee learning experience, but now the routing semantics still come from `ServeMux`.

## Public API Learning Surface

Core engine APIs:

- `stdgee.New()`
- `Engine.Handle`
- `Engine.HandleFunc`
- `Engine.GET`
- `Engine.POST`
- `Engine.Group`
- `Engine.Use`
- `Engine.Run`
- `Engine.Shutdown`

Context helpers:

- `Context.Param`
- `Context.Query`
- `Context.PostForm`
- `Context.String`
- `Context.JSON`
- `Context.HTML`
- `Context.Data`

View and assets:

- `SetFuncMap`
- `LoadHTMLGlob`
- `Static`

Middleware shape:

```go
type Middleware func(http.Handler) http.Handler
```

That signature is much closer to the broader Go ecosystem than a framework-specific `Next()` model.

## Middleware, Templates, Static Files, and Shutdown

### Middleware

```go
r.Use(stdgee.Logger(), stdgee.Recovery())
```

Groups inherit parent middleware, and child-group middleware wraps closer to the handler. The tests in this repo verify the final execution order.

### Templates

```go
r.SetFuncMap(template.FuncMap{
	"FormatAsDateTime": func(t time.Time) string {
		return t.Format("2006-01-02 15:04:05")
	},
})
r.LoadHTMLGlob("templates/*")
```

The rendering engine is still `html/template`; StdGee only provides a convenient place to store and use it.

### Static files

```go
r.Static("/assets", "./static")
```

Under the hood this is standard-library file serving via `http.FileServer` and `http.StripPrefix`.

### Graceful shutdown

```go
serverErr := make(chan error, 1)
go func() {
	serverErr <- r.Run(":9999")
}()

shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

_ = r.Shutdown(shutdownCtx)
```

Unlike many tutorial frameworks, StdGee keeps an `http.Server` so graceful shutdown is part of the public story.

## Migrating from Gee-Web

### 1. Convert route syntax

- `:name` becomes `{name}`
- `*filepath` becomes `{filepath...}`

Example:

```go
// Gee-Web
v1.GET("/hello/:name", handler)

// StdGee
v1.GET("/hello/{name}", handler)
```

### 2. Convert path parameter access

If you use StdGee's `Context`, this still works:

```go
c.Param("name")
```

If you use a standard-library handler directly, use:

```go
req.PathValue("name")
```

### 3. Convert middleware shape

Gee-Web middleware often revolves around framework context and `Next()`.
StdGee middleware uses the standard Go signature:

```go
func(http.Handler) http.Handler
```

That makes middleware easier to reuse across projects that already speak `net/http`.

## Optional Go 1.25 Addition: `CrossOriginProtection`

Because this repo targets `go 1.25.0`, you can also plug in the new standard-library cross-origin protection:

```go
cop := http.NewCrossOriginProtection()
cop.AddTrustedOrigin("https://example.com")

r.Use(stdgee.ProtectCrossOrigin(cop))
```

This is intentionally optional. The core teaching goal of StdGee is still the Go 1.22 routing model.

## Project Layout

```text
Gee-Web-Std/
|-- go.mod
|-- main.go
|-- README.md
|-- README.zh-CN.md
|-- static/
|   `-- hello.txt
|-- templates/
|   `-- index.html
`-- stdgee/
    |-- context.go
    |-- engine.go
    |-- engine_test.go
    `-- middleware.go
```

## Further Reading

- Go 1.22 Release Notes: https://go.dev/doc/go1.22
- Go 1.22 Release Blog: https://go.dev/blog/go1.22
- Go 1.25 Release Notes: https://go.dev/doc/go1.25
- `net/http` package docs for Go 1.25: https://pkg.go.dev/net/http@go1.25.0

## One-Sentence Summary

Gee-Web teaches how to build framework internals from scratch; StdGee teaches which of those internals should now be deleted in favor of modern `net/http`.
#   S t d G e e  
 