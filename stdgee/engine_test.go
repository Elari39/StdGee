package stdgee

import (
	"bytes"
	"context"
	"errors"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRouteMatchingAndParams(t *testing.T) {
	engine := New()
	engine.GET("/ping", func(c *Context) {
		c.String(http.StatusOK, "pong")
	})

	v1 := engine.Group("/v1")
	v1.GET("/hello/{name}", func(c *Context) {
		c.String(http.StatusOK, "hello %s", c.Param("name"))
	})

	t.Run("ping", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)

		engine.ServeHTTP(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", recorder.Code)
		}
		if body := recorder.Body.String(); body != "pong" {
			t.Fatalf("expected pong, got %q", body)
		}
	})

	t.Run("path value", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v1/hello/gopher", nil)

		engine.ServeHTTP(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", recorder.Code)
		}
		if body := recorder.Body.String(); body != "hello gopher" {
			t.Fatalf("expected path value response, got %q", body)
		}
	})
}

func TestHeadAndMethodNotAllowed(t *testing.T) {
	engine := New()
	engine.GET("/ping", func(c *Context) {
		c.String(http.StatusCreated, "pong")
	})

	headRecorder := httptest.NewRecorder()
	headReq := httptest.NewRequest(http.MethodHead, "/ping", nil)
	engine.ServeHTTP(headRecorder, headReq)

	if headRecorder.Code != http.StatusCreated {
		t.Fatalf("expected HEAD to match GET with 201, got %d", headRecorder.Code)
	}

	postRecorder := httptest.NewRecorder()
	postReq := httptest.NewRequest(http.MethodPost, "/ping", nil)
	engine.ServeHTTP(postRecorder, postReq)

	if postRecorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", postRecorder.Code)
	}
	if allow := postRecorder.Header().Get("Allow"); !strings.Contains(allow, http.MethodGet) || !strings.Contains(allow, http.MethodHead) {
		t.Fatalf("expected Allow header to mention GET and HEAD, got %q", allow)
	}
}

func TestConflictPatternPanics(t *testing.T) {
	engine := New()
	engine.HandleFunc("GET /posts/{id}", func(w http.ResponseWriter, req *http.Request) {})

	defer func() {
		if recover() == nil {
			t.Fatal("expected conflicting pattern registration to panic")
		}
	}()

	engine.HandleFunc("GET /posts/{slug}", func(w http.ResponseWriter, req *http.Request) {})
}

func TestMiddlewareOrderRecoveryAndLogger(t *testing.T) {
	engine := New()

	var order []string
	engine.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			order = append(order, "root-before")
			next.ServeHTTP(w, req)
			order = append(order, "root-after")
		})
	})

	v1 := engine.Group("/v1")
	v1.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			order = append(order, "group-before")
			next.ServeHTTP(w, req)
			order = append(order, "group-after")
		})
	})

	v1.GET("/hello/{name}", func(c *Context) {
		order = append(order, "handler")
		c.String(http.StatusOK, "hello %s", c.Param("name"))
	})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/hello/stdgee", nil)
	engine.ServeHTTP(recorder, req)

	got := strings.Join(order, ",")
	want := "root-before,group-before,handler,group-after,root-after"
	if got != want {
		t.Fatalf("expected middleware order %q, got %q", want, got)
	}

	var logs bytes.Buffer
	prevWriter := log.Writer()
	prevFlags := log.Flags()
	log.SetOutput(&logs)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(prevWriter)
		log.SetFlags(prevFlags)
	})

	loggerEngine := New()
	loggerEngine.Use(Logger(), Recovery())
	loggerEngine.GET("/panic", func(c *Context) {
		panic("boom")
	})

	panicRecorder := httptest.NewRecorder()
	panicReq := httptest.NewRequest(http.MethodGet, "/panic", nil)
	loggerEngine.ServeHTTP(panicRecorder, panicReq)

	if panicRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected panic to become 500, got %d", panicRecorder.Code)
	}

	logOutput := logs.String()
	if !strings.Contains(logOutput, "[500] GET /panic") {
		t.Fatalf("expected logger output to contain 500 status, got %q", logOutput)
	}
	if !strings.Contains(logOutput, "[Recovery] panic recovered: boom") {
		t.Fatalf("expected recovery output to mention panic, got %q", logOutput)
	}
}

func TestResponseHelpers(t *testing.T) {
	t.Run("json", func(t *testing.T) {
		engine := New()
		engine.GET("/json", func(c *Context) {
			c.JSON(http.StatusAccepted, map[string]string{"status": "ok"})
		})

		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/json", nil)
		engine.ServeHTTP(recorder, req)

		if recorder.Code != http.StatusAccepted {
			t.Fatalf("expected 202, got %d", recorder.Code)
		}
		if contentType := recorder.Header().Get("Content-Type"); contentType != "application/json; charset=utf-8" {
			t.Fatalf("unexpected content type %q", contentType)
		}
		if !strings.Contains(recorder.Body.String(), `"status":"ok"`) {
			t.Fatalf("unexpected JSON body %q", recorder.Body.String())
		}
	})

	t.Run("html", func(t *testing.T) {
		engine := New()
		engine.SetFuncMap(template.FuncMap{
			"upper": strings.ToUpper,
		})

		templateDir := t.TempDir()
		templatePath := filepath.Join(templateDir, "index.html")
		if err := os.WriteFile(templatePath, []byte(`<h1>{{ upper .title }}</h1>`), 0o600); err != nil {
			t.Fatalf("write template: %v", err)
		}

		engine.LoadHTMLGlob(filepath.Join(templateDir, "*.html"))
		engine.GET("/", func(c *Context) {
			c.HTML(http.StatusOK, "index.html", map[string]string{"title": "stdgee"})
		})

		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		engine.ServeHTTP(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", recorder.Code)
		}
		if contentType := recorder.Header().Get("Content-Type"); contentType != "text/html; charset=utf-8" {
			t.Fatalf("unexpected content type %q", contentType)
		}
		if body := recorder.Body.String(); !strings.Contains(body, "<h1>STDGEE</h1>") {
			t.Fatalf("unexpected HTML body %q", body)
		}
	})

	t.Run("html without templates", func(t *testing.T) {
		engine := New()
		engine.GET("/", func(c *Context) {
			c.HTML(http.StatusOK, "index.html", nil)
		})

		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		engine.ServeHTTP(recorder, req)

		if recorder.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", recorder.Code)
		}
	})
}

func TestStaticAndShutdown(t *testing.T) {
	engine := New()

	staticDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(staticDir, "hello.txt"), []byte("hello from static"), 0o600); err != nil {
		t.Fatalf("write static file: %v", err)
	}

	engine.Static("/assets", staticDir)

	staticRecorder := httptest.NewRecorder()
	staticReq := httptest.NewRequest(http.MethodGet, "/assets/hello.txt", nil)
	engine.ServeHTTP(staticRecorder, staticReq)

	if staticRecorder.Code != http.StatusOK {
		t.Fatalf("expected static file 200, got %d", staticRecorder.Code)
	}
	if body := staticRecorder.Body.String(); body != "hello from static" {
		t.Fatalf("unexpected static body %q", body)
	}

	engine.GET("/ping", func(c *Context) {
		c.String(http.StatusOK, "pong")
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	server := &http.Server{Handler: engine}
	engine.server = server

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.Serve(listener)
	}()

	resp, err := http.Get("http://" + listener.Addr().String() + "/ping")
	if err != nil {
		t.Fatalf("initial request failed: %v", err)
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if string(body) != "pong" {
		t.Fatalf("unexpected body %q", string(body))
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := engine.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}

	select {
	case err := <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) {
			t.Fatalf("expected ErrServerClosed, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not shut down in time")
	}
}
