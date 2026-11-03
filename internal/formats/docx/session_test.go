package docx

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mustSnapshot(t *testing.T, s *Session) Snapshot {
	t.Helper()
	v, e := s.Snapshot(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func createTest(t *testing.T) (*Session, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "한글 document.docx")
	s, e := Create(context.Background(), path, []InitialBlock{{Type: "paragraph", Text: "hello"}, {Type: "table", Rows: [][]string{{"A", "B"}, {"C", "D"}}}})
	if e != nil {
		t.Fatal(e)
	}
	return s, path
}
func batch(snap Snapshot, kind string, run Run, text string) ApplyInput {
	return ApplyInput{SessionID: snap.ID, BaseRevision: snap.Revision, Operations: []Operation{{Type: kind, TargetID: run.ID, ExpectedText: run.Text, Text: text}}}
}
func fileBytes(t *testing.T, path string) []byte {
	t.Helper()
	data, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	return data
}
func rewritePackage(t *testing.T, path string, change func(string, []byte) []byte, extra map[string][]byte) {
	t.Helper()
	data := fileBytes(t, path)
	reader, e := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for _, f := range reader.File {
		r, e := f.Open()
		if e != nil {
			t.Fatal(e)
		}
		d, e := io.ReadAll(r)
		r.Close()
		if e != nil {
			t.Fatal(e)
		}
		out, e := w.CreateHeader(&f.FileHeader)
		if e != nil {
			t.Fatal(e)
		}
		out.Write(change(f.Name, d))
	}
	for n, d := range extra {
		out, e := w.Create(n)
		if e != nil {
			t.Fatal(e)
		}
		out.Write(d)
	}
	if e := w.Close(); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(path, b.Bytes(), 0o600); e != nil {
		t.Fatal(e)
	}
}
func withDocument(t *testing.T, xml string) (*Session, string) {
	t.Helper()
	_, path := createTest(t)
	rewritePackage(t, path, func(n string, b []byte) []byte {
		if n == "word/document.xml" {
			return []byte(xml)
		}
		return b
	}, nil)
	s, e := Open(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	return s, path
}
func bodyXML(body string) string {
	return `<w:document xmlns:w="` + wordNS + `"><w:body>` + body + `</w:body></w:document>`
}

func TestCreateEditReopenAndPreserve(t *testing.T) {
	s, path := createTest(t)
	ctx := context.Background()
	snap := mustSnapshot(t, s)
	before, e := loadPackage(fileBytes(t, path))
	if e != nil {
		t.Fatal(e)
	}
	input := batch(snap, "replace_run_text", snap.Blocks[0].Paragraph.Runs[0], " 한글 <새 문서> & ")
	cell := snap.Blocks[1].Rows[0].Cells[1].Paragraphs[0].Runs[0]
	input.Operations = append(input.Operations, Operation{Type: "replace_cell_text", TargetID: cell.ID, ExpectedText: cell.Text, Text: "표 변경"})
	after, e := s.Apply(ctx, input, "ai")
	if e != nil {
		t.Fatal(e)
	}
	if after.Revision != 2 || after.Sequence != 1 || after.Blocks[0].Paragraph.Runs[0].Text != " 한글 <새 문서> & " {
		t.Fatalf("bad commit: %+v", after)
	}
	if len(s.events) != 1 || s.events[0].Actor != "ai" {
		t.Fatal("missing commit event")
	}
	reopened, e := Open(ctx, path)
	if e != nil {
		t.Fatal(e)
	}
	round := mustSnapshot(t, reopened)
	if round.Blocks[1].Rows[0].Cells[1].Paragraphs[0].Runs[0].Text != "표 변경" {
		t.Fatal("table not saved")
	}
	current, e := loadPackage(fileBytes(t, path))
	if e != nil {
		t.Fatal(e)
	}
	for i, p := range before.parts {
		if p.header.Name != "word/document.xml" && !bytes.Equal(p.data, current.parts[i].data) {
			t.Fatal("unrelated package part changed")
		}
	}
	input = batch(after, "replace_run_text", after.Blocks[0].Paragraph.Runs[0], "second")
	if _, e = s.Apply(ctx, input, "human"); e != nil {
		t.Fatal(e)
	}
	if _, e = Create(ctx, path, []InitialBlock{{Type: "paragraph", Text: "overwrite"}}); e == nil {
		t.Fatal("existing destination overwritten")
	}
}

func TestRejectedBatchesRemainAtomic(t *testing.T) {
	for _, name := range []string{"stale", "session", "expected", "missing", "duplicate", "invalid", "kind", "mixed", "empty"} {
		t.Run(name, func(t *testing.T) {
			s, path := createTest(t)
			snap := mustSnapshot(t, s)
			before := fileBytes(t, path)
			input := batch(snap, "replace_run_text", snap.Blocks[0].Paragraph.Runs[0], "new")
			switch name {
			case "stale":
				input.BaseRevision = 0
			case "session":
				input.SessionID = "old"
			case "expected":
				input.Operations[0].ExpectedText = "wrong"
			case "missing":
				input.Operations[0].TargetID = "missing"
			case "duplicate":
				input.Operations = append(input.Operations, input.Operations[0])
			case "invalid":
				input.Operations[0].Text = "bad\x00"
			case "kind":
				input.Operations[0].Type = "replace_cell_text"
			case "mixed":
				input.Operations = append(input.Operations, Operation{TargetID: "bad"})
			case "empty":
				input.Operations = nil
			}
			if _, e := s.Apply(context.Background(), input, "ai"); e == nil {
				t.Fatal("accepted invalid batch")
			}
			after := mustSnapshot(t, s)
			if after.Revision != snap.Revision || after.Sequence != 0 || !bytes.Equal(before, fileBytes(t, path)) {
				t.Fatal("rejected batch mutated data")
			}
		})
	}
}
func TestPersistenceFailuresAndExternalChanges(t *testing.T) {
	for _, name := range []string{"rename", "external", "lock", "deleted", "cancel"} {
		t.Run(name, func(t *testing.T) {
			s, path := createTest(t)
			snap := mustSnapshot(t, s)
			input := batch(snap, "replace_run_text", snap.Blocks[0].Paragraph.Runs[0], "new")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch name {
			case "rename":
				s.rename = func(string, string) error { return fmt.Errorf("injected save failure") }
			case "external":
				os.WriteFile(path, []byte("outside"), 0o600)
			case "lock":
				os.Mkdir(path+".document-workspace.lock", 0o700)
			case "deleted":
				os.Remove(path)
			case "cancel":
				cancel()
			}
			before, _ := os.ReadFile(path)
			if _, e := s.Apply(ctx, input, "ai"); e == nil {
				t.Fatal("expected failure")
			}
			after := mustSnapshot(t, s)
			disk, _ := os.ReadFile(path)
			if after.Revision != 1 || after.Sequence != 0 || !bytes.Equal(before, disk) {
				t.Fatal("failed save mutated authoritative state")
			}
			leftovers, _ := filepath.Glob(filepath.Join(filepath.Dir(path), ".document-workspace-docx-*.tmp"))
			if len(leftovers) > 0 {
				t.Fatal("temporary file leaked")
			}
		})
	}
}
func TestNoopAndStaleTargets(t *testing.T) {
	s, path := createTest(t)
	snap := mustSnapshot(t, s)
	before := fileBytes(t, path)
	input := batch(snap, "replace_run_text", snap.Blocks[0].Paragraph.Runs[0], "hello")
	result, e := s.Apply(context.Background(), input, "ai")
	if e != nil || result.Revision != 1 || !bytes.Equal(before, fileBytes(t, path)) {
		t.Fatal("noop changed document")
	}
	input.Operations[0].Text = "new"
	if _, e = s.Apply(context.Background(), input, "ai"); e != nil {
		t.Fatal(e)
	}
	input.BaseRevision = 2
	if _, e = s.Apply(context.Background(), input, "ai"); e == nil {
		t.Fatal("stale target accepted")
	}
}
func TestXMLPreservationAndEmptyRuns(t *testing.T) {
	for _, tag := range []string{`<x:t/>`, `<x:t xml:space="preserve"/>`, `<x:t></x:t>`, `<x:t>old</x:t>`} {
		t.Run(tag, func(t *testing.T) {
			xml := `<x:document xmlns:x="` + wordNS + `" xmlns:custom="urn:custom"><x:body><x:p custom:keep="yes"><x:pPr><x:jc x:val="center"/></x:pPr><!--keep--><x:r><x:rPr><x:b/></x:rPr>` + tag + `</x:r></x:p><custom:extension attr="raw"/></x:body></x:document>`
			s, path := withDocument(t, xml)
			snap := mustSnapshot(t, s)
			run := snap.Blocks[0].Paragraph.Runs[0]
			if !run.Bold || !run.Editable {
				t.Fatal("ordinary styled text not editable")
			}
			_, e := s.Apply(context.Background(), batch(snap, "replace_run_text", run, " 한글 & <> "), "ai")
			if e != nil {
				t.Fatal(e)
			}
			pkg, e := loadPackage(fileBytes(t, path))
			if e != nil {
				t.Fatal(e)
			}
			str := string(pkg.document)
			for _, preserved := range []string{`custom:keep="yes"`, `<x:pPr><x:jc x:val="center"/></x:pPr><!--keep-->`, `<x:rPr><x:b/></x:rPr>`, `<custom:extension attr="raw"/>`} {
				if !strings.Contains(str, preserved) {
					t.Fatal("unrelated XML changed")
				}
			}
			if mustSnapshot(t, s).Blocks[0].Paragraph.Runs[0].Text != " 한글 & <> " {
				t.Fatal("text failed to roundtrip")
			}
		})
	}
}
func TestUnsupportedStructuresAreReadOnly(t *testing.T) {
	for _, body := range []string{
		`<w:p><w:r><w:fldChar w:fldCharType="begin"/></w:r></w:p><w:p><w:r><w:t>field result</w:t></w:r></w:p><w:p><w:r><w:fldChar w:fldCharType="end"/></w:r></w:p>`,
		`<w:p><w:r><w:instrText> PAGE </w:instrText></w:r><w:r><w:t>field</w:t></w:r></w:p>`,
		`<w:p><w:ins><w:r><w:t>tracked</w:t></w:r></w:ins></w:p>`,
		`<w:tbl><w:tr><w:tc><w:tcPr><w:gridSpan w:val="2"/></w:tcPr><w:p><w:r><w:t>merged</w:t></w:r></w:p></w:tc></w:tr></w:tbl>`,
		`<w:tbl><w:tr><w:tc><w:p><w:r><w:t>nested</w:t></w:r></w:p><w:tbl/></w:tc></w:tr></w:tbl>`,
	} {
		t.Run(body[:20], func(t *testing.T) {
			s, _ := withDocument(t, bodyXML(body))
			_, targets, _, e := inspect(s.pkg.document, s.id, 1)
			if e != nil {
				t.Fatal(e)
			}
			if len(targets) > 0 {
				t.Fatalf("unsupported targets editable: %+v", targets)
			}
		})
	}
}
func TestTwoWritersAndEvents(t *testing.T) {
	s, path := createTest(t)
	other, e := Open(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	snap := mustSnapshot(t, s)
	if _, e = s.Apply(context.Background(), batch(snap, "replace_run_text", snap.Blocks[0].Paragraph.Runs[0], "winner"), "ai"); e != nil {
		t.Fatal(e)
	}
	stale := mustSnapshot(t, other)
	_, e = other.Apply(context.Background(), batch(stale, "replace_run_text", stale.Blocks[0].Paragraph.Runs[0], "loser"), "ai")
	if !errors.Is(e, ErrExternalChange) {
		t.Fatalf("second writer error: %v", e)
	}
	events, e := s.EventsAfter(context.Background(), 0)
	if e != nil || len(events) != 1 || events[0].Type != "docx.committed" {
		t.Fatal("event replay failed")
	}
	events, _ = s.EventsAfter(context.Background(), 99)
	if events[0].Type != "docx.reload" {
		t.Fatal("future cursor not reset")
	}
}
func TestPackageSafety(t *testing.T) {
	for _, name := range []string{"duplicate", "traversal", "directive", "missing", "relationship", "depth", "signature", "macro"} {
		t.Run(name, func(t *testing.T) {
			_, path := createTest(t)
			extra := map[string][]byte{}
			switch name {
			case "duplicate":
				extra["word/document.xml"] = []byte("duplicate")
			case "traversal":
				extra["../bad"] = []byte("bad")
			case "signature":
				extra["_xmlsignatures/sig1.xml"] = []byte("<x/>")
			case "macro":
				extra["word/vbaProject.bin"] = []byte("x")
			}
			rewritePackage(t, path, func(n string, b []byte) []byte {
				if n == "word/document.xml" {
					switch name {
					case "directive":
						return []byte(`<!DOCTYPE x [<!ENTITY y SYSTEM "file:///etc/passwd">]>` + bodyXML(""))
					case "missing":
						return []byte("<document/>")
					case "depth":
						return []byte(strings.Repeat("<x>", 70) + strings.Repeat("</x>", 70))
					}
				}
				if n == "_rels/.rels" && name == "relationship" {
					return bytes.ReplaceAll(b, []byte("word/document.xml"), []byte("missing.xml"))
				}
				return b
			}, extra)
			if _, e := Open(context.Background(), path); e == nil {
				t.Fatal("unsafe package accepted")
			}
		})
	}
}
func TestCreateFailureNeverOverwrites(t *testing.T) {
	_, path := createTest(t)
	before := sha256.Sum256(fileBytes(t, path))
	for _, blocks := range [][]InitialBlock{{{Type: "table", Rows: [][]string{{"a"}, {"b", "c"}}}}, {{Type: "paragraph", Text: "bad\ntext"}}, {{Type: "unknown"}}} {
		if _, e := Create(context.Background(), path, blocks); e == nil {
			t.Fatal("invalid creation accepted")
		}
		if sha256.Sum256(fileBytes(t, path)) != before {
			t.Fatal("destination changed")
		}
	}
}

func TestCompatibilityCorpus(t *testing.T) {
	for _, name := range []string{"generated-basic.docx", "generated-preservation.docx"} {
		t.Run(name, func(t *testing.T) {
			source := filepath.Join("../../../testdata/docx/compatibility", name)
			path := filepath.Join(t.TempDir(), name)
			original := fileBytes(t, source)
			if e := os.WriteFile(path, original, 0o600); e != nil {
				t.Fatal(e)
			}
			before, e := loadPackage(original)
			if e != nil {
				t.Fatal(e)
			}
			expected := string(before.document)
			expected = strings.Replace(expected, `<w:t>repeat</w:t>`, `<w:t xml:space="preserve">changed longer 한글</w:t>`, 1)
			expected = strings.Replace(expected, `<w:t>B</w:t>`, `<w:t xml:space="preserve">표 변경</w:t>`, 1)
			s, e := Open(context.Background(), path)
			if e != nil {
				t.Fatal(e)
			}
			snap := mustSnapshot(t, s)
			run := snap.Blocks[0].Paragraph.Runs[1]
			input := batch(snap, "replace_run_text", run, "changed longer 한글")
			cell := snap.Blocks[1].Rows[0].Cells[1].Paragraphs[0].Runs[0]
			input.Operations = append(input.Operations, Operation{Type: "replace_cell_text", TargetID: cell.ID, ExpectedText: cell.Text, Text: "표 변경"})
			if _, e = s.Apply(context.Background(), input, "ai"); e != nil {
				t.Fatal(e)
			}
			for round := 0; round < 2; round++ {
				after, e := loadPackage(fileBytes(t, path))
				if e != nil {
					t.Fatal(e)
				}
				if string(after.document) != expected {
					t.Fatal("target-external document XML bytes changed")
				}
				if len(before.parts) != len(after.parts) {
					t.Fatal("part count changed")
				}
				for i, p := range before.parts {
					if p.header.Name != "word/document.xml" && sha256.Sum256(p.data) != sha256.Sum256(after.parts[i].data) {
						t.Fatalf("unrelated part changed: %s", p.header.Name)
					}
				}
				s, e = Open(context.Background(), path)
				if e != nil {
					t.Fatal(e)
				}
				snap = mustSnapshot(t, s)
				if snap.Blocks[0].Paragraph.Runs[2].Text != "repeat" {
					t.Fatal("repeated text resolved to wrong run")
				}
				input = batch(snap, "replace_run_text", snap.Blocks[0].Paragraph.Runs[1], "changed longer 한글")
				if _, e = s.Apply(context.Background(), input, "ai"); e != nil {
					t.Fatal(e)
				}
			}
		})
	}
}

func TestConcurrentBatchHasOneCommit(t *testing.T) {
	s, _ := createTest(t)
	snap := mustSnapshot(t, s)
	input := batch(snap, "replace_run_text", snap.Blocks[0].Paragraph.Runs[0], "winner")
	results := make(chan error, 2)
	for range 2 {
		go func() { _, err := s.Apply(context.Background(), input, "ai"); results <- err }()
	}
	successes, conflicts := 0, 0
	for range 2 {
		err := <-results
		if err == nil {
			successes++
		} else if errors.Is(err, ErrRevisionConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if successes != 1 || conflicts != 1 || mustSnapshot(t, s).Revision != 2 || len(s.events) != 1 {
		t.Fatal("concurrent batch was not atomic")
	}
}

func TestEventReplayGap(t *testing.T) {
	s, _ := createTest(t)
	for i := 0; i < 130; i++ {
		snap := mustSnapshot(t, s)
		_, err := s.Apply(context.Background(), batch(snap, "replace_run_text", snap.Blocks[0].Paragraph.Runs[0], fmt.Sprintf("edit %d", i)), "ai")
		if err != nil {
			t.Fatal(err)
		}
	}
	events, err := s.EventsAfter(context.Background(), 0)
	if err != nil || len(events) != 1 || events[0].Type != "docx.reload" || events[0].Sequence != 130 {
		t.Fatal("replay gap silently lost changes")
	}
}
