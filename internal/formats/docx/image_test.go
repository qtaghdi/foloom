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

func TestPictureCaptionRenumbersExistingFigureAndKeepsMedia(t *testing.T) {
	ctx := context.Background()
	s, filename := withDocument(t, bodyXML(`<w:p><w:r><w:t>first image</w:t></w:r></w:p><w:p><w:r><w:t>second image</w:t></w:r></w:p>`))
	v := mustSnapshot(t, s)
	var pngBytes bytes.Buffer
	if err := png.Encode(&pngBytes, image.NewRGBA(image.Rect(0, 0, 4, 2))); err != nil {
		t.Fatal(err)
	}
	picture := &ImageEdit{Action: "insert", Base64: base64.StdEncoding.EncodeToString(pngBytes.Bytes())}
	save := func(v Snapshot, ops ...Operation) (Snapshot, error) {
		return s.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: ops}, "human")
	}
	v, err := save(v, Operation{Type: "update_image", TargetID: v.Blocks[0].ID, Image: picture},
		Operation{Type: "update_image", TargetID: v.Blocks[1].ID, Image: picture})
	if err != nil {
		t.Fatal(err)
	}
	original := fileBytes(t, filename)
	media := s.pkg.picturePartMap()
	if !v.Blocks[0].Paragraph.Runs[1].Picture.CaptionEditable || !v.Blocks[1].Paragraph.Runs[1].Picture.CaptionEditable {
		t.Fatal("supported body pictures were not offered captions")
	}
	v, err = save(v, Operation{Type: "insert_picture_caption", TargetID: v.Blocks[1].Paragraph.Runs[1].ID, Text: "Second & image"})
	if err != nil || v.Blocks[2].Paragraph.Runs[1].Text != "1" {
		t.Fatal("initial figure caption missing", err)
	}
	v, err = save(v, Operation{Type: "insert_picture_caption", TargetID: v.Blocks[0].Paragraph.Runs[1].ID, Text: "First image"})
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	v = mustSnapshot(t, reopened)
	if len(v.Blocks) != 4 || v.Blocks[1].Paragraph.Runs[1].Text != "1" || v.Blocks[3].Paragraph.Runs[1].Text != "2" ||
		v.Blocks[3].Paragraph.Runs[2].Text != ". Second & image" || v.Blocks[0].Paragraph.Runs[1].Picture.CaptionEditable {
		t.Fatal("caption numbers, captions or duplicate guard did not survive open")
	}
	for _, name := range media {
		if !bytes.Equal(pngBytes.Bytes(), reopened.pkg.partData(name, nil)) {
			t.Fatal("caption changed original image bytes")
		}
	}
	committed := fileBytes(t, filename)
	s = reopened
	if _, err := save(v, Operation{Type: "insert_picture_caption", TargetID: v.Blocks[0].Paragraph.Runs[1].ID, Text: "Duplicate"}); err == nil ||
		!bytes.Equal(committed, fileBytes(t, filename)) {
		t.Fatal("duplicate caption changed the document")
	}
	pictureRun := v.Blocks[2].Paragraph.Runs[1]
	if pictureRun.Picture.Caption != "Second & image" {
		t.Fatal("the picture did not expose its own caption text")
	}
	edit := Operation{Type: "edit_picture_caption", TargetID: pictureRun.ID, ExpectedText: "stale", Text: "새 설명"}
	if _, err := save(v, edit); err == nil || !bytes.Equal(committed, fileBytes(t, filename)) {
		t.Fatal("stale caption text was accepted")
	}
	originalXML := bytes.Clone(s.pkg.document)
	edit.ExpectedText = "Second & image"
	updated, err := save(v, edit)
	if err != nil {
		t.Fatal(err)
	}
	expectedXML := bytes.Replace(originalXML, []byte(". Second &amp; image"), []byte(". 새 설명"), 1)
	if !bytes.Equal(s.pkg.document, expectedXML) ||
		updated.Blocks[2].Paragraph.Runs[1].Picture.Caption != edit.Text ||
		updated.Blocks[3].Paragraph.Runs[1].Text != "2" {
		t.Fatal("editing a caption affected other XML or changed the number")
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(committed, fileBytes(t, filename)) {
		t.Fatal("undo did not restore the original caption")
	}
	historyRestore(t, reopened, "undo")
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(original, fileBytes(t, filename)) {
		t.Fatal("undo changed original picture package bytes")
	}
}

func TestCaptionedPictureMovesAsOnePair(t *testing.T) {
	ctx := context.Background()
	const textBefore = `<w:p><w:r><w:t>Before</w:t></w:r></w:p>`
	const textAfter = `<w:p><w:r><w:t>After</w:t></w:r></w:p>`
	s, filename := withDocument(t, bodyXML(textBefore+"\n  <w:p/>\n"+textAfter))
	save := func(s *Session, v Snapshot, ops ...Operation) (Snapshot, error) {
		return s.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: ops}, "human")
	}
	v := mustSnapshot(t, s)
	var pngBytes bytes.Buffer
	if err := png.Encode(&pngBytes, image.NewRGBA(image.Rect(0, 0, 4, 2))); err != nil {
		t.Fatal(err)
	}
	picture := &ImageEdit{Action: "insert", Base64: base64.StdEncoding.EncodeToString(pngBytes.Bytes()), Alt: "fidelity"}
	v, err := save(s, v, Operation{Type: "update_image", TargetID: v.Blocks[1].ID, Image: picture})
	if err != nil {
		t.Fatal(err)
	}
	pictureRun := v.Blocks[1].Paragraph.Runs[0]
	if pictureRun.Picture.MoveCaptionUp || pictureRun.Picture.MoveCaptionDown {
		t.Fatal("uncaptioned image offered movement")
	}
	v, err = save(s, v, Operation{Type: "insert_picture_caption", TargetID: pictureRun.ID, Text: "First caption"})
	if err != nil {
		t.Fatal(err)
	}
	pictureRun = v.Blocks[1].Paragraph.Runs[0]
	if !pictureRun.Picture.MoveCaptionUp || !pictureRun.Picture.MoveCaptionDown {
		t.Fatal("both directions should be available")
	}
	original := fileBytes(t, filename)
	baseline := bytes.Clone(s.pkg.document)
	up := Operation{Type: "move_picture_caption_up", TargetID: pictureRun.ID}
	if _, err := save(s, v, up, up); err == nil || !bytes.Equal(original, fileBytes(t, filename)) {
		t.Fatal("movement accepted a compound operation")
	}
	v, err = save(s, v, up)
	if err != nil {
		t.Fatal(err)
	}
	pictureRun = v.Blocks[0].Paragraph.Runs[0]
	if pictureRun.Picture.Alt != "fidelity" || pictureRun.Picture.Caption != "First caption" ||
		pictureRun.Picture.MoveCaptionUp || !pictureRun.Picture.MoveCaptionDown ||
		v.Blocks[1].Paragraph.Runs[1].Text != "1" || v.Blocks[2].Paragraph.Runs[0].Text != "Before" {
		t.Fatal("picture, caption, number or new order did not survive movement")
	}
	moved := fileBytes(t, filename)
	if _, err := save(s, v, up); err == nil || !bytes.Equal(moved, fileBytes(t, filename)) {
		t.Fatal("movement used an earlier picture run ID")
	}
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	v = mustSnapshot(t, reopened)
	run := v.Blocks[0].Paragraph.Runs[0]
	if media, _, err := reopened.Picture(ctx, run.ID); err != nil || !bytes.Equal(media, pngBytes.Bytes()) {
		t.Fatal("reopened image bytes differed", err)
	}
	if _, err := save(reopened, v, Operation{Type: "move_picture_caption_up", TargetID: run.ID}); err == nil ||
		!bytes.Equal(fileBytes(t, filename), moved) {
		t.Fatal("movement accepted a forbidden direction")
	}
	if _, err := save(reopened, v, Operation{Type: "move_picture_caption_down", TargetID: run.ID}); err != nil ||
		!bytes.Equal(baseline, reopened.pkg.document) {
		t.Fatal("inverse move did not restore original XML bytes", err)
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(fileBytes(t, filename), moved) {
		t.Fatal("undo did not restore the exact moved package")
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(fileBytes(t, filename), original) {
		t.Fatal("undo did not restore the exact original package")
	}
}

func TestCaptionMoveProtectsComplexBoundaries(t *testing.T) {
	const plain = `<w:p><w:r><w:t>Plain</w:t></w:r></w:p>`
	const label = `<w:r><w:t>그림 </w:t></w:r><w:fldSimple w:instr="SEQ Figure \* ARABIC"><w:r><w:t>1</w:t></w:r></w:fldSimple><w:r><w:t>. Caption</w:t></w:r>`
	picture := inlinePicture("1", "rId1", "", 100000, 100000)
	for _, tc := range []struct {
		name, before, picture, gap, caption, after string
	}{
		{name: "section", before: `<w:p><w:pPr><w:sectPr/></w:pPr></w:p>`},
		{name: "table", before: `<w:tbl/>`},
		{name: "mixed-picture-and-text", before: plain, picture: `<w:r><w:t>Text</w:t></w:r>`},
		{name: "intervening-markup", before: plain, gap: "<!-- private marker -->"},
		{name: "caption-style", before: plain, caption: `<w:pPr><w:pStyle w:val="Caption"/></w:pPr>`},
		{name: "bookmark-spanning-picture", before: `<w:p><w:bookmarkStart w:id="1" w:name="B"/></w:p>` + plain,
			after: `<w:p><w:bookmarkEnd w:id="1"/></w:p>`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, targets, _, err := inspect([]byte(bodyXML(tc.before+"<w:p>"+tc.picture+picture+"</w:p>"+tc.gap+
				"<w:p>"+tc.caption+label+"</w:p>"+tc.after)), "test", 1)
			if err != nil {
				t.Fatal(err)
			}
			for _, target := range targets {
				if target.kind == "picture" {
					if target.moveUp != nil || target.moveDown != nil {
						t.Fatal("protected boundary was offered for movement")
					}
					return
				}
			}
			t.Fatal("test did not contain a picture target")
		})
	}
}

func TestDOCXPicturePlacementPreservesNativeInlineFallback(t *testing.T) {
	ctx := context.Background()
	s, filename := withDocument(t, bodyXML(
		`<w:p><w:pPr><w:jc w:val="right"/></w:pPr></w:p>`+
			`<w:p><w:r><w:t>Mixed text</w:t></w:r></w:p>`))
	apply := func(session *Session, v Snapshot, ops ...Operation) (Snapshot, error) {
		return session.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: ops}, "human")
	}
	var pixels bytes.Buffer
	if err := png.Encode(&pixels, image.NewRGBA(image.Rect(0, 0, 8, 4))); err != nil {
		t.Fatal(err)
	}
	v := mustSnapshot(t, s)
	picture := &ImageEdit{Action: "insert", Base64: base64.StdEncoding.EncodeToString(pixels.Bytes()), Alt: "Illustration"}
	v, err := apply(s, v, Operation{Type: "update_image", TargetID: v.Blocks[0].ID, Image: picture},
		Operation{Type: "update_image", TargetID: v.Blocks[1].ID, Image: picture})
	if err != nil {
		t.Fatal(err)
	}
	run := v.Blocks[0].Paragraph.Runs[0]
	mixed := v.Blocks[1].Paragraph.Runs[1]
	if run.Picture.Placement != "inline" || mixed.Picture.Placement != "" {
		t.Fatal("only the dedicated image paragraph should permit placement")
	}
	original := fileBytes(t, filename)
	originalXML := bytes.Clone(s.pkg.document)
	change := func(id, placement string) Operation {
		return Operation{Type: "update_image", TargetID: id, Image: &ImageEdit{Action: "set_placement", Placement: placement}}
	}
	center := change(run.ID, "floating_center")
	for _, ops := range [][]Operation{
		{center, change(mixed.ID, "floating_left")},
		{change(run.ID, "inline")},
		{change(run.ID, "square")},
	} {
		if _, err := apply(s, v, ops...); err == nil || !bytes.Equal(original, fileBytes(t, filename)) {
			t.Fatal("unsupported placement changed the package")
		}
	}
	v, err = apply(s, v, center)
	if err != nil {
		t.Fatal(err)
	}
	if p := v.Blocks[0].Paragraph; p.Layout == nil || p.Layout.Align != "end" ||
		p.Runs[0].Picture.Placement != "floating_center" || p.Runs[0].Picture.Alt != "Illustration" ||
		!bytes.Contains(s.pkg.document, []byte(`<wp:wrapTopAndBottom/>`)) {
		t.Fatal("native anchor, paragraph alignment, or picture metadata were lost")
	}
	anchored := fileBytes(t, filename)
	for _, changeXML := range [][2]string{
		{"wrapTopAndBottom", "wrapSquare"},
		{`relativeFrom="column"`, `relativeFrom="page"`},
		{`allowOverlap="0"`, `allowOverlap="1"`},
	} {
		raw := bytes.Replace(s.pkg.document, []byte(changeXML[0]), []byte(changeXML[1]), 1)
		if bytes.Equal(raw, s.pkg.document) {
			t.Fatal("test did not change the native anchor")
		}
		_, targets, _, err := inspect(raw, v.ID, v.Revision)
		if err != nil || targets[v.Blocks[0].Paragraph.Runs[0].ID].picturePlacement != "" {
			t.Fatal("unsupported anchor geometry was offered for conversion", err)
		}
	}
	if _, err := apply(s, v, center); err == nil || !bytes.Equal(anchored, fileBytes(t, filename)) {
		t.Fatal("placement accepted an earlier revision's picture ID")
	}
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	v = mustSnapshot(t, reopened)
	run = v.Blocks[0].Paragraph.Runs[0]
	if media, _, err := reopened.Picture(ctx, run.ID); err != nil || !bytes.Equal(media, pixels.Bytes()) {
		t.Fatal("the anchored picture could not be reopened with identical media", err)
	}
	v, err = apply(reopened, v, change(run.ID, "floating_right"))
	if err != nil || v.Blocks[0].Paragraph.Runs[0].Picture.Placement != "floating_right" {
		t.Fatal("repositioning the anchored picture failed", err)
	}
	v, err = apply(reopened, v, change(v.Blocks[0].Paragraph.Runs[0].ID, "inline"))
	if err != nil || !bytes.Equal(originalXML, reopened.pkg.document) ||
		v.Blocks[0].Paragraph.Runs[0].Picture.Placement != "inline" {
		t.Fatal("restoring the inline fallback did not restore the original XML", err)
	}
	historyRestore(t, reopened, "undo")
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(anchored, fileBytes(t, filename)) {
		t.Fatal("undo did not recover the original anchor package")
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(original, fileBytes(t, filename)) {
		t.Fatal("undo did not recover the original inline package")
	}
}
