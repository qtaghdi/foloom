package docx

import "strings"

const commentExtContentType = "application/vnd.openxmlformats-officedocument.wordprocessingml.commentsExtended+xml"

func (p *documentPackage) extendNewComment(root, comments *node, id, text, actor string, changes map[string][]byte) (string, error) {
	// Word's newer durable comment formats require separate metadata.
	// Keep the existing writing path for those and for legacy comments.
	rels, err := parseXML(p.partData(documentRelationships, changes))
	if err != nil {
		return "", err
	}
	for _, rel := range rels.children {
		kind := attribute(rel, "", "Type")
		if strings.HasSuffix(kind, "/commentsIds") || strings.HasSuffix(kind, "/commentsExtensible") {
			return "", nil
		}
	}
	filename, extended, err := p.commentExtensions(changes)
	if err != nil {
		return "", nil
	}
	if extended == nil && comments != nil || extended != nil && comments == nil {
		return "", nil
	}
	if extended != nil && len(commentExtensionNodes(comments, extended)) != len(comments.children) {
		return "", nil
	}
	paraID := p.nextCommentParagraphID(root, comments, extended, changes)
	entry := `<w15:commentEx xmlns:w15="` + commentExtNS + `" w15:paraId="` + paraID + `" w15:done="0"/>`
	if extended == nil {
		filename = p.availablePart(changes, "word/commentsExtended", ".xml")
		if _, err := p.addRelatedPart(changes, filename, commentExtRelType, commentExtContentType, "rIdFoloomCommentsExtended"); err != nil {
			return "", err
		}
		changes[filename] = []byte(`<w15:commentsEx xmlns:w15="` + commentExtNS + `">` + entry + `</w15:commentsEx>`)
	} else {
		data, err := appendXMLChild(extended.source, entry)
		if err != nil {
			return "", err
		}
		changes[filename] = data
	}
	return threadedComment(id, paraID, text, actor), nil
}
