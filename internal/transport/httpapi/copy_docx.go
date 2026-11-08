package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/qtaghdi/document-workspace/internal/formats/docx"
)

func (s *Server) copyDOCX(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	if s.scoped {
		http.NotFound(w, r)
		return
	}
	var input struct {
		savedFileInput
		Name string `json:"name"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil || decoder.Decode(new(any)) != io.EOF || input.SessionID == "" || input.BaseRevision == 0 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("복사할 문서와 저장된 리비전을 지정하세요"))
		return
	}
	name, err := docxName(input.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	s.openMu.Lock()
	defer s.openMu.Unlock()
	if err = s.checkDocumentLimit(); err == nil {
		err = s.checkDocumentName(name)
	}
	if err != nil {
		writeError(w, http.StatusConflict, err)
		return
	}
	s.sessionMu.RLock()
	source := s.documents[input.SessionID]
	s.sessionMu.RUnlock()
	if source == nil || source.Kind != "docx" {
		writeError(w, http.StatusNotFound, fmt.Errorf("복사할 DOCX 문서가 열려 있지 않습니다"))
		return
	}
	_, content, _, err := source.server.savedFile(r.Context(), input.savedFileInput)
	if err != nil {
		writeError(w, http.StatusConflict, err)
		return
	}
	opened, err := s.openDOCXCopy(r.Context(), name, "copy", content)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("작업 복사본을 만들지 못했습니다"))
		return
	}
	writeJSON(w, http.StatusCreated, opened)
}

// openDOCXCopy runs under openMu. Only validated names may reach the filesystem.
func (s *Server) openDOCXCopy(ctx context.Context, name, origin string, content []byte) (*openedDocument, error) {
	dir, err := os.MkdirTemp("", "document-workspace-copy-*")
	if err != nil {
		return nil, err
	}
	retained := false
	defer func() {
		if !retained {
			_ = os.RemoveAll(dir)
		}
	}()
	path := filepath.Join(dir, name)
	if err = os.WriteFile(path, content, 0600); err != nil {
		return nil, err
	}
	document, err := docx.Open(ctx, path)
	if err != nil {
		return nil, err
	}
	opened, err := s.registerDocument(document.SourcePath(), nil, document, dir, origin)
	if err != nil {
		return nil, err
	}
	s.sessionMu.Lock()
	s.docxSession = document
	s.sessionMu.Unlock()
	retained = true
	return opened, nil
}
