package docx

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"fmt"
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

// A paragraph plan may add paragraphs in independent body and table ranges.
type paragraphIDs map[string]bool

func usedParagraphIDs(root *node) paragraphIDs {
	used := paragraphIDs{}
	var visit func(*node)
	visit = func(n *node) {
		if id := attribute(n, word2010NS, "paraId"); id != "" {
			used[strings.ToUpper(id)] = true
		}
		for _, child := range n.children {
			visit(child)
		}
	}
	visit(root)
	return used
}

func (used paragraphIDs) reserve() (string, error) {
	// Random IDs avoid immediately recycling an ID after undo or deletion.
	for {
		var entropy [4]byte
		if _, err := rand.Read(entropy[:]); err != nil {
			return "", fmt.Errorf("generate DOCX paragraph ID: %w", err)
		}
		value := binary.BigEndian.Uint32(entropy[:]) & 0x7FFFFFFF
		id := fmt.Sprintf("%08X", value)
		if value != 0 && !used[id] {
			used[id] = true
			return id, nil
		}
	}
}

func newParagraphOpening(source *node, data []byte, id string) (string, string) {
	nextPrefix := func(prefix string) string {
		for namespaceForPrefix(source, prefix) != "" {
			prefix += "p"
		}
		return prefix
	}
	wordPrefix := nextPrefix("dw14")
	compatPrefix := nextPrefix("dwmc")
	opening, closing := scopedOpening(source, data, true)
	// Older readers must know they may ignore this Office extension.
	return strings.TrimSuffix(opening, ">") +
		` xmlns:` + wordPrefix + `="` + word2010NS +
		`" xmlns:` + compatPrefix + `="` + markupCompatNS +
		`" ` + compatPrefix + `:Ignorable="` + wordPrefix +
		`" ` + wordPrefix + `:paraId="` + id + `">`, closing
}
