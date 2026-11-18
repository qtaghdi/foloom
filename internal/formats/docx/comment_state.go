package docx

import (
	"fmt"
	"path"
	"strconv"
	"strings"
)

const (
	commentExtNS      = "http://schemas.microsoft.com/office/word/2012/wordml"
	commentExtRelType = "http://schemas.microsoft.com/office/2011/relationships/commentsExtended"
)

func (p *documentPackage) commentExtensions(changes map[string][]byte) (string, *node, error) {
	data := p.partData(documentRelationships, changes)
	if len(data) == 0 {
		return "", nil, nil
	}
	rels, err := parseXML(data)
	if err != nil || rels.name.Space != packageRelNS || rels.name.Local != "Relationships" {
		return "", nil, fmt.Errorf("invalid DOCX document relationships")
	}
	filename := ""
	for _, rel := range rels.children {
		if rel.name.Space != packageRelNS || rel.name.Local != "Relationship" ||
			attribute(rel, "", "Type") != commentExtRelType {
			continue
		}
		mode, target := attribute(rel, "", "TargetMode"), attribute(rel, "", "Target")
		if filename != "" || target == "" || strings.ContainsAny(target, "\\?#:") ||
			mode != "" && mode != "Internal" {
			return "", nil, fmt.Errorf("ambiguous DOCX extended comments relationship")
		}
		if strings.HasPrefix(target, "/") {
			filename = strings.TrimPrefix(path.Clean(target), "/")
		} else {
			filename = path.Clean(path.Join("word", target))
		}
	}
	if filename == "" {
		return "", nil, nil
	}
	if !strings.HasPrefix(filename, "word/") {
		return "", nil, fmt.Errorf("invalid DOCX extended comments target")
	}
	root, err := parseXML(p.partData(filename, changes))
	if err != nil || root.name.Space != commentExtNS || root.name.Local != "commentsEx" {
		return "", nil, fmt.Errorf("invalid DOCX extended comments part")
	}
	return filename, root, nil
}

func commentParagraphID(raw string) string {
	n, err := strconv.ParseUint(raw, 16, 32)
	if err != nil || len(raw) != 8 || n == 0 || n >= 0x80000000 {
		return ""
	}
	return strings.ToUpper(raw)
}

func commentDone(n *node) (bool, bool) {
	switch attribute(n, commentExtNS, "done") {
	case "", "0", "false", "off":
		return false, true
	case "1", "true", "on":
		return true, true
	default:
		return false, false
	}
}

func commentResolutionNodes(comments, extended *node) map[string]*node {
	byParagraph := map[string]*node{}
	seen := map[string]bool{}
	for _, entry := range extended.children {
		if entry.name.Space != commentExtNS || entry.name.Local != "commentEx" {
			continue
		}
		id := commentParagraphID(attribute(entry, commentExtNS, "paraId"))
		if id == "" {
			continue
		}
		_, known := commentDone(entry)
		if seen[id] || !known || attribute(entry, commentExtNS, "paraIdParent") != "" ||
			len(entry.children) != 0 || strings.TrimSpace(entry.text.String()) != "" {
			delete(byParagraph, id)
		} else {
			byParagraph[id] = entry
		}
		seen[id] = true
	}
	paragraphs := map[string]int{}
	ids := map[int64]int{}
	idToParagraph := map[string]string{}
	for _, comment := range comments.children {
		raw := comment.attr("id")
		id, err := strconv.ParseInt(raw, 10, 32)
		if !comment.is("comment") || err != nil || id < 0 {
			continue
		}
		ids[id]++
		if len(comment.children) == 0 {
			continue
		}
		last := comment.children[len(comment.children)-1]
		if !last.is("p") {
			continue
		}
		key := commentParagraphID(attribute(last, word2010NS, "paraId"))
		if key != "" {
			paragraphs[key]++
			idToParagraph[raw] = key
		}
	}
	result := map[string]*node{}
	for raw, key := range idToParagraph {
		id, _ := strconv.ParseInt(raw, 10, 32)
		if ids[id] == 1 && paragraphs[key] == 1 && byParagraph[key] != nil {
			result[raw] = byParagraph[key]
		}
	}
	return result
}

func (p *documentPackage) setCommentDone(id string, done bool, changes map[string][]byte) error {
	_, _, comments, err := p.commentsPart(changes)
	if err != nil || comments == nil {
		return fmt.Errorf("DOCX comments part unavailable")
	}
	filename, extended, err := p.commentExtensions(changes)
	if err != nil || extended == nil {
		return fmt.Errorf("DOCX extended comments part unavailable")
	}
	entry := commentResolutionNodes(comments, extended)[id]
	if entry == nil || strings.Contains(string(extended.source[entry.start:entry.end]), "<!--") {
		return fmt.Errorf("protected or ambiguous DOCX comment state")
	}
	current, _ := commentDone(entry)
	if current == done {
		return nil
	}
	value := "0"
	if done {
		value = "1"
	}
	open, close := scopedOpening(entry, extended.source, false)
	open, err = patchOpeningAttributes(open, commentExtNS, map[string]string{"done": value}, nil)
	if err != nil {
		return err
	}
	raw := append(append([]byte(nil), extended.source[:entry.start]...), open+close...)
	changes[filename] = append(raw, extended.source[entry.end:]...)
	return nil
}
