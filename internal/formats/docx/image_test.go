package docx

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/png"
	"strings"
	"testing"
)

func TestDOCXInlinePictureKeepsExistingPackagePartsAndUndo(t *testing.T) {
	ctx := context.Background()
	s, path := withDocument(t, bodyXML(`<w:p><w:r><w:t>first</w:t></w:r></w:p><w:p><w:r><w:t>second</w:t></w:r></w:p>`))
	original := fileBytes(t, path)
	var pngBytes bytes.Buffer
	if err := png.Encode(&pngBytes, image.NewRGBA(image.Rect(0, 0, 4, 2))); err != nil {
		t.Fatal(err)
	}
	snapshot := mustSnapshot(t, s)
	picture := ImageEdit{Action: "insert", Base64: base64.StdEncoding.EncodeToString(pngBytes.Bytes()), Alt: `test & "quoted"`, WidthEMU: 400000}
	batch := ApplyInput{SessionID: snapshot.ID, BaseRevision: snapshot.Revision, Operations: []Operation{
		{Type: "update_image", TargetID: snapshot.Blocks[0].ID, Image: &picture},
		{Type: "define_style", StyleDefinition: &StyleDefinition{ID: "TestNew", Name: "test style", Kind: "paragraph"}},
	}}
	bad := ImageEdit{Action: "insert", Base64: "c3Zn"}
	batch.Operations = append(batch.Operations, Operation{Type: "update_image", TargetID: snapshot.Blocks[1].ID, Image: &bad})
	if _, err := s.Apply(ctx, batch, "human"); err == nil || !bytes.Equal(original, fileBytes(t, path)) {
		t.Fatal("invalid image was persisted")
	}
	batch.Operations = batch.Operations[:2]
	saved, err := s.Apply(ctx, batch, "human")
	if err != nil {
		t.Fatal(err)
	}
	pictureRun := saved.Blocks[0].Paragraph.Runs[1]
	if pictureRun.Picture == nil || !pictureRun.Picture.Preview || pictureRun.Picture.Alt != picture.Alt {
		t.Fatal("picture metadata not available after commit")
	}
	payload, mime, err := s.Picture(ctx, pictureRun.ID)
	if err != nil || mime != "image/png" || !bytes.Equal(payload, pngBytes.Bytes()) {
		t.Fatal("picture data not available after commit")
	}
	if _, _, err := s.Picture(ctx, snapshot.Blocks[0].Paragraph.Runs[0].ID); err == nil {
		t.Fatal("old revision read succeeded")
	}
	document := string(s.pkg.document)
	if !strings.Contains(document, `<wp:extent cx="400000" cy="200000"/>`) || !strings.Contains(document, `descr="test &amp; &#34;quoted&#34;"`) {
		t.Fatalf("missing size or escaped alternative text: %s", document)
	}
	media := s.pkg.relatedPart("rIdFoloomImage1", "image", nil)
	if media == "" || !bytes.Equal(s.pkg.partData(media, nil), pngBytes.Bytes()) || s.pkg.stylesPath == "" {
		t.Fatal("new image or same-batch style relationship lost")
	}
	inserted := fileBytes(t, path)
	priorParts := s.pkg.parts
	alt := ImageEdit{Action: "set_alt", Alt: `대체 & "설명"`}
	update := ApplyInput{SessionID: saved.ID, BaseRevision: saved.Revision, Operations: []Operation{
		{Type: "update_image", TargetID: pictureRun.ID, Image: &alt},
	}}
	if _, err := s.Apply(ctx, ApplyInput{SessionID: saved.ID, BaseRevision: saved.Revision, Operations: append(update.Operations, update.Operations[0])}, "human"); err == nil || !bytes.Equal(inserted, fileBytes(t, path)) {
		t.Fatal("duplicate description edits changed the document")
	}
	updated, err := s.Apply(ctx, update, "human")
	if err != nil {
		t.Fatal(err)
	}
	info := updated.Blocks[0].Paragraph.Runs[1].Picture
	if info == nil || info.Alt != alt.Alt || info.WidthEMU != pictureRun.Picture.WidthEMU || info.HeightEMU != pictureRun.Picture.HeightEMU {
		t.Fatalf("description or dimensions changed unexpectedly: %+v", info)
	}
	for _, part := range priorParts {
		if part.header.Name != "word/document.xml" && !bytes.Equal(part.data, s.pkg.partData(part.header.Name, nil)) {
			t.Fatalf("description edit changed %s", part.header.Name)
		}
	}
	alt.Alt = ""
	update.BaseRevision = updated.Revision
	update.Operations[0].TargetID = updated.Blocks[0].Paragraph.Runs[1].ID
	if _, err := s.Apply(ctx, update, "human"); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if info := mustSnapshot(t, reopened).Blocks[0].Paragraph.Runs[1].Picture; info == nil || info.Alt != "" {
		t.Fatalf("cleared description was not preserved: %+v", info)
	}
	historyRestore(t, reopened, "undo")
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(inserted, fileBytes(t, path)) {
		t.Fatal("undo of description changes did not restore the inserted picture")
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(original, fileBytes(t, path)) {
		t.Fatal("undo changed original bytes")
	}
}
