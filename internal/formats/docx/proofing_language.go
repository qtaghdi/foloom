package docx

import (
	"regexp"

	"golang.org/x/text/language"
)

var proofingLanguageTag = regexp.MustCompile(`^[A-Za-z0-9]+(-[A-Za-z0-9]+)*$`)

func validProofingLanguage(tag string) bool {
	if len(tag) > 35 || !proofingLanguageTag.MatchString(tag) {
		return false
	}
	_, err := language.Parse(tag)
	return err == nil
}

func previewProofingLanguages(run *Run, props *node) {
	if props == nil {
		return
	}
	run.ProofingLang = ""
	run.EastAsiaLang = ""
	if tag := props.attr("val"); validProofingLanguage(tag) {
		run.ProofingLang = tag
	}
	if tag := props.attr("eastAsia"); validProofingLanguage(tag) {
		run.EastAsiaLang = tag
	}
}

func patchProofingLanguages(props *node, data []byte, style *InlineStyle) (string, error) {
	values := map[string]string{}
	if style.ProofingLang != "" {
		values["val"] = style.ProofingLang
	}
	if style.EastAsiaLang != "" {
		values["eastAsia"] = style.EastAsiaLang
	}
	return patchProperty(props, data, "lang", values, nil)
}

// A direct w:lang replaces the inherited language element as a whole. Carry
// the effective source element forward so untouched script attributes survive.
func (s previewStyles) inheritedProofingLanguages(t target) *node {
	for _, style := range s.chain(t.run.child("rPr").child("rStyle").attr("val"), "character") {
		if lang := style.child("rPr").child("lang"); lang != nil {
			return lang
		}
	}
	styleID := t.paragraph.child("pPr").child("pStyle").attr("val")
	if styleID == "" {
		styleID = s.paragraph
	}
	for _, style := range s.chain(styleID, "paragraph") {
		if lang := style.child("rPr").child("lang"); lang != nil {
			return lang
		}
	}
	return s.defaults.child("rPrDefault").child("rPr").child("lang")
}

func withProofingSource(direct, inherited *node, data []byte) (*node, []byte) {
	if direct != nil || inherited == nil {
		return direct, data
	}
	root := inherited
	for root.parent != nil {
		root = root.parent
	}
	return inherited, root.source
}
