package stdgee

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// Context carries request/response data and convenience response helpers.
type Context struct {
	Writer  http.ResponseWriter
	Request *http.Request

	Path   string
	Method string

	StatusCode int

	engine *Engine
}

func newContext(w http.ResponseWriter, req *http.Request, engine *Engine) *Context {
	return &Context{
		Writer:  w,
		Request: req,
		Path:    req.URL.Path,
		Method:  req.Method,
		engine:  engine,
	}
}

// Param returns the named wildcard value from the matched ServeMux pattern.
func (c *Context) Param(key string) string {
	return c.Request.PathValue(key)
}

// Query returns the named query parameter.
func (c *Context) Query(key string) string {
	return c.Request.URL.Query().Get(key)
}

// PostForm returns the named form value.
func (c *Context) PostForm(key string) string {
	return c.Request.FormValue(key)
}

// Status writes the response status code.
func (c *Context) Status(code int) {
	c.StatusCode = code
	c.Writer.WriteHeader(code)
}

// SetHeader sets a response header.
func (c *Context) SetHeader(key string, value string) {
	c.Writer.Header().Set(key, value)
}

// String writes a plain text response.
func (c *Context) String(code int, format string, values ...any) {
	c.SetHeader("Content-Type", "text/plain; charset=utf-8")
	c.Status(code)
	_, _ = fmt.Fprintf(c.Writer, format, values...)
}

// Data writes raw bytes.
func (c *Context) Data(code int, data []byte) {
	c.Status(code)
	_, _ = c.Writer.Write(data)
}

// JSON writes a JSON response.
func (c *Context) JSON(code int, obj any) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(obj); err != nil {
		http.Error(c.Writer, err.Error(), http.StatusInternalServerError)
		return
	}

	c.SetHeader("Content-Type", "application/json; charset=utf-8")
	c.Status(code)
	_, _ = c.Writer.Write(buf.Bytes())
}

// HTML renders an HTML template by name.
func (c *Context) HTML(code int, name string, data any) {
	if c.engine == nil || c.engine.htmlTemplates == nil {
		http.Error(c.Writer, "no html templates configured", http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer
	if err := c.engine.htmlTemplates.ExecuteTemplate(&buf, name, data); err != nil {
		http.Error(c.Writer, err.Error(), http.StatusInternalServerError)
		return
	}

	c.SetHeader("Content-Type", "text/html; charset=utf-8")
	c.Status(code)
	_, _ = c.Writer.Write(buf.Bytes())
}
