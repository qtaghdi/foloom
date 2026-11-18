package docx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func numberedRestartParagraph(level int, text string) string {
	return fmt.Sprintf(`<w:p><w:pPr><w:numPr><w:ilvl w:val="%d"/><w:numId w:val="7"/></w:numPr></w:pPr><w:r><w:rPr><w:i/></w:rPr><w:t>%s</w:t></w:r></w:p>`, level, text)
}

const compoundRestartNumbering = `<w:numbering xmlns:w="` + wordNS + `"><w:abstractNum w:abstractNumId="5">
<w:multiLevelType w:val="multilevel"/>
<w:lvl w:ilvl="0"><w:start w:val="3"/><w:numFmt w:val="decimal"/><w:lvlText w:val="%1."/></w:lvl>
<w:lvl w:ilvl="1"><w:start w:val="1"/><w:numFmt w:val="decimal"/><w:lvlText w:val="%1.%2."/><w:pPr><w:ind w:left="1080" w:hanging="360"/></w:pPr></w:lvl>
<w:lvl w:ilvl="2"><w:start w:val="1"/><w:numFmt w:val="decimal"/><w:lvlText w:val="%1.%2.%3."/></w:lvl>
</w:abstractNum><w:num w:numId="7"><w:abstractNumId w:val="5"/></w:num></w:numbering>`

func withCompoundRestartList(t *testing.T, body, numbering string) (*Session, string) {
	t.Helper()
	_, path := withDocument(t, bodyXML(body))
	rels := []byte(`<Relationships xmlns="` + packageRelNS + `"><Relationship Id="num" Type="` + numberingRelationship + `" Target="numbering.xml"/></Relationships>`)
	rewritePackage(t, path, func(_ string, value []byte) []byte { return value }, map[string][]byte{
		"word/_rels/document.xml.rels": rels, "word/numbering.xml": []byte(numbering),
	})
	session, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	return session, path
}

func TestRestartCompoundTerminalChildrenPreservesNativeParentNumber(t *testing.T) {
	ctx := context.Background()
	var body strings.Builder
	for i, level := range []int{0, 1, 0, 1, 1, 1, 0, 1} {
		body.WriteString(numberedRestartParagraph(level, fmt.Sprintf("Paragraph %d", i)))
	}
	session, path := withCompoundRestartList(t, body.String(), compoundRestartNumbering)
	original := fileBytes(t, path)
	baseline := session.pkg
	snapshot := mustSnapshot(t, session)
	first := snapshot.Blocks[4].Paragraph.CompoundRestart
	last := snapshot.Blocks[5].Paragraph.CompoundRestart
	if first == nil || first.ParentNumber != 4 || first.Terminal || last == nil || last.ParentNumber != 4 || !last.Terminal {
		t.Fatal("terminal group or actual parent number was not projected")
	}
	start := 7
	apply := func(first, last int) error {
		_, err := session.Apply(ctx, ApplyInput{SessionID: snapshot.ID, BaseRevision: snapshot.Revision, Operations: []Operation{
			{Type: "restart_compound_sublevel", TargetID: snapshot.Blocks[first].ID, EndTargetID: snapshot.Blocks[last].ID, ListStart: &start},
		}}, "human")
		return err
	}
	for _, bad := range [][2]int{{4, 4}, {1, 5}, {2, 5}, {5, 4}, {4, 7}} {
		if err := apply(bad[0], bad[1]); err == nil || !bytes.Equal(original, fileBytes(t, path)) ||
			mustSnapshot(t, session).Revision != snapshot.Revision {
			t.Fatalf("unsafe compound restart changed the source: %v", bad)
		}
	}
	if err := apply(4, 5); err != nil {
		t.Fatal(err)
	}
	if err := apply(4, 5); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("stale compound restart reported %v", err)
	}
	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	after := mustSnapshot(t, reopened)
	for i, want := range []string{"3.", "3.1.", "4.", "4.1.", "4.7.", "4.8.", "5.", "5.1."} {
		got := after.Blocks[i].Paragraph
		if got.ListLabel != want || (got.List.ID != 7) != (i == 4 || i == 5) {
			t.Fatalf("paragraph %d: %q list %d, want %q", i, got.ListLabel, got.List.ID, want)
		}
	}
	raw := string(reopened.pkg.partData("word/numbering.xml", nil))
	root, err := parseXML([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	clone := root.children[1]
	if clone.child("lvl").child("start").attr("val") != "4" ||
		!strings.Contains(raw, `<w:start w:val="3"/>`) || strings.Count(raw, `w:lvlText w:val="%1.%2."`) != 2 ||
		strings.Count(raw, `w:left="1080" w:hanging="360"`) != 2 {
		t.Fatal("copy lost native tokens, source start or indentation")
	}
	for _, part := range baseline.parts {
		if part.header.Name != "word/numbering.xml" && part.header.Name != "word/document.xml" &&
			!bytes.Equal(part.data, reopened.pkg.partData(part.header.Name, nil)) {
			t.Fatalf("unrelated OOXML part changed: %s", part.header.Name)
		}
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(original, fileBytes(t, path)) {
		t.Fatal("undo did not restore exact DOCX bytes")
	}
}

func TestCompoundRestartProtectsLaterChildrenAndUnsupportedDefinitions(t *testing.T) {
	for _, tc := range []struct{ name, after, numbering string }{
		{"subdescendant", numberedRestartParagraph(2, "Descendant"), compoundRestartNumbering},
		{"continued after text", "<w:p/>" + numberedRestartParagraph(1, "Later child"), compoundRestartNumbering},
		{"ancestor override", "", strings.Replace(compoundRestartNumbering, "</w:num>", `<w:lvlOverride w:ilvl="0"><w:startOverride w:val="5"/></w:lvlOverride></w:num>`, 1)},
		{"zero parent start", "", strings.Replace(compoundRestartNumbering, `<w:start w:val="3"/>`, `<w:start w:val="0"/>`, 1)},
		{"missing explicit parent start", "", strings.Replace(compoundRestartNumbering, `<w:start w:val="3"/>`, "", 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			session, _ := withCompoundRestartList(t, numberedRestartParagraph(0, "Parent")+numberedRestartParagraph(1, "Child")+tc.after, tc.numbering)
			v := mustSnapshot(t, session)
			if v.Blocks[1].Paragraph.CompoundRestart != nil {
				t.Fatal("unsafe compound restart advertised to callers")
			}
		})
	}
}
