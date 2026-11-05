package docx

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type compatibilityFixture struct {
	File              string   `json:"file"`
	SHA256            string   `json:"sha256"`
	Producer          string   `json:"producer"`
	Format            string   `json:"format"`
	Date              string   `json:"date"`
	Rights            string   `json:"rights"`
	Features          []string `json:"features"`
	NativeWord        string   `json:"nativeWord"`
	NativeLibreOffice string   `json:"nativeLibreOffice"`
}

func compatibilityFixtures(t *testing.T) []compatibilityFixture {
	t.Helper()
	data := fileBytes(t, "../../../testdata/docx/compatibility/manifest.json")
	var fixtures []compatibilityFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) == 0 {
		t.Fatal("compatibility manifest is empty")
	}
	for _, fixture := range fixtures {
		if filepath.Base(fixture.File) != fixture.File || !strings.HasSuffix(fixture.File, ".docx") {
			t.Fatalf("unsafe fixture name: %q", fixture.File)
		}
	}
	return fixtures
}

// Fixture provenance is a gate, not documentation that may drift independently
// of the bytes exercised by the preservation suite.
func TestCompatibilityManifest(t *testing.T) {
	root := "../../../testdata/docx/compatibility"
	fixtures := compatibilityFixtures(t)
	seen := map[string]bool{}
	for _, fixture := range fixtures {
		if filepath.Base(fixture.File) != fixture.File || !strings.HasSuffix(fixture.File, ".docx") || seen[fixture.File] {
			t.Fatalf("unsafe or duplicate fixture name: %q", fixture.File)
		}
		seen[fixture.File] = true
		t.Run(fixture.File, func(t *testing.T) {
			if fixture.Producer == "" || fixture.Format == "" || fixture.Date == "" || fixture.Rights == "" || len(fixture.Features) == 0 || fixture.NativeWord == "" || fixture.NativeLibreOffice == "" {
				t.Fatal("fixture provenance or native status is missing")
			}
			content := fileBytes(t, filepath.Join(root, fixture.File))
			sum := sha256.Sum256(content)
			if hex.EncodeToString(sum[:]) != fixture.SHA256 {
				t.Fatal("fixture hash differs from provenance manifest")
			}
			if _, err := loadPackage(content); err != nil {
				t.Fatal(err)
			}
		})
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".docx") && !seen[entry.Name()] {
			t.Fatalf("fixture is not declared in manifest: %s", entry.Name())
		}
	}
}

// Reopening and committing distinct edits catches accumulated serialization
// changes that a second no-op save cannot expose.
func TestCompatibilityRepeatedCommits(t *testing.T) {
	for _, fixture := range compatibilityFixtures(t) {
		name := fixture.File
		t.Run(name, func(t *testing.T) {
			source := filepath.Join("../../../testdata/docx/compatibility", name)
			original := fileBytes(t, source)
			path := filepath.Join(t.TempDir(), name)
			if err := os.WriteFile(path, original, 0o600); err != nil {
				t.Fatal(err)
			}
			originalPackage, err := loadPackage(original)
			if err != nil {
				t.Fatal(err)
			}
			for _, value := range []string{"첫 저장 😀", " second & <change> ", "마지막 저장 e\u0301"} {
				session, err := Open(context.Background(), path)
				if err != nil {
					t.Fatal(err)
				}
				snapshot := mustSnapshot(t, session)
				run := snapshot.Blocks[0].Paragraph.Runs[1]
				before := append([]byte(nil), session.pkg.document...)
				_, targets, _, err := inspect(before, snapshot.ID, snapshot.Revision)
				if err != nil {
					t.Fatal(err)
				}
				target := targets[run.ID].node
				after, err := session.Apply(context.Background(), batch(snapshot, "replace_run_text", run, value), "ai")
				if err != nil {
					t.Fatal(err)
				}
				if after.Revision != snapshot.Revision+1 || after.Sequence != 1 {
					t.Fatal("commit did not advance exactly once")
				}
				if after.Blocks[0].Paragraph.Runs[1].Text != value || after.Blocks[0].Paragraph.Runs[2].Text != "repeat" || !after.Blocks[0].Paragraph.Runs[0].Bold || !after.Blocks[0].Paragraph.Runs[2].Italic {
					t.Fatal("text or unrelated formatting changed")
				}
				_, nextTargets, _, err := inspect(session.pkg.document, after.ID, after.Revision)
				if err != nil {
					t.Fatal(err)
				}
				next := nextTargets[after.Blocks[0].Paragraph.Runs[1].ID].node
				if !bytes.Equal(before[:target.start], session.pkg.document[:next.start]) || !bytes.Equal(before[target.end:], session.pkg.document[next.end:]) {
					t.Fatal("XML outside edited text node changed")
				}
				current, err := loadPackage(fileBytes(t, path))
				if err != nil {
					t.Fatal(err)
				}
				if len(current.parts) != len(originalPackage.parts) {
					t.Fatal("package part count changed")
				}
				for i, part := range originalPackage.parts {
					if part.header.Name != "word/document.xml" && (part.header.Name != current.parts[i].header.Name || !bytes.Equal(part.data, current.parts[i].data)) {
						t.Fatalf("unrelated package part changed: %s", part.header.Name)
					}
				}
			}
			if !bytes.Equal(original, fileBytes(t, source)) {
				t.Fatal("source fixture changed")
			}
		})
	}
}
