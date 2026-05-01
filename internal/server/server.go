package server

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/open-spaced-repetition/go-fsrs/v3"
	"github.com/raphi011/kb/internal/gitrepo"
	"github.com/raphi011/kb/internal/index"
	"github.com/raphi011/kb/internal/markdown"
	"github.com/raphi011/kb/internal/server/views"
	"github.com/raphi011/kb/internal/srs"
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
	BookmarkedPaths() ([]string, error)
	AddBookmark(path string) error
	RemoveBookmark(path string) error
	ShareNote(path string) (string, error)
	UnshareNote(path string) error
	ShareTokenForNote(path string) (string, error)
	NotePathForShareToken(token string) (string, error)
	RenderWithTags(src []byte, tags []string) (markdown.RenderResult, error)
	RenderShared(src []byte) (markdown.RenderResult, error)
	RenderPreview(src []byte) (markdown.RenderResult, error)
	DueCards(notePath string, limit int) ([]srs.Card, error)
	CardByHash(hash string) (srs.Card, error)
	ReviewCard(hash string, rating fsrs.Rating) (srs.Card, error)
	PreviewCard(hash string) (srs.Previews, error)
	FlashcardStats() (srs.Stats, error)
	FlashcardsForNote(path string) ([]srs.Card, error)
	NotesWithFlashcards() ([]index.NoteFlashcardCount, error)
	ReviewSummaryForNote(notePath string) (index.ReviewSummary, error)
	CardOverviewsForNote(notePath string) ([]index.CardOverview, error)
	IndexSHA() (string, error)
}

// ReIndexer refreshes the git HEAD and re-indexes changed notes.
type ReIndexer interface {
	ReIndex() error
	ForceReIndex() error
}

// Syncer fetches from origin and fast-forwards local heads.
type Syncer interface {
	Sync(ctx context.Context, token string) (*gitrepo.SyncResult, error)
}

type Server struct {
	mux         *http.ServeMux
	handler     http.Handler
	store       Store
	reindexer   ReIndexer
	syncer      Syncer
	token       string
	originToken string
	repoPath    string
	cache       atomic.Pointer[noteCache]
	renderCache *renderCache
}

func New(store Store, reindexer ReIndexer, syncer Syncer, token, originToken, repoPath string) (*Server, error) {
	if token == "" {
		slog.Warn("kb running with authentication disabled; do not expose without an external auth proxy in front")
	}
	loadAssetManifest()
	cache, err := buildNoteCache(store)
	if err != nil {
		return nil, fmt.Errorf("build cache: %w", err)
	}
	s := &Server{
		mux:         http.NewServeMux(),
		store:       store,
		reindexer:   reindexer,
		syncer:      syncer,
		token:       token,
		originToken: originToken,
		repoPath:    repoPath,
		renderCache: newRenderCache(),
	}
	s.cache.Store(cache)
	if err := s.registerRoutes(); err != nil {
		return nil, err
	}
	s.handler = gzipMiddleware(s.authMiddleware(s.mux))
	return s, nil
}

// loadAssetManifest reads the build-time asset fingerprint manifest (if
// present) and installs it in the views package. When absent — typically a
// dev build that ran esbuild but skipped genassets — Asset() falls through
// to un-fingerprinted /static/<name> URLs.
func loadAssetManifest() {
	data, err := fs.ReadFile(staticFS, "static/dist/manifest.json")
	if err != nil {
		return
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		slog.Warn("asset manifest unparseable; serving un-fingerprinted assets", "err", err)
		return
	}
	views.SetAssets(m)
}

func (s *Server) registerRoutes() error {
	staticSub, err := fs.Sub(staticFS, "static")
	if err != nil {
		return fmt.Errorf("static fs: %w", err)
	}
	distSub, err := fs.Sub(staticSub, "dist")
	if err != nil {
		return fmt.Errorf("static dist fs: %w", err)
	}
	// Fingerprinted assets: safe to cache forever — the URL changes when the
	// content does. Registered before the broader /static/ route; ServeMux
	// picks the longest matching prefix.
	s.mux.Handle("GET /static/dist/", cacheControl("public, max-age=31536000, immutable",
		http.StripPrefix("/static/dist/", preGzipFileServer(distSub))))
	// Un-fingerprinted source assets (dev mode + sourcemap source resolution):
	// must revalidate every time so updates are picked up.
	s.mux.Handle("GET /static/", cacheControl("public, max-age=0, must-revalidate",
		http.StripPrefix("/static/", preGzipFileServer(staticSub))))
	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	s.mux.HandleFunc("GET /login", s.handleLoginPage)
	s.mux.HandleFunc("POST /login", s.handleLoginSubmit)
	s.mux.HandleFunc("GET /{$}", s.handleIndex)
	s.mux.HandleFunc("GET /search", s.handleSearch)
	s.mux.HandleFunc("GET /calendar", s.handleCalendar)
	s.mux.HandleFunc("GET /tags", s.handleTags)
	s.mux.HandleFunc("GET /notes/{path...}", s.handleNote)
	s.mux.HandleFunc("GET /bookmarks/panel", s.handleBookmarksPanel)
	s.mux.HandleFunc("PUT /api/bookmarks/{path...}", s.handleBookmarkPut)
	s.mux.HandleFunc("DELETE /api/bookmarks/{path...}", s.handleBookmarkDelete)
	s.mux.HandleFunc("POST /api/share/{path...}", s.handleShareCreate)
	s.mux.HandleFunc("DELETE /api/share/{path...}", s.handleShareDelete)
	s.mux.HandleFunc("GET /api/share/{path...}", s.handleShareGet)
	s.mux.HandleFunc("GET /s/{token}", s.handleSharedNote)
	s.mux.HandleFunc("GET /git/info/refs", s.handleGitInfoRefs)
	s.mux.HandleFunc("POST /git/git-upload-pack", s.handleGitUploadPack)
	s.mux.HandleFunc("POST /git/git-receive-pack", s.handleGitReceivePack)
	s.mux.HandleFunc("GET /flashcards", s.handleFlashcardDashboard)
	s.mux.HandleFunc("GET /flashcards/review", s.handleFlashcardReview)
	s.mux.HandleFunc("POST /flashcards/review/{hash}", s.handleFlashcardRate)
	s.mux.HandleFunc("GET /flashcards/note/{path...}", s.handleFlashcardsForNote)
	s.mux.HandleFunc("GET /api/flashcards/stats", s.handleFlashcardStatsAPI)
	s.mux.HandleFunc("GET /settings", s.handleSettings)
	s.mux.HandleFunc("POST /api/settings/sync", s.handleSync)
	s.mux.HandleFunc("POST /api/settings/reindex", s.handleForceReindex)
	s.mux.HandleFunc("GET /preview/{path...}", s.handlePreview)
	s.mux.HandleFunc("GET /api/panels/{path...}", s.handleNotePanels)
	s.mux.HandleFunc("GET /api/git/history/{path...}", s.handleGitHistory)
	s.mux.HandleFunc("GET /api/git/version/{hash}/{path...}", s.handleGitVersion)
	return nil
}

func cacheControl(value string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", value)
		next.ServeHTTP(w, r)
	})
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
	s.renderCache.clear()
	return nil
}

func (s *Server) noteCache() *noteCache {
	return s.cache.Load()
}
