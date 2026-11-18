package docx

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/png"
	"testing"
)

func TestFigureReferencesFollowRenumberingAndKeepCaptionEditable(t *testing.T) {
	ctx := context.Background()
	s, filename := withDocument(t, bodyXML(
		`<w:p/><w:p><w:r><w:t>Paragraph between figures</w:t></w:r></w:p><w:p/>`+
			`<w:p><w:r><w:t>See figure </w:t></w:r></w:p><w:p><w:r><w:t>Compare figure </w:t></w:r></w:p>`))
	save := func(session *Session, v Snapshot, ops ...Operation) (Snapshot, error) {
		return session.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: ops}, "human")
	}
	v := mustSnapshot(t, s)
	var pixels bytes.Buffer
	if err := png.Encode(&pixels, image.NewRGBA(image.Rect(0, 0, 8, 4))); err != nil {
		t.Fatal(err)
	}
	picture := &ImageEdit{Action: "insert", Base64: base64.StdEncoding.EncodeToString(pixels.Bytes())}
	v, err := save(s, v, Operation{Type: "update_image", TargetID: v.Blocks[0].ID, Image: picture},
		Operation{Type: "update_image", TargetID: v.Blocks[2].ID, Image: picture})
	if err != nil {
		t.Fatal(err)
	}
	uncaptioned := v.Blocks[0].Paragraph.Runs[0]
	firstPackage := fileBytes(t, filename)
	ref := func(destination, fig string) Operation {
		return Operation{Type: "update_reference", TargetID: destination, Reference: &ReferenceEdit{Action: "insert_figure_ref", FigureID: fig}}
	}
	if _, err := save(s, v, ref(v.Blocks[3].ID, uncaptioned.ID)); err == nil || !bytes.Equal(firstPackage, fileBytes(t, filename)) {
		t.Fatal("unidentified caption produced a reference")
	}
	v, err = save(s, v, Operation{Type: "insert_picture_caption", TargetID: v.Blocks[2].Paragraph.Runs[0].ID, Text: "Selected illustration"})
	if err != nil {
		t.Fatal(err)
	}
	selected := v.Blocks[2].Paragraph.Runs[0]
	if !selected.Picture.CaptionReference {
		t.Fatal("supported numbered caption was not offered as a reference")
	}
	baseline := fileBytes(t, filename)
	firstRef := ref(v.Blocks[4].ID, selected.ID)
	if _, err := save(s, v, firstRef, firstRef); err == nil || !bytes.Equal(baseline, fileBytes(t, filename)) {
		t.Fatal("compound figure reference modified the file")
	}
	v, err = save(s, v, firstRef)
	if err != nil {
		t.Fatal(err)
	}
	referenced := fileBytes(t, filename)
	if _, err := save(s, v, ref(v.Blocks[5].ID, selected.ID)); err == nil || !bytes.Equal(referenced, fileBytes(t, filename)) {
		t.Fatal("reference accepted a stale picture ID")
	}
	v, err = save(s, v, ref(v.Blocks[5].ID, v.Blocks[2].Paragraph.Runs[0].ID))
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Bookmarks) != 1 || !v.Bookmarks[0].Figure {
		t.Fatal("multiple references did not share one generated bookmark")
	}
	name := v.Bookmarks[0].Name
	v, err = save(s, v, Operation{Type: "edit_picture_caption", TargetID: v.Blocks[2].Paragraph.Runs[0].ID,
		ExpectedText: "Selected illustration", Text: "Updated illustration"})
	if err != nil {
		t.Fatal(err)
	}
	if !v.Blocks[2].Paragraph.Runs[0].Picture.MoveCaptionUp {
		t.Fatal("adding a reference disabled supported caption movement")
	}
	v, err = save(s, v, Operation{Type: "move_picture_caption_up", TargetID: v.Blocks[2].Paragraph.Runs[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	beforeRenumbering := fileBytes(t, filename)
	v, err = save(s, v, Operation{Type: "insert_picture_caption", TargetID: v.Blocks[0].Paragraph.Runs[0].ID, Text: "Earlier illustration"})
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	v = mustSnapshot(t, reopened)
	for _, block := range v.Blocks[len(v.Blocks)-2:] {
		run := block.Paragraph.Runs[1]
		if run.Link == nil || run.Link.Bookmark != name || run.Link.RefreshNeeded || run.Text != "2" {
			t.Fatal("reference did not display its current saved figure number", run)
		}
	}
	if v.Bookmarks[0].ParagraphID != v.Blocks[3].Paragraph.ID ||
		v.Blocks[3].Paragraph.Runs[2].Text != ". Updated illustration" ||
		v.Blocks[3].Paragraph.Runs[1].Text != "2" {
		t.Fatal("moving and renumbering did not preserve the caption target")
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(fileBytes(t, filename), beforeRenumbering) {
		t.Fatal("undo did not restore the exact pre-renumbering package")
	}
}
