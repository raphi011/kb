package server

import (
	"net/http"
)

func (s *Server) handleGitInfoRefs(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "git remote not yet wired", http.StatusNotImplemented)
}

func (s *Server) handleGitUploadPack(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "git remote not yet wired", http.StatusNotImplemented)
}

func (s *Server) handleGitReceivePack(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "git remote not yet wired", http.StatusNotImplemented)
}
