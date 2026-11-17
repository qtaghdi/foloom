package docx

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"hash/crc32"
	"os"
	"path/filepath"
	"testing"
)

func TestDOCXStoredPartsHaveCompleteLocalHeadersAfterSaveAndRedo(t *testing.T) {
	ctx := context.Background()
	corpus := filepath.Join("..", "..", "..", "testdata", "docx", "compatibility", "generated-basic.docx")
	source, err := os.ReadFile(corpus)
	if err != nil {
		t.Fatal(err)
	}
	original, err := loadPackage(source)
	if err != nil {
		t.Fatal(err)
	}
	var storedFixture bytes.Buffer
	writer := zip.NewWriter(&storedFixture)
	for _, part := range original.parts {
		size := uint64(len(part.data))
		out, err := writer.CreateRaw(&zip.FileHeader{Name: part.header.Name, Method: zip.Store,
			CRC32: crc32.ChecksumIEEE(part.data), CompressedSize64: size, UncompressedSize64: size})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := out.Write(part.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	source = storedFixture.Bytes()
	name := filepath.Join(t.TempDir(), "stored.docx")
	if err := os.WriteFile(name, source, 0o600); err != nil {
		t.Fatal(err)
	}
	session, err := Open(ctx, name)
	if err != nil {
		t.Fatal(err)
	}
	v := mustSnapshot(t, session)
	run := v.Blocks[0].Paragraph.Runs[0]
	update := ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
		{Type: "replace_run_text", TargetID: run.ID, ExpectedText: run.Text, Text: "Longer replacement text"},
	}}
	if _, err := session.Apply(ctx, update, "human"); err != nil {
		t.Fatal(err)
	}
	saved := fileBytes(t, name)
	historyRestore(t, session, "undo")
	if !bytes.Equal(source, fileBytes(t, name)) {
		t.Fatal("undo changed input bytes")
	}
	historyRestore(t, session, "redo")
	if !bytes.Equal(saved, fileBytes(t, name)) {
		t.Fatal("redo changed committed bytes")
	}
	archive, err := zip.NewReader(bytes.NewReader(saved), int64(len(saved)))
	if err != nil {
		t.Fatal(err)
	}
	stored := 0
	for _, entry := range archive.File {
		if entry.Method != zip.Store {
			continue
		}
		stored++
		offset, err := entry.DataOffset()
		if err != nil {
			t.Fatal(err)
		}
		headerOffset := offset - int64(30+len(entry.Name)+len(entry.Extra))
		header := saved[headerOffset:offset]
		if binary.LittleEndian.Uint32(header[:4]) != 0x04034b50 ||
			binary.LittleEndian.Uint16(header[6:8])&(1<<3) != 0 ||
			binary.LittleEndian.Uint32(header[14:18]) != entry.CRC32 ||
			binary.LittleEndian.Uint32(header[18:22]) != entry.CompressedSize ||
			binary.LittleEndian.Uint32(header[22:26]) != entry.UncompressedSize {
			t.Errorf("incomplete stored ZIP local header for %s", entry.Name)
		}
	}
	if stored == 0 {
		t.Fatal("fixture has no stored entries")
	}
}
