package docx

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Session owns a single DOCX. mu protects content, revisions and retained events.
// Disk replacement completes before memory and event state advance.
type Session struct {
	mu                 sync.Mutex
	path, id           string
	content            []byte
	pkg                *documentPackage
	revision, sequence uint64
	events             []Event
	history            *historyStore
	// Injected only by package tests to verify persistence failures before commit.
	rename func(string, string) error
}

// Open validates a bounded local DOCX and starts a new session-scoped revision.
func Open(ctx context.Context, filename string) (*Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !filepath.IsAbs(filename) || !strings.EqualFold(filepath.Ext(filename), ".docx") {
		return nil, fmt.Errorf("DOCX path must be absolute and end in .docx")
	}
	resolved, err := filepath.EvalSymlinks(filename)
	if err != nil {
		return nil, fmt.Errorf("resolve DOCX path: %w", err)
	}
	content, err := readFile(resolved)
	if err != nil {
		return nil, err
	}
	pkg, err := loadPackage(content)
	if err != nil {
		return nil, err
	}
	s, err := newSession(resolved, content, pkg)
	if err != nil {
		return nil, err
	}
	unlock, err := lockFile(resolved)
	if err != nil {
		return nil, err
	}
	defer unlock()
	if err = s.checkSource(); err != nil {
		return nil, err
	}
	s.history, err = loadHistory(resolved, content)
	if err != nil {
		return nil, err
	}
	s.revision = s.history.manifest.Revision
	return s, nil
}

func newSession(filename string, content []byte, pkg *documentPackage) (*Session, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return nil, err
	}
	s := &Session{path: filename, id: hex.EncodeToString(id[:]), content: content, pkg: pkg, revision: 1, rename: os.Rename}
	s.history = &historyStore{dir: historyPath(filename), manifest: historyManifest{Version: 1, Revision: 1, Current: historyHash(content)}}
	if _, err := s.snapshotLocked(); err != nil {
		return nil, err
	}
	return s, nil
}

func readFile(filename string) ([]byte, error) {
	info, err := os.Lstat(filename)
	if err != nil {
		return nil, fmt.Errorf("inspect DOCX file: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() > maxFileBytes {
		return nil, fmt.Errorf("DOCX must be a regular file no larger than 32 MiB")
	}
	// Bound the read even if a non-cooperating writer grows the file after Stat.
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return readBounded(file)
}

// Snapshot returns revision-scoped targets. IDs must be refreshed after each commit.
func (s *Session) Snapshot(ctx context.Context) (Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snapshotLocked()
}
func (s *Session) snapshotLocked() (Snapshot, error) {
	root, err := parseXML(s.pkg.document)
	if err != nil {
		return Snapshot{}, err
	}
	blocks, _, unsupported, err := inspectDocument(root, s.id, s.revision, s.pkg.styles)
	if err != nil {
		return Snapshot{}, err
	}
	warnings := []string{"본문과 단순 표의 지원 텍스트와 일반 문단 구조를 편집합니다. 구역·명시적 페이지 경계·참조는 구조 편집을 보호합니다. 용지와 일부 스타일을 미리 보며 자동 페이지 매김, 이미지, 머리글과 필드는 재현하지 않습니다. 지원 범위 밖 OOXML은 보존합니다.", "Microsoft Word의 native 열기와 시각 호환성은 아직 검증하지 않았습니다. 복사본으로 작업하세요."}
	if len(s.pkg.styles.byID) > 500 {
		warnings = append(warnings, "스타일 목록은 최대 500개입니다. 나머지 원본 정의는 보존하며 이 UI 목록에서는 생략합니다.")
	}
	if unsupported {
		warnings = append(warnings, "지원하지 않는 구조는 읽기 전용으로 표시하거나 화면에서 생략합니다.")
	}
	if s.history.warning != "" {
		warnings = append(warnings, s.history.warning)
	}
	history := HistoryState{Undo: len(s.history.manifest.Undo), Redo: len(s.history.manifest.Redo)}
	if history.Undo > 0 {
		history.UndoActor = s.history.manifest.Undo[history.Undo-1].Actor
	}
	if history.Redo > 0 {
		history.RedoActor = s.history.manifest.Redo[history.Redo-1].Actor
	}
	listRoot, _ := parseXML(s.pkg.partData(s.pkg.numberingPath, nil))
	numbering := readNumbering(listRoot)
	applyListLabels(blocks, numbering)
	return Snapshot{Lists: listCatalog(numbering), Styles: s.pkg.styleCatalog(), History: history, ID: s.id, Name: filepath.Base(s.path), Revision: s.revision, Sequence: s.sequence, Blocks: blocks, Page: pageLayout(root.child("body")), Warnings: warnings}, nil
}

type replacement struct {
	start, end int
	data       []byte
}

// Apply validates the entire batch before persisting a candidate. Rejected batches
// never change the current document, revision or event sequence.
func (s *Session) Apply(ctx context.Context, input ApplyInput, actor string) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	if input.SessionID != s.id || input.BaseRevision != s.revision {
		return Snapshot{}, ErrRevisionConflict
	}
	if len(input.Operations) == 0 || len(input.Operations) > maxOperations {
		return Snapshot{}, fmt.Errorf("DOCX batch requires 1 to 1000 operations")
	}
	parts, styles, err := s.pkg.prepareStyles(input.Operations)
	if err != nil {
		return Snapshot{}, err
	}
	listKeys, numbering, err := s.pkg.prepareNumbering(input.Operations, parts)
	if err != nil {
		return Snapshot{}, err
	}
	_, targets, _, err := inspect(s.pkg.document, s.id, s.revision)
	if err != nil {
		return Snapshot{}, err
	}
	seen := map[string]bool{}
	var edits []replacement
	textBytes := 0
	structuralBytes := 0
	for _, op := range input.Operations {
		if op.Type == "define_style" || op.Type == "define_list" {
			continue
		}
		for _, p := range op.Paragraphs {
			if p.Format != nil {
				if err := validateStyleReference(styles, p.Format.StyleID, "paragraph"); err != nil {
					return Snapshot{}, err
				}
			}
			for _, r := range p.Runs {
				if r.Style != nil {
					if err := validateStyleReference(styles, r.Style.StyleID, "character"); err != nil {
						return Snapshot{}, err
					}
				}
			}
		}
		t, ok := targets[op.TargetID]
		if op.Type == "replace_paragraphs" {
			last, exists := targets[op.EndTargetID]
			if !ok || !exists || t.kind != "replace_paragraphs" || last.kind != "replace_paragraphs" || seen[op.TargetID] {
				return Snapshot{}, fmt.Errorf("unsupported, duplicate or stale DOCX paragraph target")
			}
			for _, edit := range edits {
				if t.node.start < edit.end && edit.start < last.node.end || edit.start == edit.end && edit.start >= t.node.start && edit.start < last.node.end {
					return Snapshot{}, fmt.Errorf("overlapping DOCX operations must be combined into one paragraph plan")
				}
			}
			seen[op.TargetID] = true
			op, err := resolveParagraphLists(op, listKeys, numbering)
			if err != nil {
				return Snapshot{}, err
			}
			edit, err := paragraphReplacement(s.pkg.document, targets, op)
			if err != nil {
				return Snapshot{}, err
			}
			for _, paragraph := range op.Paragraphs {
				for _, run := range paragraph.Runs {
					textBytes += len(run.Text)
				}
			}
			if textBytes > 4<<20 {
				return Snapshot{}, fmt.Errorf("DOCX batch text exceeds 4 MiB")
			}
			structuralBytes += len(edit.data)
			if structuralBytes > maxPartBytes {
				return Snapshot{}, fmt.Errorf("DOCX batch replacement XML exceeds 16 MiB limit")
			}
			edits = append(edits, edit)
			continue
		}
		if op.EndTargetID != "" || op.Paragraphs != nil {
			return Snapshot{}, fmt.Errorf("unexpected paragraph fields on text operation")
		}
		if !ok || t.kind != op.Type || seen[op.TargetID] {
			return Snapshot{}, fmt.Errorf("unsupported, duplicate or stale DOCX target")
		}
		seen[op.TargetID] = true
		if op.ExpectedText != t.text {
			return Snapshot{}, fmt.Errorf("DOCX expected text mismatch")
		}
		textBytes += len(op.Text)
		if textBytes > 4<<20 {
			return Snapshot{}, fmt.Errorf("DOCX batch text exceeds 4 MiB")
		}
		if !validInlineText(op.Text) {
			return Snapshot{}, fmt.Errorf("DOCX text exceeds 64 KiB or contains unsupported control characters or tabs")
		}
		if op.Text == t.text {
			continue
		}
		n := t.node
		for _, edit := range edits {
			if n.start < edit.end && edit.start < n.end {
				return Snapshot{}, fmt.Errorf("overlapping DOCX operations must be combined into one paragraph plan")
			}
		}
		opening := string(s.pkg.document[n.start:n.inner])
		if n.is("p") || n.is("r") {
			// Insert into a verified empty container. A local namespace declaration
			// makes this independent of the imported document's chosen prefix.
			content := inlineXML(op.Text)
			if n.is("r") {
				opening, closing := scopedOpening(n, s.pkg.document, false)
				var props string
				if p := n.child("rPr"); p != nil {
					props = string(s.pkg.document[p.start:p.end])
				}
				edits = append(edits, replacement{n.start, n.end, []byte(opening + props + content + closing)})
				continue
			}
			if n.is("p") {
				content = `<w:r xmlns:w="` + wordNS + `">` + content + `</w:r>`
			}
			if strings.HasSuffix(opening, "/>") {
				name := strings.TrimSuffix(strings.Fields(strings.TrimPrefix(opening, "<"))[0], "/>")
				edits = append(edits, replacement{n.start, n.end, []byte(strings.TrimSuffix(opening, "/>") + ">" + content + "</" + name + ">")})
			} else {
				edits = append(edits, replacement{n.endStart, n.endStart, []byte(content)})
			}
			continue
		}
		if strings.Contains(op.Text, "\n") {
			if !plainTextAttributes(n) {
				return Snapshot{}, fmt.Errorf("DOCX inline break cannot replace opaque text attributes")
			}
			edits = append(edits, replacement{n.start, n.end, []byte(inlineXML(op.Text))})
			continue
		}
		// Self-closing text elements need an explicit close tag after insertion.
		if strings.HasSuffix(opening, "/>") {
			name := strings.Fields(strings.TrimPrefix(opening, "<"))[0]
			name = strings.TrimSuffix(strings.TrimSuffix(name, "/>"), ">")
			space := false
			for _, a := range n.attrs {
				if a.Name.Space == "http://www.w3.org/XML/1998/namespace" && a.Name.Local == "space" {
					space = true
					if a.Value != "preserve" && strings.TrimSpace(op.Text) != op.Text {
						return Snapshot{}, fmt.Errorf("target whitespace mode cannot preserve edge spaces")
					}
				}
			}
			opening = strings.TrimSuffix(opening, "/>")
			if !space {
				opening += ` xml:space="preserve"`
			}
			opening += ">"
			edits = append(edits, replacement{n.start, n.end, []byte(opening + escapeText(op.Text) + "</" + name + ">")})
		} else {
			// Preserve existing whitespace semantics without rewriting surrounding
			// run attributes or namespace declarations.
			space := false
			for _, a := range n.attrs {
				if a.Name.Space == "http://www.w3.org/XML/1998/namespace" && a.Name.Local == "space" {
					space = true
				}
			}
			if space {
				// Existing default whitespace semantics cannot preserve new edge spaces.
				for _, a := range n.attrs {
					if a.Name.Local == "space" && a.Value != "preserve" && strings.TrimSpace(op.Text) != op.Text {
						return Snapshot{}, fmt.Errorf("target whitespace mode cannot preserve edge spaces")
					}
				}
			} else {
				opening = strings.TrimSuffix(opening, ">") + ` xml:space="preserve">`
			}
			edits = append(edits, replacement{n.start, n.endStart, []byte(opening + escapeText(op.Text))})
		}
	}
	if len(edits) == 0 && len(parts) == 0 {
		if err := s.checkSource(); err != nil {
			return Snapshot{}, err
		}
		return s.snapshotLocked()
	}
	sort.Slice(edits, func(i, j int) bool { return edits[i].start < edits[j].start })
	for i := 1; i < len(edits); i++ {
		if edits[i].start < edits[i-1].end || edits[i].start == edits[i-1].start {
			return Snapshot{}, fmt.Errorf("overlapping DOCX operations must be combined into one paragraph plan")
		}
	}
	size := len(s.pkg.document)
	for _, e := range edits {
		size += len(e.data) - (e.end - e.start)
	}
	if size > maxPartBytes {
		return Snapshot{}, fmt.Errorf("DOCX document XML exceeds 16 MiB limit")
	}
	document := make([]byte, 0, size)
	offset := 0
	for _, e := range edits {
		document = append(document, s.pkg.document[offset:e.start]...)
		document = append(document, e.data...)
		offset = e.end
	}
	document = append(document, s.pkg.document[offset:]...)
	content, err := s.pkg.serializeParts(document, parts)
	if err != nil {
		return Snapshot{}, err
	}
	candidate, err := loadPackage(content)
	if err != nil {
		return Snapshot{}, err
	}
	if _, _, _, err := inspect(candidate.document, s.id, s.revision+1); err != nil {
		return Snapshot{}, err
	}
	if actor != "ai" && actor != "human" {
		return Snapshot{}, fmt.Errorf("DOCX actor must be human or ai")
	}
	m := historyManifest{Version: 1, Revision: s.revision + 1, Current: historyHash(content), Undo: append(append([]historyEntry(nil), s.history.manifest.Undo...), historyEntry{Hash: historyHash(s.content), Size: len(s.content), Actor: actor})}
	trimHistory(&m)
	return s.persistHistory(ctx, content, candidate, m, actor)
}
func (s *Session) checkSource() error {
	content, err := readFile(s.path)
	if err != nil || sha256.Sum256(content) != sha256.Sum256(s.content) {
		return ErrExternalChange
	}
	return nil
}

// EventsAfter returns a bounded replay. A gap or future cursor explicitly asks
// the browser to reload, so a disconnected client never silently loses changes.
func (s *Session) EventsAfter(ctx context.Context, after uint64) ([]Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if after > s.sequence || len(s.events) > 0 && after < s.events[0].Sequence-1 {
		return []Event{{Sequence: s.sequence, Revision: s.revision, Type: "docx.reload"}}, nil
	}
	out := []Event{}
	for _, e := range s.events {
		if e.Sequence > after {
			out = append(out, e)
		}
	}
	return out, nil
}

// ID is immutable for the lifetime of a session.
func (s *Session) ID() string { return s.id }

// SourcePath identifies the normalized file owned by this session. It is immutable.
func (s *Session) SourcePath() string { return s.path }
