package docx

import (
	"encoding/xml"
	"fmt"
	"path"
	"strings"
)

const officeRelNS = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"

type HeaderFooterUpdate struct {
	Kind         string  `json:"kind"`
	Variant      string  `json:"variant"`
	LinkPrevious bool    `json:"linkPrevious,omitempty"`
	Text         *string `json:"text,omitempty"`
}
type HeaderFooterInfo struct {
	Truncated       bool   `json:"truncated,omitempty"`
	Kind            string `json:"kind"`
	Variant         string `json:"variant"`
	Linked          bool   `json:"linked"`
	SourceSectionID string `json:"sourceSectionId,omitempty"`
	Text            string `json:"text"`
	Editable        bool   `json:"editable"`
}

func attribute(n *node, namespace, name string) string {
	if n == nil {
		return ""
	}
	for _, a := range n.attrs {
		if a.Name.Space == namespace && a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

func (p *documentPackage) relatedPart(id, kind string, changes map[string][]byte) string {
	data := p.partData("word/_rels/document.xml.rels", changes)
	if len(data) == 0 {
		return ""
	}
	root, err := parseXML(data)
	if err != nil {
		return ""
	}
	for _, child := range root.children {
		if attribute(child, "", "Id") != id || attribute(child, "", "Type") != officeRelNS+"/"+kind || attribute(child, "", "TargetMode") == "External" {
			continue
		}
		target := attribute(child, "", "Target")
		if strings.HasPrefix(target, "/") {
			return strings.TrimPrefix(path.Clean(target), "/")
		}
		return path.Clean(path.Join("word", target))
	}
	return ""
}
func (p *documentPackage) relationshipID(filename string, changes map[string][]byte) string {
	root, err := parseXML(p.partData("word/_rels/document.xml.rels", changes))
	if err != nil {
		return ""
	}
	for _, child := range root.children {
		kind := strings.TrimPrefix(attribute(child, "", "Type"), officeRelNS+"/")
		if kind != "header" && kind != "footer" && kind != "settings" || attribute(child, "", "TargetMode") == "External" {
			continue
		}
		target := attribute(child, "", "Target")
		if strings.HasPrefix(target, "/") {
			target = strings.TrimPrefix(path.Clean(target), "/")
		} else {
			target = path.Clean(path.Join("word", target))
		}
		if target == filename {
			return attribute(child, "", "Id")
		}
	}

	return ""
}

func headerReference(props *node, kind, variant string) (*node, error) {
	var found *node
	if props == nil {
		return nil, nil
	}
	for _, child := range props.children {
		if child.is(kind+"Reference") && child.attr("type") == variant {
			if found != nil {
				return nil, fmt.Errorf("duplicate DOCX header/footer reference")
			}
			found = child
		}
	}
	return found, nil
}
func plainHeader(data []byte, kind string) (string, bool) {
	root, err := parseXML(data)
	tag := "hdr"
	if kind == "footer" {
		tag = "ftr"
	}
	if err != nil || !root.is(tag) || !onlyNamespaceAttributes(root) || strings.TrimSpace(root.text.String()) != "" {
		return "", false
	}
	texts := []string{}
	editable := true
	for _, paragraph := range root.children {
		if !paragraph.is("p") || !onlyNamespaceAttributes(paragraph) || paragraph.child("pPr") != nil || strings.TrimSpace(paragraph.text.String()) != "" {
			editable = false
		}
		for _, run := range paragraph.children {
			if !run.is("r") || !onlyNamespaceAttributes(run) || run.child("rPr") != nil || strings.TrimSpace(run.text.String()) != "" {
				editable = false
			}
			for _, child := range run.children {
				if !child.is("t") || !plainTextAttributes(child) || len(child.children) > 0 {
					editable = false
				}
			}
		}
		texts = append(texts, textContent(paragraph))
	}
	text := strings.Join(texts, "\n")
	if !validInlineText(text) || len(root.children) > 100 {
		return "", false
	}
	return text, editable
}
func onlyNamespaceAttributes(n *node) bool {
	for _, a := range n.attrs {
		if !isNamespaceAttribute(a) {
			return false
		}
	}
	return true
}

// Resolve references once per snapshot and omit inherited text duplicates. A
// bounded preview cannot turn a large or opaque header into an editable blank.
func (p *documentPackage) headerFooterCatalog(targets []sectionTarget) [][]HeaderFooterInfo {
	result := make([][]HeaderFooterInfo, len(targets))
	previous := map[string]HeaderFooterInfo{}
	relations := map[string]string{}
	if root, err := parseXML(p.partData("word/_rels/document.xml.rels", nil)); err == nil {
		for _, child := range root.children {
			kind := strings.TrimPrefix(attribute(child, "", "Type"), officeRelNS+"/")
			if kind != "header" && kind != "footer" || attribute(child, "", "TargetMode") == "External" {
				continue
			}
			target := attribute(child, "", "Target")
			if strings.HasPrefix(target, "/") {
				target = strings.TrimPrefix(path.Clean(target), "/")
			} else {
				target = path.Clean(path.Join("word", target))
			}
			relations[kind+":"+attribute(child, "", "Id")] = target
		}
	}
	type content struct {
		text     string
		editable bool
	}
	cache := map[string]content{}
	remaining := 1 << 20
	for index, target := range targets {
		for _, kind := range []string{"header", "footer"} {
			for _, variant := range []string{"default", "first", "even"} {
				key := kind + ":" + variant
				info := HeaderFooterInfo{Kind: kind, Variant: variant, Linked: index > 0, Editable: true}
				ref, err := headerReference(target.props, kind, variant)
				if err != nil {
					info.Editable = false
				} else if ref == nil {
					if inherited, ok := previous[key]; ok {
						info.SourceSectionID = inherited.SourceSectionID
						info.Editable = inherited.Editable
						info.Truncated = inherited.Truncated
					}
				} else {
					info.Linked = false
					info.SourceSectionID = target.info.ID
					filename := relations[kind+":"+attribute(ref, officeRelNS, "id")]
					item, ok := cache[filename]
					if !ok {
						item.text, item.editable = plainHeader(p.partData(filename, nil), kind)
						cache[filename] = item
					}
					info.Text, info.Editable = item.text, item.editable && plainHeaderReference(ref)
					if len(info.Text) > remaining {
						info.Text = ""
						info.Editable = false
						info.Truncated = true
					} else {
						remaining -= len(info.Text)
					}
				}
				previous[key] = info
				result[index] = append(result[index], info)
			}
		}
	}
	return result
}

// Every edit gets a fresh part and reference. A shared imported header is never
// overwritten through another section, and unrelated original parts stay intact.
func (p *documentPackage) patchHeaderFooters(raw string, updates []HeaderFooterUpdate, sectionIndex int, changes map[string][]byte) (string, error) {
	if len(updates) > 6 {
		return "", fmt.Errorf("DOCX section permits six header/footer variants")
	}
	seen := map[string]bool{}
	for _, update := range updates {
		key := update.Kind + ":" + update.Variant
		if (update.Kind != "header" && update.Kind != "footer") || (update.Variant != "default" && update.Variant != "first" && update.Variant != "even") || seen[key] {
			return "", fmt.Errorf("invalid or duplicate DOCX header/footer variant")
		}
		seen[key] = true
		if update.LinkPrevious && (sectionIndex == 0 || update.Text != nil) || !update.LinkPrevious && (update.Text == nil || !validInlineText(*update.Text)) {
			return "", fmt.Errorf("DOCX header/footer requires text or a previous section link")
		}
		data := []byte(raw)
		root, err := parseXML(data)
		if err != nil {
			return "", err
		}
		ref, err := headerReference(root, update.Kind, update.Variant)
		if err != nil {
			return "", err
		}
		if ref != nil && !plainHeaderReference(ref) {
			return "", fmt.Errorf("opaque DOCX header/footer reference is protected")
		}

		replacement := ""
		if !update.LinkPrevious {
			if ref != nil {
				filename := p.relatedPart(attribute(ref, officeRelNS, "id"), update.Kind, changes)
				if _, editable := plainHeader(p.partData(filename, changes), update.Kind); !editable {
					return "", fmt.Errorf("formatted or protected DOCX header/footer cannot be flattened")
				}
			}
			filename, err := p.addWordPart(changes, update.Kind, "rIdDocx"+update.Kind)
			if err != nil {
				return "", err
			}
			tag := "hdr"
			if update.Kind == "footer" {
				tag = "ftr"
			}
			var content strings.Builder
			content.WriteString(`<w:` + tag + ` xmlns:w="` + wordNS + `">`)
			for _, line := range strings.Split(*update.Text, "\n") {
				content.WriteString(`<w:p><w:r>` + inlineXML(line) + `</w:r></w:p>`)
			}
			content.WriteString(`</w:` + tag + `>`)
			changes[filename] = []byte(content.String())
			id := p.relationshipID(filename, changes)
			if id == "" {
				return "", fmt.Errorf("DOCX header/footer relationship was not created")
			}
			replacement = `<dwh:` + update.Kind + `Reference xmlns:dwh="` + wordNS + `" xmlns:r="` + officeRelNS + `" dwh:type="` + update.Variant + `" r:id="` + id + `"/>`
		}
		if ref != nil {
			raw = string(data[:ref.start]) + replacement + string(data[ref.end:])
		} else if replacement != "" {
			// References precede scalar section properties. Header references also precede
			// footer references, while each existing variant keeps its original order.
			at := root.inner
			for _, child := range root.children {
				if update.Kind == "header" && !child.is("headerReference") || update.Kind == "footer" && !child.is("headerReference") && !child.is("footerReference") {
					at = child.start
					break
				}
				at = child.end
			}
			raw = string(data[:at]) + replacement + string(data[at:])
		}
	}
	return raw, nil
}

func isNamespaceAttribute(a xml.Attr) bool {
	return a.Name.Space == "xmlns" || a.Name.Space == "" && a.Name.Local == "xmlns"
}

func plainHeaderReference(ref *node) bool {
	if len(ref.children) > 0 || strings.TrimSpace(ref.text.String()) != "" {
		return false
	}
	for _, a := range ref.attrs {
		if !isNamespaceAttribute(a) && !(a.Name.Space == wordNS && a.Name.Local == "type") && !(a.Name.Space == officeRelNS && a.Name.Local == "id") {
			return false
		}
	}
	return true
}
