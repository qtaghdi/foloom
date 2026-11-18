package docx

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/jpeg"
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
	contentTypes := string(s.pkg.partData("[Content_Types].xml", nil))
	relationships := string(s.pkg.partData(documentRelationships, nil))
	if !strings.Contains(contentTypes, `<Override xmlns="http://schemas.openxmlformats.org/package/2006/content-types" PartName="/`+media+`"`) ||
		!strings.Contains(relationships, `<Relationship xmlns="`+packageRelNS+`" Id="rIdFoloomImage1"`) {
		t.Fatal("new package metadata uses an incompatible namespace prefix")
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
	beforeResize := fileBytes(t, path)
	before := mustSnapshot(t, reopened)
	resized, err := reopened.Apply(ctx, ApplyInput{SessionID: before.ID, BaseRevision: before.Revision, Operations: []Operation{
		{Type: "update_image", TargetID: before.Blocks[0].Paragraph.Runs[1].ID, Image: &ImageEdit{Action: "resize", WidthEMU: 600000}},
	}}, "human")
	if err != nil {
		t.Fatal(err)
	}
	size := resized.Blocks[0].Paragraph.Runs[1].Picture
	if size == nil || size.WidthEMU != 600000 || size.HeightEMU != 300000 || !bytes.Equal(reopened.pkg.partData(media, nil), pngBytes.Bytes()) ||
		!strings.Contains(string(reopened.pkg.document), `cx="600000" cy="300000"`) {
		t.Fatal("resize did not preserve the aspect ratio and media")
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(beforeResize, fileBytes(t, path)) {
		t.Fatal("undo did not restore the original picture size")
	}
	beforeCrop := fileBytes(t, path)
	before = mustSnapshot(t, reopened)
	crop := ImageCrop{Left: 80000, Right: 30000}
	cropInput := ApplyInput{SessionID: before.ID, BaseRevision: before.Revision, Operations: []Operation{
		{Type: "update_image", TargetID: before.Blocks[0].Paragraph.Runs[1].ID, Image: &ImageEdit{Action: "crop", Crop: &crop}},
	}}
	if _, err := reopened.Apply(ctx, cropInput, "human"); err == nil || !bytes.Equal(beforeCrop, fileBytes(t, path)) {
		t.Fatal("invalid crop changed the picture")
	}
	crop = ImageCrop{Left: 25000, Top: 12345}
	if _, err := reopened.Apply(ctx, cropInput, "human"); err != nil {
		t.Fatal(err)
	}
	reopened, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	cropped := mustSnapshot(t, reopened)
	croppedRun := cropped.Blocks[0].Paragraph.Runs[1]
	if croppedRun.Picture == nil || croppedRun.Picture.Crop == nil || *croppedRun.Picture.Crop != crop ||
		croppedRun.Picture.WidthEMU != 400000 || croppedRun.Picture.HeightEMU != 200000 ||
		!bytes.Equal(reopened.pkg.partData(media, nil), pngBytes.Bytes()) {
		t.Fatal("crop or original image was not preserved after reopen")
	}
	crop = ImageCrop{}
	cropInput.SessionID, cropInput.BaseRevision, cropInput.Operations[0].TargetID = cropped.ID, cropped.Revision, croppedRun.ID
	clear, err := reopened.Apply(ctx, cropInput, "human")
	if err != nil || clear.Blocks[0].Paragraph.Runs[1].Picture.Crop != nil {
		t.Fatal("crop reset did not remove the source rectangle", err)
	}
	historyRestore(t, reopened, "undo")
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(beforeCrop, fileBytes(t, path)) {
		t.Fatal("undo did not restore original picture crop")
	}
	beforeReplace := fileBytes(t, path)
	before = mustSnapshot(t, reopened)
	var jpegBytes bytes.Buffer
	if err := jpeg.Encode(&jpegBytes, image.NewRGBA(image.Rect(0, 0, 2, 4)), nil); err != nil {
		t.Fatal(err)
	}
	replace := ImageEdit{Action: "replace", Base64: "c3Zn"}
	batch = ApplyInput{SessionID: before.ID, BaseRevision: before.Revision, Operations: []Operation{
		{Type: "update_image", TargetID: before.Blocks[0].Paragraph.Runs[1].ID, Image: &replace},
	}}
	if _, err := reopened.Apply(ctx, batch, "human"); err == nil || !bytes.Equal(beforeReplace, fileBytes(t, path)) {
		t.Fatal("invalid replacement was persisted")
	}
	replace.Base64 = base64.StdEncoding.EncodeToString(jpegBytes.Bytes())
	replaced, err := reopened.Apply(ctx, batch, "human")
	if err != nil {
		t.Fatal(err)
	}
	replacedRun := replaced.Blocks[0].Paragraph.Runs[1]
	if replacedRun.Picture == nil || *replacedRun.Picture != *before.Blocks[0].Paragraph.Runs[1].Picture ||
		reopened.pkg.relatedPart("rIdFoloomImage1", "image", nil) != media ||
		!bytes.Equal(reopened.pkg.partData(media, nil), pngBytes.Bytes()) {
		t.Fatal("image replacement changed picture geometry or its original media")
	}
	reopened, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	payload, mime, err = reopened.Picture(ctx, mustSnapshot(t, reopened).Blocks[0].Paragraph.Runs[1].ID)
	if err != nil || mime != "image/jpeg" || !bytes.Equal(payload, jpegBytes.Bytes()) {
		t.Fatal("new picture could not be read after reopening")
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(beforeReplace, fileBytes(t, path)) {
		t.Fatal("undo did not restore the original picture")
	}
	beforeDelete := fileBytes(t, path)
	before = mustSnapshot(t, reopened)
	after, err := reopened.Apply(ctx, ApplyInput{SessionID: before.ID, BaseRevision: before.Revision, Operations: []Operation{
		{Type: "update_image", TargetID: before.Blocks[0].Paragraph.Runs[1].ID, Image: &ImageEdit{Action: "delete"}},
	}}, "human")
	if err != nil || len(after.Blocks[0].Paragraph.Runs) != 1 || after.Blocks[0].Paragraph.Runs[0].Text != "first" {
		t.Fatalf("deletion did not preserve surrounding text: %v", err)
	}
	if reopened.pkg.relatedPart("rIdFoloomImage1", "image", nil) != media || !bytes.Equal(reopened.pkg.partData(media, nil), pngBytes.Bytes()) {
		t.Fatal("deletion changed the relationship or media part")
	}
	reopened, err = Open(ctx, path)
	if err != nil || len(mustSnapshot(t, reopened).Blocks[0].Paragraph.Runs) != 1 {
		t.Fatalf("deletion did not survive reopening: %v", err)
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(beforeDelete, fileBytes(t, path)) {
		t.Fatal("undo did not restore the deleted picture")
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

func TestDOCXInlinePictureHalfTurnKeepsCropAndMedia(t *testing.T) {
	ctx := context.Background()
	s, filename := withDocument(t, bodyXML(`<w:p><w:r><w:t>image</w:t></w:r></w:p>`))
	var pngBytes bytes.Buffer
	if err := png.Encode(&pngBytes, image.NewRGBA(image.Rect(0, 0, 4, 2))); err != nil {
		t.Fatal(err)
	}
	apply := func(v Snapshot, target string, edit *ImageEdit) (Snapshot, error) {
		return s.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
			{Type: "update_image", TargetID: target, Image: edit},
		}}, "human")
	}
	v := mustSnapshot(t, s)
	v, err := apply(v, v.Blocks[0].ID, &ImageEdit{Action: "insert", Base64: base64.StdEncoding.EncodeToString(pngBytes.Bytes()), Alt: "original", WidthEMU: 400000})
	if err != nil {
		t.Fatal(err)
	}
	v, err = apply(v, v.Blocks[0].Paragraph.Runs[1].ID, &ImageEdit{Action: "crop", Crop: &ImageCrop{Left: 10000}})
	if err != nil {
		t.Fatal(err)
	}
	before := fileBytes(t, filename)
	angle := 45
	run := v.Blocks[0].Paragraph.Runs[1]
	if _, err := apply(v, run.ID, &ImageEdit{Action: "rotate", Rotation: &angle}); err == nil ||
		!bytes.Equal(before, fileBytes(t, filename)) {
		t.Fatal("unsupported rotation changed the package")
	}
	angle = 180
	if _, err := apply(v, run.ID, &ImageEdit{Action: "rotate", Rotation: &angle}); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	v = mustSnapshot(t, reopened)
	rotated := v.Blocks[0].Paragraph.Runs[1]
	p := rotated.Picture
	if p == nil || !p.RotationEditable || p.Rotation != 180 || p.Crop == nil || p.Crop.Left != 10000 ||
		p.WidthEMU != 400000 || p.HeightEMU != 200000 || p.Alt != "original" ||
		!strings.Contains(string(reopened.pkg.document), `rot="10800000"`) {
		t.Fatalf("rotated picture lost metadata after reopen: %+v", p)
	}
	media, _, err := reopened.Picture(ctx, rotated.ID)
	if err != nil || !bytes.Equal(media, pngBytes.Bytes()) {
		t.Fatal("rotation changed media bytes")
	}
	s = reopened
	angle = 0
	cleared, err := apply(v, rotated.ID, &ImageEdit{Action: "rotate", Rotation: &angle})
	if err != nil || cleared.Blocks[0].Paragraph.Runs[1].Picture.Rotation != 0 {
		t.Fatal("picture could not return to its original direction", err)
	}
	historyRestore(t, reopened, "undo")
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(before, fileBytes(t, filename)) {
		t.Fatal("undo did not restore the original picture XML")
	}
}

func TestDOCXInlinePictureQuarterTurnsKeepIntrinsicSize(t *testing.T) {
	ctx := context.Background()
	s, filename := withDocument(t, bodyXML(`<w:p><w:r><w:t>image</w:t></w:r></w:p>`))
	var pngBytes bytes.Buffer
	if err := png.Encode(&pngBytes, image.NewRGBA(image.Rect(0, 0, 4, 2))); err != nil {
		t.Fatal(err)
	}
	apply := func(s *Session, id string, edit ImageEdit) (Snapshot, error) {
		v := mustSnapshot(t, s)
		return s.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
			{Type: "update_image", TargetID: id, Image: &edit},
		}}, "human")
	}
	v := mustSnapshot(t, s)
	v, err := apply(s, v.Blocks[0].ID, ImageEdit{Action: "insert", Base64: base64.StdEncoding.EncodeToString(pngBytes.Bytes()), Alt: "quarter turns", WidthEMU: 400000})
	if err != nil {
		t.Fatal(err)
	}
	v, err = apply(s, v.Blocks[0].Paragraph.Runs[1].ID, ImageEdit{Action: "crop", Crop: &ImageCrop{Left: 12000, Bottom: 5000}})
	if err != nil {
		t.Fatal(err)
	}
	baseline := fileBytes(t, filename)
	before := s.pkg.document
	for _, tc := range []struct {
		angle int
		units string
	}{{90, "5400000"}, {270, "16200000"}, {0, ""}} {
		run := v.Blocks[0].Paragraph.Runs[1]
		if _, err := apply(s, run.ID, ImageEdit{Action: "rotate", Rotation: &tc.angle}); err != nil {
			t.Fatal(err)
		}
		s, err = Open(ctx, filename)
		if err != nil {
			t.Fatal(err)
		}
		v = mustSnapshot(t, s)
		run = v.Blocks[0].Paragraph.Runs[1]
		picture := run.Picture
		if picture == nil || picture.Rotation != tc.angle || !picture.RotationEditable ||
			picture.WidthEMU != 400000 || picture.HeightEMU != 200000 || picture.Alt != "quarter turns" ||
			picture.Crop == nil || *picture.Crop != (ImageCrop{Left: 12000, Bottom: 5000}) {
			t.Fatalf("angle %d reopens as %+v", tc.angle, picture)
		}
		if tc.units != "" && !strings.Contains(string(s.pkg.document), `rot="`+tc.units+`"`) {
			t.Fatalf("direction %d was not saved in the native transform", tc.angle)
		}
		if tc.angle == 0 && strings.Contains(string(s.pkg.document), ` rot="`) {
			t.Fatal("restoring the default direction must remove the rotation")
		}
		media, _, err := s.Picture(ctx, run.ID)
		if err != nil || !bytes.Equal(media, pngBytes.Bytes()) {
			t.Fatal("rotation changed media bytes")
		}
	}
	for range 3 {
		historyRestore(t, s, "undo")
	}
	if !bytes.Equal(before, s.pkg.document) || !bytes.Equal(baseline, fileBytes(t, filename)) {
		t.Fatal("undo did not recover the exact original document")
	}
}
