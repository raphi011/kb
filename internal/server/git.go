package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"os/exec"
	"strings"
)

func (s *Server) handleGitInfoRefs(w http.ResponseWriter, r *http.Request) {
	service := r.URL.Query().Get("service")
	if service != "git-upload-pack" && service != "git-receive-pack" {
		http.Error(w, "invalid service", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", fmt.Sprintf("application/x-%s-advertisement", service))
	w.Header().Set("Cache-Control", "no-cache")

	// Write pkt-line service announcement + flush-pkt.
	pkt := fmt.Sprintf("# service=%s\n", service)
	fmt.Fprintf(w, "%04x%s0000", len(pkt)+4, pkt)

	// service is "git-upload-pack" or "git-receive-pack"; strip "git-" prefix
	// to form the git subcommand (e.g. "git upload-pack").
	subcmd := strings.TrimPrefix(service, "git-")
	cmd := exec.CommandContext(r.Context(), "git", subcmd, "--stateless-rpc", "--advertise-refs", s.repoPath)
	cmd.Stdout = w
	if err := cmd.Run(); err != nil {
		slog.Error("git advertise-refs", "service", service, "error", err)
	}
}

func (s *Server) handleGitUploadPack(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/x-git-upload-pack-result")
	s.gitServiceRPC(w, r, "git-upload-pack")
}

func (s *Server) handleGitReceivePack(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/x-git-receive-pack-result")

	if err := s.gitServiceRPC(w, r, "git-receive-pack"); err != nil {
		return
	}

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

func (s *Server) gitServiceRPC(w http.ResponseWriter, r *http.Request, service string) error {
	defer r.Body.Close()

	subcmd := strings.TrimPrefix(service, "git-")
	cmd := exec.CommandContext(r.Context(), "git", subcmd, "--stateless-rpc", s.repoPath)
	cmd.Stdin = r.Body
	cmd.Stdout = w

	if err := cmd.Run(); err != nil {
		slog.Error("git rpc", "service", service, "error", err)
		return err
	}
	return nil
}
