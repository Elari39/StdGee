# StdGee

> A Gee-style teaching framework rebuilt on top of modern `net/http`.

[中文文档 / Chinese README](README.zh-CN.md)

| Snapshot | Details |
| --- | --- |
| Goal | Rebuild the ideas from Gee-Web without a custom router |
| Audience | Readers learning modern `net/http` or revisiting Gee-Web after Go 1.22 |
| Go baseline | `go 1.25.0` |
| Main takeaway | Keep the friendly API, delete the routing code the standard library now handles well |

StdGee is a small teaching project that keeps `Engine`, `Group`, `Context`, templates, static files, middleware, and graceful shutdown, while delegating routing, method matching, path wildcards, and core HTTP semantics to `http.ServeMux`.

## Table of Contents

- [Quick Start](#quick-start)
- [Why This Repo Exists](#why-this-repo-exists)
- [What Go 1.22 Changed](#what-go-122-changed)
- [Gee-Web vs StdGee](#gee-web-vs-stdgee)
- [What Comes From `net/http`](#what-comes-from-nethttp)
- [Two Ways to Register Handlers](#two-ways-to-register-handlers)
- [Public API Surface](#public-api-surface)
- [Common Building Blocks](#common-building-blocks)
- [Migration from Gee-Web](#migration-from-gee-web)
- [Optional Go 1.25 Addition](#optional-go-125-addition)
- [Project Layout](#project-layout)
- [Further Reading](#further-reading)

## Quick Start

Run the demo:

```bash
go run .
```

Then open `http://localhost:9999`.

### Demo Routes

| Route | Purpose |
| --- | --- |
| `GET /` | HTML template rendering demo |
| `GET /ping` | Basic method-aware route |
| `GET /posts/` | Exact trailing-slash matching with `{$}` |
| `GET /v1/hello/gopher` | Path wildcard + `Context.Param` |
| `GET /v1/json` | Standard-library-first handler style |
| `GET /assets/hello.txt` | Static file serving |
| `GET /v1/panic` | Recovery middleware demo |

Form example:

```bash
curl -X POST -d "username=gopher&password=123456" http://localhost:9999/v1/login
```

## Why This Repo Exists

Gee-Web is great for learning how a small web framework can be built from scratch. StdGee asks a different question:

> After Go 1.22, which parts of that framework should still exist, and which parts should be handed back to the standard library?

The answer is the core idea of this repo:

- keep the teaching-friendly API surface
- keep grouping, middleware composition, templates, and response helpers
- stop re-implementing router behavior that `http.ServeMux` now handles well

## What Go 1.22 Changed

Go 1.22 made `http.ServeMux` much more capable for small web applications.

| Feature | Example | Why it matters |
| --- | --- | --- |
| Method-aware patterns | `GET /posts/{id}` | No separate method dispatch table is needed |
| Single-segment wildcard | `{id}` | No custom params parser for common path params |
| Catch-all wildcard | `{filepath...}` | Static and nested path matching become simpler |
| Exact path match | `GET /posts/{$}` | Trailing-slash behavior is easier to express |
| Native path params | `req.PathValue("id")` | No framework-owned params map is required |
| Automatic `HEAD` for `GET` | `GET /ping` also matches `HEAD /ping` | Less manual HTTP compatibility code |
| Automatic `405` + `Allow` | Wrong method on a matched path | Better defaults without extra framework logic |

In practice, that lets StdGee delete a lot of code a tutorial router used to need:

- custom trie routing
- custom path param storage
- manual method dispatch
- manual `HEAD` and `405` behavior

## Gee-Web vs StdGee

| Dimension | Gee-Web style | StdGee |
| --- | --- | --- |
| Router core | Custom trie router | `http.ServeMux` |
| Route syntax | `:name`, `*filepath` | `{name}`, `{filepath...}`, `{$}` |
| Method matching | Framework-managed | Standard-library patterns like `GET /path` |
| Path params | Custom params map | `req.PathValue` |
| Middleware model | `Context.Next()` chain | `func(http.Handler) http.Handler` |
| `HEAD` for `GET` | Usually handled manually | Built in |
| `405` and `Allow` | Usually handled manually | Built in |
| Conflict detection | Framework-specific rules | `ServeMux` panics on conflicting registrations |
| Graceful shutdown | Often omitted in tutorials | Exposed through `http.Server.Shutdown` |

## What Comes From `net/http`

| Owned by the standard library | Provided by StdGee |
| --- | --- |
| Route parsing and matching | Gee-like route grouping with `Group("/v1")` |
| Path wildcard extraction | Middleware collection and wrapping order |
| `HEAD` compatibility for `GET` | `Context` helpers such as `String`, `JSON`, and `HTML` |
| `405 Method Not Allowed` + `Allow` | Template loading and rendering helpers |
| Conflicting route panic behavior | Static file mounting helpers |
|  | Graceful shutdown through the engine wrapper |

That split is the architectural lesson of the project.

## Two Ways to Register Handlers

StdGee intentionally supports both a standard-library-first style and a Gee-style sugar layer.

### Standard-library-first

```go
v1 := r.Group("/v1")

v1.HandleFunc("GET /json", func(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"message": "hello from stdgee",
		"ts":      time.Now().Unix(),
		"path":    req.URL.Path,
		"route":   req.Pattern,
	})
})
```

Use this style when you want to teach or learn modern `net/http` directly.

### Gee-style sugar

```go
v1 := r.Group("/v1")

v1.GET("/hello/{name}", func(c *stdgee.Context) {
	c.String(http.StatusOK, "hello %s, you are at %s\n", c.Param("name"), c.Path)
})
```

Use this style when you want the original Gee learning feel, but with standard-library routing underneath.

## Public API Surface

| Area | APIs |
| --- | --- |
| Engine | `stdgee.New()`, `Handle`, `HandleFunc`, `GET`, `POST`, `Group`, `Use`, `Run`, `Shutdown` |
| Context | `Param`, `Query`, `PostForm`, `String`, `JSON`, `HTML`, `Data` |
| Views and assets | `SetFuncMap`, `LoadHTMLGlob`, `Static` |
| Middleware | `type Middleware func(http.Handler) http.Handler` |

That middleware signature is especially important: it aligns StdGee with the broader Go `net/http` ecosystem instead of a framework-only `Next()` model.

## Common Building Blocks

### Middleware

```go
r.Use(stdgee.Logger(), stdgee.Recovery())
```

Parent-group middleware wraps outside child-group middleware. The tests in this repo verify the final execution order.

### Templates

```go
r.SetFuncMap(template.FuncMap{
	"FormatAsDateTime": func(t time.Time) string {
		return t.Format("2006-01-02 15:04:05")
	},
})
r.LoadHTMLGlob("templates/*")
```

StdGee stores the templates, but rendering still comes from `html/template`.

### Static files

```go
r.Static("/assets", "./static")
```

Under the hood this uses `http.FileServer` and `http.StripPrefix`.

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

Unlike many tutorial frameworks, StdGee keeps an `http.Server`, so graceful shutdown is part of the public story.

## Migration from Gee-Web

### 1. Convert route syntax

- `:name` becomes `{name}`
- `*filepath` becomes `{filepath...}`

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

That makes middleware easier to reuse across other `net/http` projects.

## Optional Go 1.25 Addition

Because this repo targets `go 1.25.0`, it can also demonstrate the standard-library cross-origin protection added in Go 1.25:

```go
cop := http.NewCrossOriginProtection()
cop.AddTrustedOrigin("https://example.com")

r.Use(stdgee.ProtectCrossOrigin(cop))
```

This is intentionally optional. The main teaching focus is still the Go 1.22 routing model.

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
