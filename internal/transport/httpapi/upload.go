package httpapi

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

const maxUploadBytes int64 = 100 << 20

// Browser-selected files have no authorized server path. Keep a private working
// copy and never infer an original path from the client-supplied filename.
func (s *Server) uploadDocument(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	s.openMu.Lock()
	defer s.openMu.Unlock()
	if err := s.checkDocumentLimit(); err != nil {
		writeError(w, http.StatusConflict, err)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes+(64<<10))
	reader, err := r.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid file upload"))
		return
	}
	part, err := reader.NextPart()
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("select an XLSX or DOCX file"))
		return
	}
	name := part.FileName()
	if part.FormName() != "file" || name == "" || len(name) > 240 || !utf8.ValidString(name) || strings.ContainsAny(name, `/\\`) || strings.ContainsFunc(name, unicode.IsControl) {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid upload filename"))
		return
	}
	limit := maxUploadBytes
	switch strings.ToLower(filepath.Ext(name)) {
	case ".xlsx":
	case ".docx":
		limit = 32 << 20
	default:
		writeError(w, http.StatusBadRequest, fmt.Errorf("XLSX 또는 DOCX 파일을 선택하세요"))
		return
	}
	dir, err := os.MkdirTemp("", "document-workspace-upload-*")
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("cannot prepare working copy"))
		return
	}
	retained := false
	defer func() {
		if !retained {
			_ = os.RemoveAll(dir)
		}
	}()
	path := filepath.Join(dir, name)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("cannot prepare working copy"))
		return
	}
	count, copyErr := io.Copy(file, io.LimitReader(part, limit+1))
	closeErr := file.Close()
	if copyErr != nil || closeErr != nil || count == 0 || count > limit {
		writeError(w, http.StatusBadRequest, fmt.Errorf("file upload failed or exceeds size limit"))
		return
	}
	if _, err := reader.NextPart(); err != io.EOF {
		writeError(w, http.StatusBadRequest, fmt.Errorf("upload exactly one file per request"))
		return
	}
	if err := r.Context().Err(); err != nil {
		return
	}
	opened, err := s.openDocumentPath(r.Context(), path, dir)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	retained = true
	writeJSON(w, http.StatusOK, opened)
}
