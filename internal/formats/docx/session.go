package docx

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
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
	pictureAssets      map[string]pictureAsset
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

// Picture returns only a validated current-revision media asset addressed by a run ID.
func (s *Session) Picture(ctx context.Context, id string) ([]byte, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !strings.HasPrefix(id, s.id+":"+strconv.FormatUint(s.revision, 10)+":") {
		return nil, "", fmt.Errorf("stale DOCX picture")
	}
	asset, ok := s.pictureAssets[id]
	if !ok {
		return nil, "", fmt.Errorf("DOCX picture unavailable")
	}
	return bytes.Clone(asset.data), asset.mime, nil
}

func (s *Session) snapshotLocked() (Snapshot, error) {
	root, err := parseXML(s.pkg.document)
	if err != nil {
		return Snapshot{}, err
	}
	blocks, targetsByID, unsupported, err := inspectDocument(root, s.id, s.revision, s.pkg.styles)
	if err != nil {
		return Snapshot{}, err
	}
	s.addPicturePreviews(blocks, targetsByID)
	for _, kind := range []string{"footnote", "endnote"} {
		if root.has(kind + "Reference") {
			s.pkg.projectNotes(kind, root.child("body"), blocks, targetsByID)
		}
	}
	bookmarks := s.pkg.projectReferences(root, blocks, targetsByID)
	warnings := []string{"본문과 단순 표의 지원 텍스트와 일반 문단 구조를 편집합니다. 구역·명시적 페이지 경계·참조는 구조 편집을 보호합니다. 용지, 일부 스타일과 머리글·바닥글을 미리 보며 자동 페이지 매김, 떠 있는 그림과 필드는 재현하지 않습니다. 지원 범위 밖 OOXML은 보존합니다.", "Microsoft Word의 native 열기와 시각 호환성은 아직 검증하지 않았습니다. 복사본으로 작업하세요."}
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
	sections := []SectionInfo{}
	targets := inspectSections(root, s.id, s.revision)
	headers := s.pkg.headerFooterCatalog(targets)
	for index, target := range targets {
		target.info.HeaderFooters = headers[index]
		if n := target.props.child("titlePg"); n != nil {
			target.info.FirstPage = onOff(n.attr("val"), true)
		}
		sections = append(sections, target.info)
		for index := target.info.StartBlock; index <= target.info.EndBlock && index < len(blocks); index++ {
			block := &blocks[index]
			if block.Paragraph != nil {
				block.Paragraph.SectionID = target.info.ID
			}
			for row := range block.Rows {
				for cell := range block.Rows[row].Cells {
					for paragraph := range block.Rows[row].Cells[cell].Paragraphs {
						block.Rows[row].Cells[cell].Paragraphs[paragraph].SectionID = target.info.ID
					}
				}
			}
		}
	}
	listRoot, _ := parseXML(s.pkg.partData(s.pkg.numberingPath, nil))
	numbering := readNumbering(listRoot)
	applyListLabels(blocks, numbering)
	return Snapshot{Lists: listCatalog(numbering), Comments: s.pkg.projectComments(root, blocks), TrackedChanges: root.has("ins") || root.has("del") || root.has("moveFrom") || root.has("moveTo") || root.has("rPrChange") || root.has("pPrChange"), Bookmarks: bookmarks, EvenAndOddHeaders: s.pkg.headerParity(), Sections: sections, Styles: s.pkg.styleCatalog(), History: history, ID: s.id, Name: filepath.Base(s.path), Revision: s.revision, Sequence: s.sequence, Blocks: blocks, Page: pageLayout(root.child("body")), Warnings: warnings}, nil
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
	if actor != "ai" && actor != "human" {
		return Snapshot{}, fmt.Errorf("DOCX actor must be human or ai")
	}
	content, err := s.prepareOperations(input, actor)
	if err != nil {
		return Snapshot{}, err
	}
	if content == nil {
		if err := s.checkSource(); err != nil {
			return Snapshot{}, err
		}
		return s.snapshotLocked()
	}
	candidate, err := loadPackage(content)
	if err != nil {
		return Snapshot{}, err
	}
	if _, _, _, err := inspect(candidate.document, s.id, s.revision+1); err != nil {
		return Snapshot{}, err
	}
	m := historyManifest{Version: 1, Revision: s.revision + 1, Current: historyHash(content), Undo: append(append([]historyEntry(nil), s.history.manifest.Undo...), historyEntry{Hash: historyHash(s.content), Size: len(s.content), Actor: actor})}
	trimHistory(&m)
	textOnly := true
	for _, op := range input.Operations {
		if op.Type != "replace_run_text" && op.Type != "replace_cell_text" {
			textOnly = false
			break
		}
	}
	return s.persistHistory(ctx, content, candidate, m, actor, textOnly)
}

// RecoveryFile validates a draft against the exact in-memory base. It never
// touches the source path, even if another application has replaced the file.
// Draft changes become committed bytes only in an explicitly reopened copy.
func (s *Session) RecoveryFile(ctx context.Context, input ApplyInput) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if input.SessionID != s.id || input.BaseRevision != s.revision {
		return nil, ErrRevisionConflict
	}
	if len(input.Operations) == 0 {
		return append([]byte(nil), s.content...), nil
	}
	content, err := s.prepareOperations(input, "human")
	if err != nil {
		return nil, err
	}
	if content == nil {
		return append([]byte(nil), s.content...), nil
	}
	candidate, err := loadPackage(content)
	if err != nil {
		return nil, err
	}
	if _, _, _, err := inspect(candidate.document, s.id, s.revision+1); err != nil {
		return nil, err
	}
	return content, nil
}

// prepareOperations runs under mu and does not read or write the source.
func (s *Session) prepareOperations(input ApplyInput, actor string) ([]byte, error) {
	if input.SessionID != s.id || input.BaseRevision != s.revision {
		return nil, ErrRevisionConflict
	}
	if len(input.Operations) == 0 || len(input.Operations) > maxOperations {
		return nil, fmt.Errorf("DOCX batch requires 1 to 1000 operations")
	}
	parts, styles, err := s.pkg.prepareStyles(input.Operations)
	if err != nil {
		return nil, err
	}
	root, err := parseXML(s.pkg.document)
	if err != nil {
		return nil, err
	}
	listKeys, numbering, err := s.pkg.prepareNumbering(input.Operations, parts)
	if err != nil {
		return nil, err
	}
	_, targets, _, err := inspect(s.pkg.document, s.id, s.revision)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var edits []replacement
	headerCount := 0
	var drawingID uint64
	nextDrawingID := func() (uint32, error) {
		if drawingID == 0 {
			next, err := s.pkg.availableDrawingID()
			if err != nil {
				return 0, err
			}
			drawingID = uint64(next)
		}
		if drawingID > 1<<32-1 {
			return 0, fmt.Errorf("DOCX drawing IDs exhausted")
		}
		id := uint32(drawingID)
		drawingID++
		return id, nil
	}
	imageCount := 0
	textBytes := 0
	structuralBytes := 0
	var bookmarks *bookmarkIndex
	var moveTargets map[string]target
	referenceCount := 0
	var revisionIDs map[int64]bool
	var nextRevisionID int64
	var revisionDate string
	for _, op := range input.Operations {
		if op.RectangleColors != nil && op.Type != "update_rectangle_colors" {
			return nil, fmt.Errorf("unexpected DOCX rectangle colors")
		}
		if op.Type == "update_rectangle_colors" || op.Type == "delete_rectangle" {
			t := targets[op.TargetID]
			if t.kind != "inline_rectangle" || seen[op.TargetID] ||
				op.Type == "update_rectangle_colors" && op.RectangleColors == nil ||
				op.Image != nil || op.Reference != nil || op.Math != nil || op.Comment != nil || op.ListDefinition != nil ||
				op.Table != nil || op.Boundary != "" || op.Section != nil || op.SectionID != "" || op.EvenAndOddHeaders != nil ||
				op.StyleDefinition != nil || op.EndTargetID != "" || op.Paragraphs != nil || op.Text != "" || op.ExpectedText != "" {
				return nil, fmt.Errorf("invalid DOCX rectangle operation")
			}
			seen[op.TargetID] = true
			if op.Type == "delete_rectangle" {
				edits = append(edits, replacement{start: t.run.start, end: t.run.end})
				continue
			}
			updated, err := editInlineRectangleColors(s.pkg.document, t, *op.RectangleColors)
			if err != nil {
				return nil, err
			}
			edits = append(edits, updated...)
			continue
		}
		if op.Type == "move_bookmark_up" || op.Type == "move_bookmark_down" {
			if op.Image != nil || op.Reference != nil || op.Math != nil || op.Comment != nil || op.ListDefinition != nil ||
				op.Table != nil || op.Boundary != "" || op.Section != nil || op.SectionID != "" || op.EvenAndOddHeaders != nil ||
				op.StyleDefinition != nil || op.EndTargetID != "" || op.Paragraphs != nil || op.Text != "" || op.ExpectedText != "" {
				return nil, fmt.Errorf("invalid DOCX bookmark move fields")
			}
			if moveTargets == nil {
				_, moveTargets, _, err = inspectDocument(root, s.id, s.revision, s.pkg.styles)
				if err != nil {
					return nil, err
				}
			}
			edit, err := moveBookmarkedParagraph(s.pkg.document, moveTargets[op.TargetID], op.Type)
			if err != nil {
				return nil, err
			}
			structuralBytes += len(edit.data)
			if structuralBytes > maxPartBytes {
				return nil, fmt.Errorf("DOCX batch replacement XML exceeds 16 MiB limit")
			}
			edits = append(edits, edit)
			continue
		}
		if op.Type == "track_run_text" {
			if op.Image != nil || op.Reference != nil || op.Math != nil || op.Comment != nil || op.ListDefinition != nil ||
				op.Table != nil || op.Boundary != "" || op.Section != nil || op.SectionID != "" || op.EvenAndOddHeaders != nil ||
				op.StyleDefinition != nil || op.EndTargetID != "" || op.Paragraphs != nil || referenceCount >= 32 {
				return nil, fmt.Errorf("invalid DOCX tracked text fields")
			}
			if revisionIDs == nil {
				revisionIDs = s.pkg.revisionIDs(root)
				revisionDate = time.Now().UTC().Format(time.RFC3339)
			}
			author := "사용자"
			if actor == "ai" {
				author = "AI"
			}
			edit, err := trackedTextEdit(s.pkg.document, targets[op.TargetID], op, author, revisionDate, revisionIDs, &nextRevisionID)
			if err != nil {
				return nil, err
			}
			edits = append(edits, edit)
			referenceCount++
			continue
		}
		if op.Type == "resolve_comment" || op.Type == "reopen_comment" {
			key := "comment:" + op.TargetID
			if op.Image != nil || op.Reference != nil || op.Math != nil || op.Comment != nil || op.ListDefinition != nil ||
				op.Table != nil || op.Boundary != "" || op.Section != nil || op.SectionID != "" || op.EvenAndOddHeaders != nil ||
				op.StyleDefinition != nil || op.EndTargetID != "" || op.Paragraphs != nil || op.Text != "" || op.ExpectedText != "" ||
				seen[key] || referenceCount >= 32 {
				return nil, fmt.Errorf("invalid DOCX comment state fields")
			}
			if err := s.pkg.setCommentDone(op.TargetID, op.Type == "resolve_comment", parts); err != nil {
				return nil, err
			}
			seen[key] = true
			referenceCount++
			continue
		}
		if op.Type == "accept_revision" || op.Type == "reject_revision" {
			if op.Image != nil || op.Reference != nil || op.Math != nil || op.Comment != nil || op.ListDefinition != nil ||
				op.Table != nil || op.Boundary != "" || op.Section != nil || op.SectionID != "" || op.EvenAndOddHeaders != nil ||
				op.StyleDefinition != nil || op.EndTargetID != "" || op.Paragraphs != nil || op.Text != "" || op.ExpectedText != "" || referenceCount >= 32 {
				return nil, fmt.Errorf("invalid DOCX revision review fields")
			}
			edit, err := revisionReplacement(s.pkg.document, targets[op.TargetID], op.Type)
			if err != nil {
				return nil, err
			}
			edits = append(edits, edit)
			referenceCount++
			continue
		}
		if op.Type == "update_table" {
			key := "table:" + op.TargetID
			t, ok := targets[op.TargetID]
			if !ok || seen[key] || op.Table == nil || op.StyleDefinition != nil || op.EndTargetID != "" || op.Paragraphs != nil || op.Text != "" || op.ExpectedText != "" {
				return nil, fmt.Errorf("invalid or duplicate DOCX table operation")
			}
			seen[key] = true
			edit, err := tableReplacement(s.pkg.document, t, *op.Table)
			if err != nil {
				return nil, err
			}
			structuralBytes += len(edit.data)
			if structuralBytes > maxPartBytes {
				return nil, fmt.Errorf("DOCX batch replacement XML exceeds 16 MiB limit")
			}
			edits = append(edits, edit)
			continue
		}
		if op.Table != nil {
			return nil, fmt.Errorf("unexpected DOCX table fields")
		}
		if op.Type == "update_math" {
			t, ok := targets[op.TargetID]
			if !ok || op.Math == nil || op.Comment != nil || op.Reference != nil || op.Image != nil || op.Boundary != "" || op.Section != nil || op.SectionID != "" || op.EvenAndOddHeaders != nil || op.StyleDefinition != nil || op.EndTargetID != "" || op.Paragraphs != nil || op.Text != "" || op.ExpectedText != "" || referenceCount >= 32 {
				return nil, fmt.Errorf("invalid DOCX equation operation")
			}
			edit, err := mathReplacement(s.pkg.document, t, *op.Math)
			if err != nil {
				return nil, err
			}
			edits = append(edits, edit)
			referenceCount++
			continue
		}
		if op.Math != nil {
			return nil, fmt.Errorf("unexpected DOCX equation fields")
		}
		if op.Type == "add_comment" {
			t, ok := targets[op.TargetID]
			key := "comment:" + op.TargetID
			if !ok || seen[key] || op.Comment == nil || op.Reference != nil || op.Image != nil || op.Boundary != "" || op.Section != nil || op.SectionID != "" || op.EvenAndOddHeaders != nil || op.StyleDefinition != nil || op.EndTargetID != "" || op.Paragraphs != nil || op.Text != "" || op.ExpectedText != "" || referenceCount >= 32 {
				return nil, fmt.Errorf("invalid or duplicate DOCX comment operation")
			}
			seen[key] = true
			changes, err := s.pkg.appendComment(root, t, *op.Comment, actor, targets, parts)
			if err != nil {
				return nil, err
			}
			edits = append(edits, changes...)
			referenceCount++
			continue
		}
		if op.Comment != nil {
			return nil, fmt.Errorf("unexpected DOCX comment fields")
		}
		if op.Type == "update_reference" {
			t, ok := targets[op.TargetID]
			if !ok || op.Reference == nil || op.Image != nil || op.Boundary != "" || op.Section != nil || op.SectionID != "" || op.EvenAndOddHeaders != nil || op.StyleDefinition != nil || op.EndTargetID != "" || op.Paragraphs != nil || op.Text != "" || op.ExpectedText != "" || referenceCount >= 32 {
				return nil, fmt.Errorf("invalid DOCX reference operation")
			}
			if (op.Reference.Action == "rename_bookmark" || op.Reference.Action == "delete_bookmark") && len(input.Operations) != 1 {
				return nil, fmt.Errorf("DOCX bookmark edit must stand alone")
			}
			if bookmarks == nil {
				index := readBookmarks(root)
				bookmarks = &index
			}
			replacements, err := s.pkg.referenceEdits(s.pkg.document, t, *op.Reference, bookmarks, parts)
			if err != nil {
				return nil, err
			}
			edits = append(edits, replacements...)
			referenceCount++
			continue
		}
		if op.Reference != nil {
			return nil, fmt.Errorf("unexpected DOCX reference fields")
		}
		if op.Type == "insert_textbox" || op.Type == "insert_rectangle" {
			t := targets[op.TargetID]
			if t.kind != "replace_paragraphs" || t.paragraph == nil || !t.paragraph.parent.is("body") ||
				op.Image != nil || op.Reference != nil || op.Math != nil || op.Comment != nil || op.ListDefinition != nil ||
				op.Table != nil || op.Boundary != "" || op.Section != nil || op.SectionID != "" || op.EvenAndOddHeaders != nil ||
				op.StyleDefinition != nil || op.EndTargetID != "" || op.Paragraphs != nil || op.ExpectedText != "" {
				return nil, fmt.Errorf("DOCX inline shape requires an editable body paragraph")
			}
			if op.Type == "insert_rectangle" && op.Text != "" {
				return nil, fmt.Errorf("unexpected DOCX rectangle text")
			}
			if op.Type == "insert_textbox" && (len(op.Text) > 1024 || !validText(op.Text) || strings.TrimSpace(op.Text) == "") {
				return nil, fmt.Errorf("DOCX text box requires plain text")
			}
			id, err := nextDrawingID()
			if err != nil {
				return nil, err
			}
			var drawing string
			if op.Type == "insert_rectangle" {
				drawing = inlineRectangle(id)
			} else {
				drawing = inlineTextBox(id, op.Text)
			}
			edits = append(edits, appendDrawingRun(s.pkg.document, t.paragraph, drawing))
			continue
		}
		if op.Type == "update_image" {
			target, ok := targets[op.TargetID]
			if !ok || op.Image == nil || op.ListDefinition != nil || op.Table != nil || op.Math != nil || op.Comment != nil || op.Reference != nil || op.Boundary != "" || op.Section != nil || op.SectionID != "" || op.EvenAndOddHeaders != nil || op.StyleDefinition != nil || op.EndTargetID != "" || op.Paragraphs != nil || op.Text != "" || op.ExpectedText != "" {
				return nil, fmt.Errorf("invalid DOCX image operation")
			}
			if imageCount >= 4 {
				return nil, fmt.Errorf("DOCX batch image limit exceeded")
			}
			var edit replacement
			if op.Image.Action == "set_alt" || op.Image.Action == "delete" || op.Image.Action == "resize" || op.Image.Action == "replace" || op.Image.Action == "crop" || op.Image.Action == "rotate" {
				edit, err = s.pkg.pictureUpdateReplacement(target, *op.Image, parts)
			} else {
				id, allocationErr := nextDrawingID()
				if allocationErr != nil {
					return nil, allocationErr
				}
				edit, err = s.pkg.imageInsertion(s.pkg.document, target, *op.Image, id, parts)
			}
			if err != nil {
				return nil, err
			}
			imageCount++
			edits = append(edits, edit)
			continue
		}
		if op.Image != nil {
			return nil, fmt.Errorf("unexpected DOCX image fields")
		}
		if op.Type == "update_boundary" {
			key := "boundary:" + op.TargetID
			target, ok := targets[op.TargetID]
			supported := target.kind == "replace_paragraphs" ||
				target.kind == "update_boundary" && (op.Boundary == "page_before" || op.Boundary == "no_page_before" || op.Boundary == "no_page_after" || op.Boundary == "remove_section") ||
				target.kind == "update_trailing_boundary" && op.Boundary == "no_page_after" ||
				target.kind == "update_section_boundary" && (op.Boundary == "remove_section" || op.Boundary == "no_page_after" || op.Boundary == "move_section_earlier" || op.Boundary == "move_section_later")
			if seen[key] || !ok || !supported || op.ExpectedText != "" || op.EndTargetID != "" || op.Section != nil || op.SectionID != "" || op.EvenAndOddHeaders != nil || op.Paragraphs != nil || op.Text != "" || op.StyleDefinition != nil {
				return nil, fmt.Errorf("invalid or stale DOCX boundary target")
			}
			seen[key] = true
			replacements, err := boundaryReplacements(target.paragraph.parent.parent, s.pkg.document, s.id, s.revision, target.paragraph, op.Boundary)
			if err != nil {
				return nil, err
			}
			edits = append(edits, replacements...)
			continue
		}
		if op.Boundary != "" {
			return nil, fmt.Errorf("unexpected DOCX boundary field")
		}
		if op.Type == "update_header_parity" {
			if seen[op.Type] || op.EvenAndOddHeaders == nil || op.Section != nil || op.SectionID != "" || op.TargetID != "" || op.EndTargetID != "" || op.Paragraphs != nil || op.Text != "" || op.ExpectedText != "" || op.StyleDefinition != nil {
				return nil, fmt.Errorf("invalid or duplicate DOCX header parity operation")
			}
			seen[op.Type] = true
			if err := s.pkg.updateHeaderParity(*op.EvenAndOddHeaders, parts); err != nil {
				return nil, err
			}
			continue
		}
		if op.EvenAndOddHeaders != nil {
			return nil, fmt.Errorf("unexpected DOCX header parity field")
		}
		if op.Type == "update_section" {
			key := "section:" + op.SectionID
			if seen[key] {
				return nil, fmt.Errorf("duplicate DOCX section update")
			}
			seen[key] = true
			if op.Section == nil {
				return nil, fmt.Errorf("DOCX section update requires settings")
			}
			headerCount += len(op.Section.HeaderFooters)
			if headerCount > 32 {
				return nil, fmt.Errorf("DOCX batch permits 32 header/footer updates")
			}
			edit, err := s.pkg.sectionReplacement(s.pkg.document, root, s.id, s.revision, op, parts)
			if err != nil {
				return nil, err
			}
			edits = append(edits, edit)
			continue
		}
		if op.Section != nil || op.SectionID != "" {
			return nil, fmt.Errorf("unexpected DOCX section fields")
		}
		if op.Type == "define_style" || op.Type == "define_list" {
			continue
		}
		for _, p := range op.Paragraphs {
			if p.Format != nil {
				if err := validateStyleReference(styles, p.Format.StyleID, "paragraph"); err != nil {
					return nil, err
				}
			}
			for _, r := range p.Runs {
				if r.Style != nil {
					if err := validateStyleReference(styles, r.Style.StyleID, "character"); err != nil {
						return nil, err
					}
				}
			}
		}
		t, ok := targets[op.TargetID]
		if op.Type == "replace_paragraphs" {
			last, exists := targets[op.EndTargetID]
			if !ok || !exists || t.kind != "replace_paragraphs" || last.kind != "replace_paragraphs" || seen[op.TargetID] {
				return nil, fmt.Errorf("unsupported, duplicate or stale DOCX paragraph target")
			}
			for _, edit := range edits {
				if t.node.start < edit.end && edit.start < last.node.end || edit.start == edit.end && edit.start >= t.node.start && edit.start < last.node.end {
					return nil, fmt.Errorf("overlapping DOCX operations must be combined into one paragraph plan")
				}
			}
			seen[op.TargetID] = true
			op, err := resolveParagraphLists(op, listKeys, numbering)
			if err != nil {
				return nil, err
			}
			edit, err := paragraphReplacement(s.pkg.document, targets, op)
			if err != nil {
				return nil, err
			}
			for _, paragraph := range op.Paragraphs {
				for _, run := range paragraph.Runs {
					textBytes += len(run.Text)
				}
			}
			if textBytes > 4<<20 {
				return nil, fmt.Errorf("DOCX batch text exceeds 4 MiB")
			}
			structuralBytes += len(edit.data)
			if structuralBytes > maxPartBytes {
				return nil, fmt.Errorf("DOCX batch replacement XML exceeds 16 MiB limit")
			}
			edits = append(edits, edit)
			continue
		}
		if op.EndTargetID != "" || op.Paragraphs != nil {
			return nil, fmt.Errorf("unexpected paragraph fields on text operation")
		}
		if !ok || t.kind != op.Type || seen[op.TargetID] {
			return nil, fmt.Errorf("unsupported, duplicate or stale DOCX target")
		}
		seen[op.TargetID] = true
		editingObjectText := op.Type == "update_control_text" || op.Type == "update_textbox_text"
		if editingObjectText && (len(op.Text) > 1024 || !validText(op.Text) || op.SectionID != "") {
			return nil, fmt.Errorf("DOCX object requires plain text of up to 1024 bytes")
		}
		if op.ExpectedText != t.text {
			return nil, fmt.Errorf("DOCX expected text mismatch")
		}
		textBytes += len(op.Text)
		if textBytes > 4<<20 {
			return nil, fmt.Errorf("DOCX batch text exceeds 4 MiB")
		}
		if !validInlineText(op.Text) {
			return nil, fmt.Errorf("DOCX text exceeds 64 KiB or contains unsupported control characters")
		}
		if op.Text == t.text {
			continue
		}
		n := t.node
		for _, edit := range edits {
			if n.start < edit.end && edit.start < n.end {
				return nil, fmt.Errorf("overlapping DOCX operations must be combined into one paragraph plan")
			}
		}
		if editingObjectText {
			for _, story := range t.textStories {
				changes, err := plainRunTextEdits(s.pkg.document, story, t.text, op.Text)
				if err != nil {
					return nil, err
				}
				edits = append(edits, changes...)
			}
			continue
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
		if strings.ContainsAny(op.Text, "\n\t") {
			if !plainTextAttributes(n) {
				return nil, fmt.Errorf("DOCX inline break cannot replace opaque text attributes")
			}
			edits = append(edits, replacement{n.start, n.end, []byte(inlineXML(op.Text))})
			continue
		}
		edit, err := replaceTextNode(s.pkg.document, n, op.Text)
		if err != nil {
			return nil, err
		}
		edits = append(edits, edit)
	}
	if len(edits) == 0 && len(parts) == 0 {
		return nil, nil
	}
	sort.Slice(edits, func(i, j int) bool { return edits[i].start < edits[j].start })
	for i := 1; i < len(edits); i++ {
		if edits[i].start < edits[i-1].end || edits[i].start == edits[i-1].start {
			return nil, fmt.Errorf("overlapping DOCX operations must be combined into one paragraph plan")
		}
	}
	size := len(s.pkg.document)
	for _, e := range edits {
		size += len(e.data) - (e.end - e.start)
	}
	if size > maxPartBytes {
		return nil, fmt.Errorf("DOCX document XML exceeds 16 MiB limit")
	}
	document := make([]byte, 0, size)
	offset := 0
	for _, e := range edits {
		document = append(document, s.pkg.document[offset:e.start]...)
		document = append(document, e.data...)
		offset = e.end
	}
	document = append(document, s.pkg.document[offset:]...)
	return s.pkg.serializeParts(document, parts)
}

func replaceTextNode(data []byte, n *node, text string) (replacement, error) {
	opening := string(data[n.start:n.inner])
	// Self-closing text elements need an explicit close tag after insertion.
	if strings.HasSuffix(opening, "/>") {
		name := strings.Fields(strings.TrimPrefix(opening, "<"))[0]
		name = strings.TrimSuffix(strings.TrimSuffix(name, "/>"), ">")
		space := false
		for _, a := range n.attrs {
			if a.Name.Space == "http://www.w3.org/XML/1998/namespace" && a.Name.Local == "space" {
				space = true
				if a.Value != "preserve" && strings.TrimSpace(text) != text {
					return replacement{}, fmt.Errorf("target whitespace mode cannot preserve edge spaces")
				}
			}
		}
		opening = strings.TrimSuffix(opening, "/>")
		if !space {
			opening += ` xml:space="preserve"`
		}
		opening += ">"
		return replacement{n.start, n.end, []byte(opening + escapeText(text) + "</" + name + ">")}, nil
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
				if a.Name.Local == "space" && a.Value != "preserve" && strings.TrimSpace(text) != text {
					return replacement{}, fmt.Errorf("target whitespace mode cannot preserve edge spaces")
				}
			}
		} else {
			opening = strings.TrimSuffix(opening, ">") + ` xml:space="preserve">`
		}
		return replacement{n.start, n.endStart, []byte(opening + escapeText(text))}, nil
	}
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
