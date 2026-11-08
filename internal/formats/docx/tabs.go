package docx

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type TabStop struct {
	Position int    `json:"position" jsonschema:"Tab position in twips, from 0 to 28800"`
	Align    string `json:"align" jsonschema:"left, center, right, decimal, bar, or clear"`
	Leader   string `json:"leader,omitempty" jsonschema:"none, dot, hyphen, underscore, heavy, or middleDot"`
}

func validateTabs(stops []TabStop) error {
	if len(stops) > 32 {
		return fmt.Errorf("DOCX tab update exceeds 32 positions")
	}
	seen := map[int]bool{}
	for _, tab := range stops {
		if tab.Position < 0 || tab.Position > 28800 || seen[tab.Position] ||
			!strings.Contains(" left center right decimal bar clear ", " "+tab.Align+" ") ||
			tab.Leader != "" && !strings.Contains(" none dot hyphen underscore heavy middleDot ", " "+tab.Leader+" ") ||
			tab.Align == "clear" && tab.Leader != "" && tab.Leader != "none" {
			return fmt.Errorf("invalid or duplicate DOCX tab stop")
		}
		seen[tab.Position] = true
	}
	return nil
}

// Updates are keyed by position. A clear entry remains explicit so inherited
// style tab stops do not reappear after a direct stop is removed.
func patchTabs(n *node, data []byte, stops []TabStop) (string, error) {
	if err := validateTabs(stops); err != nil {
		return "", err
	}
	type chunk struct {
		position int
		raw      string
	}
	chunks := []chunk{}
	updates := map[int]TabStop{}
	for _, tab := range stops {
		updates[tab.Position] = tab
	}
	seen := map[int]bool{}
	open, close := `<dwt:tabs xmlns:dwt="`+wordNS+`">`, `</dwt:tabs>`
	if n != nil {
		if strings.TrimSpace(n.text.String()) != "" {
			return "", fmt.Errorf("opaque DOCX tab stops")
		}
		open, close = scopedOpening(n, data, false)
		for _, child := range n.children {
			if !child.is("tab") {
				return "", fmt.Errorf("unsupported DOCX tab stop properties")
			}
			position := listInt(child.attr("pos"), -1)
			if position < 0 || seen[position] {
				return "", fmt.Errorf("invalid or duplicate original DOCX tab position")
			}
			seen[position] = true
			if tab, changed := updates[position]; changed {
				values := map[string]string{"pos": strconv.Itoa(position), "val": tab.Align}
				if tab.Leader != "" {
					values["leader"] = tab.Leader
				}
				if tab.Align == "clear" {
					values["leader"] = "none"
				}
				raw, err := patchProperty(child, data, "tab", values, nil)
				if err != nil {
					return "", err
				}
				chunks = append(chunks, chunk{position, raw})
				delete(updates, position)
			} else {
				chunks = append(chunks, chunk{position, string(data[child.start:child.end])})
			}
		}
	}
	for position, tab := range updates {
		values := map[string]string{"pos": strconv.Itoa(position), "val": tab.Align}
		if tab.Leader != "" {
			values["leader"] = tab.Leader
		}
		raw, err := patchProperty(nil, data, "tab", values, nil)
		if err != nil {
			return "", err
		}
		chunks = append(chunks, chunk{position, raw})
	}
	sort.SliceStable(chunks, func(i, j int) bool { return chunks[i].position < chunks[j].position })
	var out strings.Builder
	out.WriteString(open)
	for _, chunk := range chunks {
		out.WriteString(chunk.raw)
	}
	out.WriteString(close)
	return out.String(), nil
}
