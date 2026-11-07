package docx

import (
	"fmt"
	"strings"
)

func (p *documentPackage) partData(filename string, changes map[string][]byte) []byte {
	if data, ok := changes[filename]; ok {
		return data
	}
	for _, part := range p.parts {
		if part.header.Name == filename {
			return part.data
		}
	}
	return nil
}

func appendXMLChild(data []byte, child string) ([]byte, error) {
	root, err := parseXML(data)
	if err != nil {
		return nil, err
	}
	if strings.HasSuffix(string(data[root.start:root.inner]), "/>") {
		open, close := scopedOpening(root, data, false)
		return []byte(open + child + close), nil
	}
	return append(append(append([]byte(nil), data[:root.endStart]...), child...), data[root.endStart:]...), nil
}

// addWordPart adds one internal relationship and content type override without
// normalizing other entries, including changes staged by the same batch.
func (p *documentPackage) addWordPart(changes map[string][]byte, kind, relationshipID string) (string, error) {
	reserved := map[string]bool{}
	if root, err := parseXML(p.partData("[Content_Types].xml", changes)); err == nil {
		for _, child := range root.children {
			reserved[strings.TrimPrefix(attribute(child, "", "PartName"), "/")] = true
		}
	}
	filename := "word/" + kind + ".xml"
	for i := 1; p.partData(filename, changes) != nil || reserved[filename]; i++ {
		filename = fmt.Sprintf("word/%s-%d.xml", kind, i)
	}
	const ctPath = "[Content_Types].xml"
	ct, err := appendXMLChild(p.partData(ctPath, changes), `<dwct:Override xmlns:dwct="http://schemas.openxmlformats.org/package/2006/content-types" PartName="/`+filename+`" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.`+kind+`+xml"/>`)
	if err != nil {
		return "", err
	}
	const relPath = "word/_rels/document.xml.rels"
	rels := p.partData(relPath, changes)
	if len(rels) == 0 {
		rels = []byte(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"></Relationships>`)
	}
	root, err := parseXML(rels)
	if err != nil {
		return "", err
	}
	ids := map[string]bool{}
	for _, n := range root.children {
		for _, a := range n.attrs {
			if a.Name.Local == "Id" {
				ids[a.Value] = true
			}
		}
	}
	for ids[relationshipID] {
		relationshipID += "X"
	}
	rels, err = appendXMLChild(rels, `<dwr:Relationship xmlns:dwr="http://schemas.openxmlformats.org/package/2006/relationships" Id="`+relationshipID+`" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/`+kind+`" Target="`+strings.TrimPrefix(filename, "word/")+`"/>`)
	if err != nil {
		return "", err
	}
	changes[ctPath], changes[relPath] = ct, rels
	return filename, nil
}
