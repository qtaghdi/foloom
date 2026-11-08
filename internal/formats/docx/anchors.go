package docx

import (
	"bytes"
	"strconv"
	"strings"
)

const word2010NS = "http://schemas.microsoft.com/office/word/2010/wordml"

// Only an unambiguous native ID in the main story can survive structural edits.
func paragraphAnchors(root *node) map[*node]string {
	if !bytes.Contains(root.source, []byte("paraId")) {
		return nil
	}
	candidates := map[string]*node{}
	duplicate := map[string]bool{}
	var walk func(*node)
	walk = func(n *node) {
		if n.is("p") {
			id := attribute(n, word2010NS, "paraId")
			if value, err := strconv.ParseUint(id, 16, 31); len(id) == 8 && err == nil && value > 0 {
				id = strings.ToUpper(id)
				if candidates[id] != nil {
					duplicate[id] = true
				} else {
					candidates[id] = n
				}
			}
		}
		for _, child := range n.children {
			walk(child)
		}
	}
	walk(root)
	out := map[*node]string{}
	for id, n := range candidates {
		if !duplicate[id] {
			out[n] = id
		}
	}
	return out
}
