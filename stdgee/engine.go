package stdgee

import (
	"context"
	"errors"
	"html/template"
	"net/http"
	"strings"
	"sync"
)

// Middleware wraps an http.Handler and returns a new handler.
type Middleware func(http.Handler) http.Handler

// ContextHandler keeps a Gee-like handler shape for helpers such as String and JSON.
type ContextHandler func(*Context)

// Engine is the main framework instance. It delegates routing to http.ServeMux.
type Engine struct {
	mux *http.ServeMux

	*RouterGroup

	htmlTemplates *template.Template
	funcMap       template.FuncMap

	mu     sync.Mutex
	server *http.Server
}

// RouterGroup groups routes with a common prefix and middleware chain.
type RouterGroup struct {
	prefix      string
	parent      *RouterGroup
	middlewares []Middleware
	engine      *Engine
}

// New creates a new Engine.
func New() *Engine {
	engine := &Engine{
		mux: http.NewServeMux(),
	}
	engine.RouterGroup = &RouterGroup{
		engine: engine,
	}
	return engine
}

// ServeHTTP implements http.Handler.
func (engine *Engine) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	engine.mux.ServeHTTP(w, req)
}

// Handle registers a standard library handler with a ServeMux pattern.
func (engine *Engine) Handle(pattern string, h http.Handler) {
	engine.RouterGroup.Handle(pattern, h)
}

// HandleFunc registers a standard library handler function with a ServeMux pattern.
func (engine *Engine) HandleFunc(pattern string, fn func(http.ResponseWriter, *http.Request)) {
	engine.RouterGroup.HandleFunc(pattern, fn)
}

// GET registers a Gee-style handler for the GET method.
func (engine *Engine) GET(path string, handler ContextHandler) {
	engine.RouterGroup.GET(path, handler)
}

// POST registers a Gee-style handler for the POST method.
func (engine *Engine) POST(path string, handler ContextHandler) {
	engine.RouterGroup.POST(path, handler)
}

// Group creates a sub group with a shared path prefix.
func (engine *Engine) Group(prefix string) *RouterGroup {
	return engine.RouterGroup.Group(prefix)
}

// Use appends middleware to the root group.
func (engine *Engine) Use(middlewares ...Middleware) {
	engine.RouterGroup.Use(middlewares...)
}

// SetFuncMap sets the FuncMap used during template parsing.
func (engine *Engine) SetFuncMap(funcMap template.FuncMap) {
	engine.funcMap = funcMap
}

// LoadHTMLGlob parses templates from a glob pattern.
func (engine *Engine) LoadHTMLGlob(pattern string) {
	engine.htmlTemplates = template.Must(template.New("").Funcs(engine.funcMap).ParseGlob(pattern))
}

// Run starts the HTTP server.
func (engine *Engine) Run(addr string) error {
	server := &http.Server{
		Addr:    addr,
		Handler: engine,
	}

	engine.mu.Lock()
	engine.server = server
	engine.mu.Unlock()

	err := server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Shutdown gracefully stops the HTTP server.
func (engine *Engine) Shutdown(ctx context.Context) error {
	engine.mu.Lock()
	server := engine.server
	engine.mu.Unlock()

	if server == nil {
		return nil
	}
	return server.Shutdown(ctx)
}

// Group creates a child RouterGroup with an inherited prefix.
func (group *RouterGroup) Group(prefix string) *RouterGroup {
	return &RouterGroup{
		prefix: normalizeGroupPrefix(joinPath(group.prefix, prefix)),
		parent: group,
		engine: group.engine,
	}
}

// Use appends middleware to the current group.
func (group *RouterGroup) Use(middlewares ...Middleware) {
	group.middlewares = append(group.middlewares, middlewares...)
}

// Handle registers a standard library handler on the current group.
func (group *RouterGroup) Handle(pattern string, h http.Handler) {
	group.engine.mux.Handle(group.qualifyPattern(pattern), wrapMiddlewares(h, group.collectMiddlewares()))
}

// HandleFunc registers a standard library handler function on the current group.
func (group *RouterGroup) HandleFunc(pattern string, fn func(http.ResponseWriter, *http.Request)) {
	group.Handle(pattern, http.HandlerFunc(fn))
}

// GET registers a ContextHandler on the current group.
func (group *RouterGroup) GET(path string, handler ContextHandler) {
	group.Handle("GET "+path, group.contextHandler(handler))
}

// POST registers a ContextHandler on the current group.
func (group *RouterGroup) POST(path string, handler ContextHandler) {
	group.Handle("POST "+path, group.contextHandler(handler))
}

// Static serves files under the given mount path.
func (group *RouterGroup) Static(relativePath string, root string) {
	mountPath := joinPath(group.prefix, relativePath)
	fileServer := http.StripPrefix(mountPath, http.FileServer(http.Dir(root)))
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		fileServer.ServeHTTP(w, req)
	})

	base := strings.TrimSuffix(relativePath, "/") + "/"
	group.Handle("GET "+base+"{$}", handler)
	group.Handle("GET "+base+"{filepath...}", handler)
}

func (group *RouterGroup) contextHandler(handler ContextHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		handler(newContext(w, req, group.engine))
	})
}

func (group *RouterGroup) collectMiddlewares() []Middleware {
	var groups []*RouterGroup
	for current := group; current != nil; current = current.parent {
		groups = append(groups, current)
	}

	size := 0
	for i := len(groups) - 1; i >= 0; i-- {
		size += len(groups[i].middlewares)
	}

	chain := make([]Middleware, 0, size)
	for i := len(groups) - 1; i >= 0; i-- {
		chain = append(chain, groups[i].middlewares...)
	}
	return chain
}

func (group *RouterGroup) qualifyPattern(pattern string) string {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		panic("stdgee: route pattern cannot be empty")
	}

	if group.prefix == "" {
		return pattern
	}

	method, path, hasMethod := splitPattern(pattern)
	if !strings.HasPrefix(path, "/") {
		panic("stdgee: grouped patterns must use path-based ServeMux syntax")
	}

	qualifiedPath := joinPath(group.prefix, path)
	if !hasMethod {
		return qualifiedPath
	}
	return method + " " + qualifiedPath
}

func splitPattern(pattern string) (method string, path string, hasMethod bool) {
	if idx := strings.IndexByte(pattern, ' '); idx >= 0 {
		method = strings.TrimSpace(pattern[:idx])
		path = strings.TrimSpace(pattern[idx+1:])
		return method, path, true
	}
	return "", pattern, false
}

func normalizeGroupPrefix(prefix string) string {
	if prefix == "" || prefix == "/" {
		return ""
	}
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	return strings.TrimRight(prefix, "/")
}

func joinPath(prefix string, route string) string {
	switch {
	case prefix == "" || prefix == "/":
		if route == "" {
			return "/"
		}
		if strings.HasPrefix(route, "/") {
			return route
		}
		return "/" + route
	case route == "" || route == "/":
		return prefix
	default:
		return strings.TrimRight(prefix, "/") + "/" + strings.TrimLeft(route, "/")
	}
}

func wrapMiddlewares(handler http.Handler, middlewares []Middleware) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}
