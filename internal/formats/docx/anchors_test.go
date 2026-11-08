package docx

import "testing"

func TestNativeParagraphAnchorsOmitInvalidAndDuplicateIDs(t *testing.T) {
	s, _ := withDocument(t, bodyXML(`<w:p xmlns:q="`+word2010NS+`" q:paraId="0000002a"/><w:p xmlns:q="`+word2010NS+`" q:paraId="0000002B"/><w:p xmlns:q="`+word2010NS+`" q:paraId="0000002b"/><w:p xmlns:q="`+word2010NS+`" q:paraId="80000000"/>`))
	v := mustSnapshot(t, s)
	for i, expected := range []string{"0000002A", "", "", ""} {
		if v.Blocks[i].Paragraph.Anchor != expected {
			t.Fatalf("anchor %d: %q", i, v.Blocks[i].Paragraph.Anchor)
		}
	}
}
