package server

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/format/pktline"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/plumbing/transport"
	gitserver "github.com/go-git/go-git/v5/plumbing/transport/server"
	"github.com/go-git/go-git/v5/storage"
)

// singleRepoLoader wraps a single storer.Storer so that go-git's server
// transport can load it for any endpoint.
type singleRepoLoader struct {
	storer storage.Storer
}

func (l *singleRepoLoader) Load(_ *transport.Endpoint) (storer.Storer, error) {
	return l.storer, nil
}

func (s *Server) gitTransport() transport.Transport {
	return gitserver.NewServer(&singleRepoLoader{storer: s.store.Storer()})
}

// dummyEndpoint is used to satisfy go-git's session API. The actual repo
// is resolved by singleRepoLoader regardless of the endpoint value.
var dummyEndpoint = &transport.Endpoint{Protocol: "http", Path: "/"}

func (s *Server) handleGitInfoRefs(w http.ResponseWriter, r *http.Request) {
	service := r.URL.Query().Get("service")
	if service == "" {
		http.Error(w, "service parameter required", http.StatusBadRequest)
		return
	}

	srv := s.gitTransport()

	var adv *packp.AdvRefs
	var err error

	switch service {
	case "git-upload-pack":
		session, sessionErr := srv.NewUploadPackSession(dummyEndpoint, nil)
		if sessionErr != nil {
			slog.Error("upload-pack session", "error", sessionErr)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		defer session.Close()
		adv, err = session.AdvertisedReferencesContext(r.Context())

	case "git-receive-pack":
		session, sessionErr := srv.NewReceivePackSession(dummyEndpoint, nil)
		if sessionErr != nil {
			slog.Error("receive-pack session", "error", sessionErr)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		defer session.Close()
		adv, err = session.AdvertisedReferencesContext(r.Context())

	default:
		http.Error(w, "unknown service", http.StatusForbidden)
		return
	}

	if err != nil {
		slog.Error("advertise references", "service", service, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	adv.Prefix = [][]byte{
		[]byte("# service=" + service),
		pktline.Flush,
	}

	w.Header().Set("Content-Type", "application/x-"+service+"-advertisement")
	w.Header().Set("Cache-Control", "no-cache")
	if err := adv.Encode(w); err != nil {
		slog.Error("encode advertised refs", "error", err)
	}
}

func (s *Server) handleGitUploadPack(w http.ResponseWriter, r *http.Request) {
	srv := s.gitTransport()

	session, err := srv.NewUploadPackSession(dummyEndpoint, nil)
	if err != nil {
		slog.Error("upload-pack session", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	defer session.Close()

	req := packp.NewUploadPackRequest()
	if err := req.Decode(r.Body); err != nil {
		slog.Error("decode upload-pack request", "error", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	resp, err := session.UploadPack(r.Context(), req)
	if err != nil {
		slog.Error("upload-pack", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/x-git-upload-pack-result")
	if err := resp.Encode(w); err != nil {
		slog.Error("encode upload-pack response", "error", err)
	}
}

func (s *Server) handleGitReceivePack(w http.ResponseWriter, r *http.Request) {
	srv := s.gitTransport()

	session, err := srv.NewReceivePackSession(dummyEndpoint, nil)
	if err != nil {
		slog.Error("receive-pack session", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	defer session.Close()

	req := packp.NewReferenceUpdateRequest()
	if err := req.Decode(r.Body); err != nil {
		slog.Error("decode receive-pack request", "error", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	status, err := session.ReceivePack(r.Context(), req)
	if err != nil {
		if !strings.Contains(err.Error(), "update reference") {
			slog.Error("receive-pack", "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		slog.Warn("receive-pack partial failure", "error", err)
	}

	w.Header().Set("Content-Type", "application/x-git-receive-pack-result")
	if status != nil {
		if err := status.Encode(w); err != nil {
			slog.Error("encode receive-pack response", "error", err)
		}
	}

	// Async reindex after push
	go func() {
		if err := s.reindexer.ReIndex(); err != nil {
			slog.Error("post-push reindex", "error", err)
			return
		}
		if err := s.RefreshCache(); err != nil {
			slog.Error("post-push refresh cache", "error", err)
		}
		slog.Info("post-push reindex complete")
	}()
}
