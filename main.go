package main

import (
	"context"
	"encoding/json"
	"errors"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gee-web-std/stdgee"
)

func main() {
	r := stdgee.New()
	r.Use(stdgee.Logger(), stdgee.Recovery())

	r.SetFuncMap(template.FuncMap{
		"FormatAsDateTime": func(t time.Time) string {
			return t.Format("2006-01-02 15:04:05")
		},
	})
	r.LoadHTMLGlob("templates/*")
	r.Static("/assets", "./static")

	r.GET("/", func(c *stdgee.Context) {
		c.HTML(http.StatusOK, "index.html", map[string]any{
			"title":   "StdGee on net/http",
			"message": "This demo keeps Gee's learning path, but routes with the modern standard library.",
			"now":     time.Now(),
			"items": []string{
				"GET /ping",
				"GET /posts/",
				"GET /v1/hello/gopher",
				"GET /assets/hello.txt",
			},
		})
	})

	r.HandleFunc("GET /ping", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("pong\n"))
	})

	r.HandleFunc("GET /posts/{$}", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("posts index\n"))
	})

	v1 := r.Group("/v1")
	v1.GET("/hello/{name}", func(c *stdgee.Context) {
		c.String(http.StatusOK, "hello %s, you are at %s\n", c.Param("name"), c.Path)
	})

	v1.HandleFunc("GET /json", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"message": "hello from stdgee",
			"ts":      time.Now().Unix(),
			"path":    req.URL.Path,
			"route":   req.Pattern,
		})
	})

	v1.POST("/login", func(c *stdgee.Context) {
		c.JSON(http.StatusOK, map[string]any{
			"username": c.PostForm("username"),
			"password": c.PostForm("password"),
			"status":   "ok",
		})
	})

	v1.GET("/panic", func(c *stdgee.Context) {
		panic("intentional panic for recovery demo")
	})

	// Optional CSRF protection based on Go 1.25+ net/http.
	//
	// cop := http.NewCrossOriginProtection()
	// cop.AddTrustedOrigin("https://example.com")
	// r.Use(stdgee.ProtectCrossOrigin(cop))

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- r.Run(":9999")
	}()

	log.Println("StdGee demo server listening on http://localhost:9999")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	case <-ctx.Done():
		log.Println("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := r.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("shutdown failed: %v", err)
	}
}
