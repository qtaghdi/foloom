package docx

import (
	"bytes"
	"fmt"
	"strings"
	"time"
)

const commentMarkupCompatibilityNS = "http://schemas.openxmlformats.org/markup-compatibility/2006"

func (p *documentPackage) appendCommentReply(root *node, parentID string, edit CommentEdit, actor string, changes map[string][]byte) error {
	if edit.StartRunID != "" || edit.EndRunID != "" || edit.StartOffset != nil || edit.EndOffset != nil ||
		len(edit.Text) > 4000 || !validInlineText(edit.Text) || strings.TrimSpace(edit.Text) == "" {
		return fmt.Errorf("DOCX reply requires plain text")
	}
	filename, commentsRaw, comments, err := p.commentsPart(changes)
	if err != nil || comments == nil {
		return fmt.Errorf("DOCX comments part unavailable")
	}
	extName, extended, err := p.commentExtensions(changes)
	if err != nil || extended == nil {
		return fmt.Errorf("DOCX extended comments part unavailable")
	}
	parent := commentExtensionNodes(comments, extended)[parentID]
	if parent == nil || attribute(parent, commentExtNS, "paraIdParent") != "" {
		return fmt.Errorf("DOCX reply requires an unambiguous top-level comment")
	}
	id, err := nextCommentID(root, comments)
	if err != nil {
		return err
	}
	paraID := p.nextReplyParagraphID(root, comments, extended, changes)
	author := "사용자"
	if actor == "ai" {
		author = "AI"
	}
	var xml strings.Builder
	xml.WriteString(`<w:comment xmlns:w="` + wordNS + `" xmlns:w14="` + word2010NS + `" xmlns:mc="` + commentMarkupCompatibilityNS +
		`" mc:Ignorable="w14" w:id="` + id + `" w:author="` + author + `" w:date="` + time.Now().UTC().Format(time.RFC3339) + `">`)
	lines := strings.Split(edit.Text, "\n")
	for index, line := range lines {
		opening := "<w:p>"
		if index == len(lines)-1 {
			opening = `<w:p w14:paraId="` + paraID + `">`
		}
		xml.WriteString(opening + "<w:r>" + inlineXML(line) + "</w:r></w:p>")
	}
	xml.WriteString("</w:comment>")
	newComments, err := appendXMLChild(commentsRaw, xml.String())
	if err != nil {
		return err
	}
	ext := `<w15:commentEx xmlns:w15="` + commentExtNS + `" w15:paraId="` + paraID +
		`" w15:paraIdParent="` + commentParagraphID(attribute(parent, commentExtNS, "paraId")) + `" w15:done="0"/>`
	newExtended, err := appendXMLChild(extended.source, ext)
	if err != nil {
		return err
	}
	changes[filename], changes[extName] = newComments, newExtended
	return nil
}

func (p *documentPackage) nextReplyParagraphID(root, comments, extended *node, changes map[string][]byte) string {
	used := map[string]bool{}
	var visit func(*node)
	visit = func(n *node) {
		if n.is("p") {
			if id := commentParagraphID(attribute(n, word2010NS, "paraId")); id != "" {
				used[id] = true
			}
		}
		for _, child := range n.children {
			visit(child)
		}
	}
	visit(root)
	visit(comments)
	for _, entry := range extended.children {
		for _, name := range []string{"paraId", "paraIdParent"} {
			if id := commentParagraphID(attribute(entry, commentExtNS, name)); id != "" {
				used[id] = true
			}
		}
	}
	// Other Word stories, including header and footer parts, can already
	// have a native paragraph ID imported from the same document.
	for _, part := range p.parts {
		name := part.header.Name
		if name == "word/document.xml" || !strings.HasPrefix(name, "word/") || !strings.HasSuffix(name, ".xml") {
			continue
		}
		raw := p.partData(name, changes)
		if !bytes.Contains(raw, []byte("paraId")) {
			continue
		}
		if other, err := parseXML(raw); err == nil {
			visit(other)
		}
	}
	for next := uint32(1); ; next++ {
		id := fmt.Sprintf("%08X", next)
		if !used[id] {
			return id
		}
	}
}
