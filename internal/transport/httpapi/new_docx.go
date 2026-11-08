package httpapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/qtaghdi/foloom/internal/formats/docx"
	"golang.org/x/text/unicode/norm"
)

var reservedDocumentName = regexp.MustCompile(`(?i)^(CON|PRN|AUX|NUL|COM[1-9]|LPT[1-9])(?:\.|$)`)

func docxName(input string) (string, error) {
	name := norm.NFC.String(strings.TrimSpace(input))
	if name == "" || strings.HasPrefix(name, ".") || strings.ContainsAny(name, `/\:*?"<>|`) || strings.ContainsFunc(name, unicode.IsControl) || !utf8.ValidString(name) || reservedDocumentName.MatchString(name) {
		return "", fmt.Errorf("파일 이름에 사용할 수 없는 문자가 있습니다. 이름만 입력하세요")
	}
	if !strings.EqualFold(filepath.Ext(name), ".docx") {
		switch strings.ToLower(filepath.Ext(name)) {
		case ".xlsx", ".xls", ".pdf", ".pptx", ".doc", ".txt":
			return "", fmt.Errorf("새 문서는 DOCX 형식입니다. 파일 이름의 다른 확장자를 제거하세요")
		}
		name += ".docx"
	}
	if len(name) > 240 || strings.HasSuffix(strings.TrimSuffix(name, filepath.Ext(name)), ".") {
		return "", fmt.Errorf("파일 이름을 줄이고 끝의 마침표를 제거하세요")
	}
	return name, nil
}
func (s *Server) checkDocumentName(name string) error {
	s.sessionMu.RLock()
	defer s.sessionMu.RUnlock()
	for _, opened := range s.documents {
		if strings.EqualFold(norm.NFC.String(opened.Name), name) {
			return fmt.Errorf("같은 이름의 문서가 열려 있습니다. 다른 이름을 지정하세요")
		}
	}
	return nil
}
func (s *Server) newDOCX(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	if s.scoped {
		http.NotFound(w, r)
		return
	}
	var input struct {
		Name string `json:"name"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("문서 이름을 확인하세요"))
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		writeError(w, http.StatusBadRequest, fmt.Errorf("한 개의 문서 생성 요청만 보낼 수 있습니다"))
		return
	}
	name, err := docxName(input.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	s.openMu.Lock()
	defer s.openMu.Unlock()
	if err = s.checkDocumentLimit(); err != nil {
		writeError(w, http.StatusConflict, err)
		return
	}
	if err = s.checkDocumentName(name); err != nil {
		writeError(w, http.StatusConflict, err)
		return
	}
	dir, err := os.MkdirTemp("", "document-workspace-new-*")
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("새 문서의 작업 복사본을 준비하지 못했습니다"))
		return
	}
	retained := false
	defer func() {
		if !retained {
			_ = os.RemoveAll(dir)
		}
	}()
	document, err := docx.Create(r.Context(), filepath.Join(dir, name), []docx.InitialBlock{{Type: "paragraph", Text: ""}})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	opened, err := s.registerDocument(document.SourcePath(), nil, document, dir, "new")
	if err != nil {
		writeError(w, http.StatusConflict, err)
		return
	}
	s.sessionMu.Lock()
	s.docxSession = document
	s.sessionMu.Unlock()
	retained = true
	writeJSON(w, http.StatusCreated, opened)
}
