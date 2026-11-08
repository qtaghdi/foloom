package httpapi

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"

	"github.com/qtaghdi/foloom/internal/formats/docx"
)

func (s *Server) exportDOCXDraft(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var input struct {
		docx.ApplyInput
		Name string `json:"name"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil || decoder.Decode(new(any)) != io.EOF || input.SessionID == "" || input.SessionID != r.URL.Query().Get("document") {
		writeError(w, http.StatusBadRequest, fmt.Errorf("내보낼 문서와 기준 리비전을 확인하세요"))
		return
	}
	name, err := docxName(input.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	target, release, err := s.acquireDOCX()
	if err != nil {
		writeError(w, http.StatusConflict, err)
		return
	}
	defer release()
	content, err := target.RecoveryFile(r.Context(), input.ApplyInput)
	if err != nil {
		writeError(w, http.StatusConflict, err)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	w.Header().Set("Content-Length", strconv.Itoa(len(content)))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Document-ID", input.SessionID)
	w.Header().Set("X-Document-Revision", strconv.FormatUint(input.BaseRevision, 10))
	_, _ = w.Write(content)
}
