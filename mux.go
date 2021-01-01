package mux

import (
	"context"
	"fmt"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"
)

type Server struct {
	HTTPServer                    *http.Server
	prehandlers                   []func(http.ResponseWriter, *http.Request) bool
	r, mr                         map[string]func(http.ResponseWriter, *http.Request)
	get, post, put, delete, patch map[string]func(http.ResponseWriter, *http.Request)
}

func NewServer(addr string) *Server {
	s := &Server{}
	s.HTTPServer = &http.Server{Addr: addr, Handler: s}
	s.r = make(map[string]func(http.ResponseWriter, *http.Request))
	s.mr = make(map[string]func(http.ResponseWriter, *http.Request))
	s.get = make(map[string]func(http.ResponseWriter, *http.Request))
	s.post = make(map[string]func(http.ResponseWriter, *http.Request))
	s.put = make(map[string]func(http.ResponseWriter, *http.Request))
	s.delete = make(map[string]func(http.ResponseWriter, *http.Request))
	s.patch = make(map[string]func(http.ResponseWriter, *http.Request))
	return s
}

func (s *Server) ListenAndServe() error {
	return s.HTTPServer.ListenAndServe()
}

func (s *Server) Stop() error {
	if s != nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		// Doesn't block if no connections, but will otherwise wait
		// until the timeout deadline.
		e := s.HTTPServer.Shutdown(ctx)
		return e
	}
	return nil
}

func (s *Server) GET(url string, f func(http.ResponseWriter, *http.Request)) {
	s.get[url] = f
}

func (s *Server) POST(url string, f func(http.ResponseWriter, *http.Request)) {
	s.post[url] = f
}

func (s *Server) PUT(url string, f func(http.ResponseWriter, *http.Request)) {
	s.put[url] = f
}

func (s *Server) DELETE(url string, f func(http.ResponseWriter, *http.Request)) {
	s.delete[url] = f
}

func (s *Server) PATCH(url string, f func(http.ResponseWriter, *http.Request)) {
	s.patch[url] = f
}

func (s *Server) HandleFunc(url string, f func(http.ResponseWriter, *http.Request)) {
	s.r[url] = f
}

func (s *Server) ServeBytes(url string, bytes []byte) {
	s.r[url] = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", mime.TypeByExtension(path.Ext(url)))
		w.Write(bytes)
	}
}

func (s *Server) ServeFile(uri string, path string) {
	s.r[uri] = func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, path)
	}
}

func (s *Server) HandleWoff(url string, bytes []byte) {
	s.r[url] = func(w http.ResponseWriter, r *http.Request) {
		SetWoffHeader(w)
		w.Write(bytes)
	}
}

func (s *Server) HandleHtml(url string, text []byte) {
	s.r[url] = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write(text)
	}
}

func (s *Server) HandleHtmlFunc(url string, fn func(w http.ResponseWriter, r *http.Request)) {
	s.r[url] = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fn(w, r)
	}
}

func (s *Server) HandleJs(url string, text []byte) {
	s.r[url] = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		w.Write(text)
	}
}
func (s *Server) HandleCss(url string, text []byte) {
	s.r[url] = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css")
		w.Write(text)
	}
}
func (s *Server) HandleSvg(url string, text []byte) {
	s.r[url] = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Write(text)
	}
}

func (s *Server) HandleMultiReqs(url string, f func(http.ResponseWriter, *http.Request)) {
	s.mr[url] = f
}

func (s *Server) Handle(pattern string, h http.Handler) {
	s.r[pattern] = h.ServeHTTP
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for _, v := range s.prehandlers {
		interrupt := v(w, r)
		if interrupt {
			return
		}
	}
	url := strings.Split(r.URL.String(), "?")[0]

	switch r.Method {
	case http.MethodGet:
		if h, ok := s.get[url]; ok {
			h(w, r)
			return
		}
	case http.MethodPost:
		if h, ok := s.post[url]; ok {
			h(w, r)
			return
		}
	case http.MethodPut:
		if h, ok := s.put[url]; ok {
			h(w, r)
			return
		}
	case http.MethodDelete:
		if h, ok := s.delete[url]; ok {
			h(w, r)
			return
		}
	case http.MethodPatch:
		if h, ok := s.patch[url]; ok {
			h(w, r)
			return
		}
	}

	if h, ok := s.r[url]; ok {
		h(w, r)
	} else if k, ok := hasPreffixInMap(s.mr, r.URL.String()); ok {
		s.mr[k](w, r)
	} else {
		fmt.Fprint(w, `<!DOCTYPE html><html><head><title>404</title><meta charset="utf-8"><meta name="viewpos" content="width=device-width"></head><body>404 not found</body></html>`)
	}
}

func (s *Server) findMethod(url string) (string, func(http.ResponseWriter, *http.Request), bool) {

	return "", nil, false
}

func hasPreffixInMap(m map[string]func(http.ResponseWriter, *http.Request), p string) (string, bool) {
	for k, _ := range m {
		if len(p) >= len(k) && k == p[:len(k)] {
			return k, true
		}
	}
	return "", false
}

// AddPrehandler adds prehandler which returns interrupt
func (s *Server) AddPrehandler(f func(http.ResponseWriter, *http.Request) bool) {
	s.prehandlers = append(s.prehandlers, f)
}

// AddRoutes adds all s2's routes to server
func (s *Server) AddRoutes(s2 *Server) {
	for k, v := range s2.r {
		_, ok := s.r[k]
		if !ok {
			s.r[k] = v
		}
	}

	for k, v := range s2.mr {
		_, ok := s.mr[k]
		if !ok {
			s.mr[k] = v
		}
	}
}
