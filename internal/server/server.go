package server

import (
	"bytes"
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"sync/atomic"
	"time"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/raphi011/kb/internal/index"
	"github.com/raphi011/kb/internal/markdown"
)

//go:embed static
var staticFS embed.FS

// Store is the data-access interface consumed by the server.
type Store interface {
	AllNotes() ([]index.Note, error)
	AllTags() ([]index.Tag, error)
	Search(q string, tags []string) ([]index.Note, error)
	NoteByPath(path string) (*index.Note, error)
	OutgoingLinks(path string) ([]index.Link, error)
	Backlinks(path string) ([]index.Link, error)
	ActivityDays(year, month int) (map[int]bool, error)
	NotesByDate(date string) ([]index.Note, error)
	ReadFile(path string) ([]byte, error)
	Render(src []byte) (markdown.RenderResult, error)
}

type Server struct {
	mux         *http.ServeMux
	handler     http.Handler
	store       Store
	token       string
	cache       atomic.Pointer[noteCache]
	chromaDark  []byte
	chromaLight []byte
}

func New(store Store, token string) (*Server, error) {
	dark, err := buildChromaCSS("dracula")
	if err != nil {
		return nil, fmt.Errorf("chroma dark css: %w", err)
	}
	light, err := buildChromaCSS("github")
	if err != nil {
		return nil, fmt.Errorf("chroma light css: %w", err)
	}
	cache, err := buildNoteCache(store)
	if err != nil {
		return nil, fmt.Errorf("build cache: %w", err)
	}
	s := &Server{
		mux:         http.NewServeMux(),
		store:       store,
		token:       token,
		chromaDark:  dark,
		chromaLight: light,
	}
	s.cache.Store(cache)
	s.registerRoutes()
	s.handler = s.authMiddleware(s.mux)
	return s, nil
}

func buildChromaCSS(styleName string) ([]byte, error) {
	style := styles.Get(styleName)
	if style == nil {
		style = styles.Fallback
	}
	var buf bytes.Buffer
	if err := chromahtml.New(chromahtml.WithClasses(true)).WriteCSS(&buf, style); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (s *Server) registerRoutes() {
	staticSub, _ := fs.Sub(staticFS, "static")
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))
	s.mux.HandleFunc("GET /static/chroma.css", s.handleChromaCSS)
	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	s.mux.HandleFunc("GET /login", s.handleLoginPage)
	s.mux.HandleFunc("POST /login", s.handleLoginSubmit)
	s.mux.HandleFunc("GET /{$}", s.handleIndex)
	s.mux.HandleFunc("GET /search", s.handleSearch)
	s.mux.HandleFunc("GET /calendar", s.handleCalendar)
	s.mux.HandleFunc("GET /tags", s.handleTags)
	s.mux.HandleFunc("GET /notes/{path...}", s.handleNote)
	s.mux.HandleFunc("GET /git/info/refs", s.handleGitInfoRefs)
	s.mux.HandleFunc("POST /git/git-upload-pack", s.handleGitUploadPack)
	s.mux.HandleFunc("POST /git/git-receive-pack", s.handleGitReceivePack)
}

func (s *Server) handleChromaCSS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Write(scopeChromaCSS(s.chromaDark, `html:not([data-theme="light"]) `))
	w.Write(scopeChromaCSS(s.chromaLight, `[data-theme="light"] `))
}

func scopeChromaCSS(css []byte, scope string) []byte {
	var out bytes.Buffer
	for _, line := range bytes.Split(css, []byte("\n")) {
		if idx := bytes.Index(line, []byte(".chroma")); idx >= 0 {
			out.Write(line[:idx])
			out.WriteString(scope)
			out.Write(line[idx:])
		} else {
			out.Write(line)
		}
		out.WriteByte('\n')
	}
	return out.Bytes()
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}

func (s *Server) ListenAndServe(ctx context.Context, addr string) error {
	srv := &http.Server{Addr: addr, Handler: s}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Println("shutting down...")
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutCtx); err != nil {
			return fmt.Errorf("shutdown: %w", err)
		}
		if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}

func (s *Server) RefreshCache() error {
	cache, err := buildNoteCache(s.store)
	if err != nil {
		return err
	}
	s.cache.Store(cache)
	return nil
}

func (s *Server) noteCache() *noteCache {
	return s.cache.Load()
}
