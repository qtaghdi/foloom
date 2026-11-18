package docx

import (
	"maps"
	"testing"
)

func TestContinuousNoteLabels(t *testing.T) {
	const first = `<w:p><w:r><w:footnoteReference w:id="9"/></w:r><w:r><w:endnoteReference w:id="3"/></w:r></w:p>`
	const second = `<w:p><w:r><w:footnoteReference w:id="2"/></w:r><w:r><w:endnoteReference w:id="4"/></w:r></w:p>`
	parse := func(raw string) *node {
		t.Helper()
		n, err := parseXML([]byte(`<w:root xmlns:w="` + wordNS + `">` + raw + `</w:root>`))
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	body := parse(first + second)
	footnotes := map[string]bool{"2": true, "9": true}
	endnotes := map[string]bool{"3": true, "4": true}
	settings := parse(`<w:footnotePr><w:numStart w:val="5"/><w:numRestart w:val="continuous"/></w:footnotePr><w:endnotePr><w:numFmt w:val="upperRoman"/><w:numStart w:val="4"/></w:endnotePr>`)
	if actual := continuousNoteLabels("footnote", body, settings, footnotes); !maps.Equal(actual, map[string]string{"9": "5", "2": "6"}) {
		t.Fatalf("footnote labels followed IDs instead of body order: %v", actual)
	}
	if actual := continuousNoteLabels("endnote", body, settings, endnotes); !maps.Equal(actual, map[string]string{"3": "IV", "4": "V"}) {
		t.Fatalf("incorrect endnote labels: %v", actual)
	}
	section := parse(first + second + `<w:sectPr><w:footnotePr><w:numFmt w:val="lowerRoman"/></w:footnotePr></w:sectPr>`)
	if actual := continuousNoteLabels("footnote", section, settings, footnotes); !maps.Equal(actual, map[string]string{"9": "v", "2": "vi"}) {
		t.Fatalf("section format did not override document format and inherit its start: %v", actual)
	}
	if actual := continuousNoteLabels("endnote", body, nil, endnotes); !maps.Equal(actual, map[string]string{"3": "i", "4": "ii"}) {
		t.Fatalf("incorrect default Word endnote labels: %v", actual)
	}
	for _, tc := range []struct{ text, properties string }{
		{text: first + first},
		{text: first + `<w:p><w:r><w:footnoteReference w:id="99"/></w:r></w:p>`},
		{text: first + `<w:p><w:r><w:footnoteReference w:id="2" w:customMarkFollows="1"/></w:r></w:p>`},
		{text: first + `<w:tbl><w:tr><w:tc>` + second + `</w:tc></w:tr></w:tbl>`},
		{text: first + second + `<w:sectPr><w:footnotePr><w:numRestart w:val="eachSect"/></w:footnotePr></w:sectPr>`},
		{text: first + second, properties: `<w:footnotePr><w:numRestart w:val="eachPage"/></w:footnotePr>`},
		{text: first + second, properties: `<w:footnotePr><w:numFmt w:val="chicago"/></w:footnotePr>`},
	} {
		if actual := continuousNoteLabels("footnote", parse(tc.text), parse(tc.properties), footnotes); actual != nil {
			t.Fatalf("ambiguous numbering was projected for %+v: %v", tc, actual)
		}
	}
}
