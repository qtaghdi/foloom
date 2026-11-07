package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const recoveryRoute = "/api/documents/docx-recovery"

func (s *Server) docxRecoveryRoutes() {
	if s.scoped {
		return
	}
	s.mux.Handle("GET "+recoveryRoute, s.requireSession(http.HandlerFunc(s.listDOCXRecovery)))
	s.mux.Handle("POST "+recoveryRoute, s.requireSession(http.HandlerFunc(s.checkpointDOCXRecovery)))
	s.mux.Handle("POST "+recoveryRoute+"/{id}/open", s.requireSession(http.HandlerFunc(s.reopenDOCXRecovery)))
	s.mux.Handle("DELETE "+recoveryRoute+"/{id}", s.requireSession(http.HandlerFunc(s.deleteDOCXRecovery)))
}

func (s *Server) listDOCXRecovery(w http.ResponseWriter, r *http.Request) {
	s.openMu.Lock()
	defer s.openMu.Unlock()
	entries := []recoveryEntry{}
	root, err := s.recoveryRoot(false)
	if err == nil {
		entries, _, err = scanRecovery(root, time.Now())
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("복구 사본 목록을 읽지 못했습니다"))
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, entries)
}

func (s *Server) checkpointDOCXRecovery(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var input savedFileInput
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil || decoder.Decode(new(any)) != io.EOF || !recoveryID.MatchString(input.SessionID) || input.BaseRevision == 0 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("복구할 문서와 기준 리비전을 지정하세요"))
		return
	}
	s.openMu.Lock()
	defer s.openMu.Unlock()
	s.sessionMu.RLock()
	source := s.documents[input.SessionID]
	s.sessionMu.RUnlock()
	if source == nil || source.Kind != "docx" {
		writeError(w, http.StatusNotFound, fmt.Errorf("복구할 DOCX 문서가 열려 있지 않습니다"))
		return
	}
	_, content, _, err := source.server.savedFile(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusConflict, err)
		return
	}
	name, err := docxName(source.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	root, err := s.recoveryRoot(true)
	now := time.Now().UTC()
	if err == nil {
		var count int
		_, count, err = scanRecovery(root, now)
		_, _, existing := readRecovery(root, input.SessionID, false)
		if err == nil && count >= maxRecoveryCount && existing != nil {
			writeError(w, http.StatusConflict, fmt.Errorf("복구 사본은 최대 3개입니다. 기존 사본 하나를 삭제하세요"))
			return
		}
	}
	entry := recoveryEntry{ID: input.SessionID, Name: name, UpdatedAt: now}
	if err == nil {
		err = writeRecovery(root, entry, content)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("복구 사본을 보관하지 못했습니다"))
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, entry)
}

func (s *Server) reopenDOCXRecovery(w http.ResponseWriter, r *http.Request) {
	s.openMu.Lock()
	defer s.openMu.Unlock()
	if err := s.checkDocumentLimit(); err != nil {
		writeError(w, http.StatusConflict, err)
		return
	}
	root, err := s.recoveryRoot(false)
	var entry recoveryEntry
	var content []byte
	if err == nil {
		if _, _, err = scanRecovery(root, time.Now()); err == nil {
			entry, content, err = readRecovery(root, r.PathValue("id"), true)
		}
	}
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Errorf("열 수 있는 DOCX 복구 사본이 없습니다"))
		return
	}
	for attempt := 1; attempt <= maxOpenDocuments+1; attempt++ {
		name := recoveryName(entry, attempt)
		if s.checkDocumentName(name) != nil {
			continue
		}
		opened, err := s.openDOCXCopy(r.Context(), name, "recovery", content)
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Errorf("복구 사본을 열지 못했습니다"))
			return
		}
		writeJSON(w, http.StatusCreated, opened)
		return
	}
	writeError(w, http.StatusConflict, fmt.Errorf("복구 사본 이름을 만들지 못했습니다"))
}

func (s *Server) deleteDOCXRecovery(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !recoveryID.MatchString(id) {
		http.NotFound(w, r)
		return
	}
	s.openMu.Lock()
	defer s.openMu.Unlock()
	root, err := s.recoveryRoot(false)
	if err == nil {
		err = os.Remove(recoveryPath(root, id))
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("복구 사본을 삭제하지 못했습니다"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
