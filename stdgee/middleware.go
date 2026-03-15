package stdgee

import (
	"bufio"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"runtime/debug"
	"time"
)

type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func newStatusWriter(w http.ResponseWriter) *statusWriter {
	return &statusWriter{ResponseWriter: w}
}

func (w *statusWriter) WriteHeader(status int) {
	if !w.wroteHeader {
		w.status = status
		w.wroteHeader = true
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(p []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(p)
}

func (w *statusWriter) ReadFrom(r io.Reader) (int64, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if rf, ok := w.ResponseWriter.(io.ReaderFrom); ok {
		return rf.ReadFrom(r)
	}
	return io.Copy(w.ResponseWriter, r)
}

func (w *statusWriter) Flush() {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("stdgee: hijacker not supported")
	}
	return hijacker.Hijack()
}

func (w *statusWriter) Push(target string, opts *http.PushOptions) error {
	pusher, ok := w.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return pusher.Push(target, opts)
}

func (w *statusWriter) Status() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

// Logger logs the final status code, method, route pattern, and latency.
func Logger() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			recorder := newStatusWriter(w)
			start := time.Now()

			next.ServeHTTP(recorder, req)

			pattern := req.Pattern
			if pattern == "" {
				pattern = req.URL.Path
			}

			log.Printf("[%d] %s %s pattern=%q %v", recorder.Status(), req.Method, req.RequestURI, pattern, time.Since(start))
		})
	}
}

// Recovery converts panics into HTTP 500 responses and logs the stack trace.
func Recovery() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					log.Printf("[Recovery] panic recovered: %v\n%s", err, debug.Stack())

					if recorder, ok := w.(*statusWriter); ok && recorder.wroteHeader {
						return
					}
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				}
			}()

			next.ServeHTTP(w, req)
		})
	}
}

// ProtectCrossOrigin wraps a handler with Go 1.25+ CrossOriginProtection support.
func ProtectCrossOrigin(cop *http.CrossOriginProtection) Middleware {
	if cop == nil {
		cop = http.NewCrossOriginProtection()
	}
	return func(next http.Handler) http.Handler {
		return cop.Handler(next)
	}
}
