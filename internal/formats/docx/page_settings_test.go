package docx

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestDocumentHeaderParityPreservesSettingsAndSharesHeaderBatch(t *testing.T) {
	for _, existing := range []bool{false, true} {
		ctx := context.Background()
		s, filename := headerDocument(t)
		const settings = "word/custom-settings.xml"
		if existing {
			rel := `<Relationships xmlns="` + packageRelNS + `"><Relationship Id="custom" Type="` + officeRelNS + `/settings" Target="custom-settings.xml"/></Relationships>`
			rewritePackage(t, filename, func(_ string, b []byte) []byte { return b }, map[string][]byte{
				documentRelationships: []byte(rel),
				settings:              []byte(`<w:settings xmlns:w="` + wordNS + `"><w:defaultTabStop w:val="720"/><!-- keep --><w:compat/></w:settings>`),
			})
			var err error
			s, err = Open(ctx, filename)
			if err != nil {
				t.Fatal(err)
			}
		}
		original := fileBytes(t, filename)
		before := mustSnapshot(t, s)
		text, enabled := "even page", true
		batch := headerBatch(before, 0, HeaderFooterUpdate{Kind: "header", Variant: "even", Text: &text})
		op := Operation{Type: "update_header_parity", EvenAndOddHeaders: &enabled}
		batch.Operations = append(batch.Operations, op, op)
		if _, err := s.Apply(ctx, batch, "human"); err == nil || !bytes.Equal(original, fileBytes(t, filename)) {
			t.Fatal("duplicate settings update must reject the entire batch")
		}
		batch.Operations = batch.Operations[:2]
		after, err := s.Apply(ctx, batch, "human")
		if err != nil || after.EvenAndOddHeaders == nil || !*after.EvenAndOddHeaders || headerSlot(t, after, 0, "header", "even").Text != text {
			t.Fatalf("settings and header batch: %v %+v", err, after)
		}
		if existing {
			raw := string(s.pkg.partData(settings, nil))
			position := strings.Index(raw, "<dwf:evenAndOddHeaders")
			if !strings.Contains(raw, `<w:defaultTabStop w:val="720"/><!-- keep -->`) || position < 0 || position >= strings.Index(raw, "<w:compat/>") {
				t.Fatalf("aliased settings were not patched in order: %s", raw)
			}
		}
		reopened, err := Open(ctx, filename)
		if err != nil {
			t.Fatal(err)
		}
		if parity := mustSnapshot(t, reopened).EvenAndOddHeaders; parity == nil || !*parity {
			t.Fatal("settings did not survive reopen")
		}
		historyRestore(t, reopened, "undo")
		if !bytes.Equal(original, fileBytes(t, filename)) {
			t.Fatal("undo changed original bytes")
		}
	}
}
