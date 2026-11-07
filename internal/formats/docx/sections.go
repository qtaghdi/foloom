package docx

import (
	"fmt"
	"strconv"
	"strings"
)

type SectionFormat struct {
	Width       *int   `json:"width,omitempty"`
	Height      *int   `json:"height,omitempty"`
	Orientation string `json:"orientation,omitempty"`
	Top         *int   `json:"top,omitempty"`
	Right       *int   `json:"right,omitempty"`
	Bottom      *int   `json:"bottom,omitempty"`
	Left        *int   `json:"left,omitempty"`
	Columns     *int   `json:"columns,omitempty"`
	ColumnSpace *int   `json:"columnSpace,omitempty"`
	BreakType   string `json:"breakType,omitempty"`
}
type SectionInfo struct {
	ID          string     `json:"id"`
	StartBlock  int        `json:"startBlock"`
	EndBlock    int        `json:"endBlock"`
	Page        PageLayout `json:"page"`
	Orientation string     `json:"orientation"`
	Columns     int        `json:"columns"`
	ColumnSpace int        `json:"columnSpace"`
	BreakType   string     `json:"breakType"`
	Editable    bool       `json:"editable"`
}
type sectionTarget struct {
	info  SectionInfo
	props *node
	body  *node
}

func sectionPage(props *node) PageLayout {
	page := PageLayout{Width: 11906, Height: 16838, Top: 1440, Right: 1440, Bottom: 1440, Left: 1440}
	for _, v := range []struct {
		node     *node
		key      string
		value    *int
		min, max int
	}{{props.child("pgSz"), "w", &page.Width, 2000, 50000}, {props.child("pgSz"), "h", &page.Height, 2000, 50000}, {props.child("pgMar"), "top", &page.Top, 0, 50000}, {props.child("pgMar"), "right", &page.Right, 0, 50000}, {props.child("pgMar"), "bottom", &page.Bottom, 0, 50000}, {props.child("pgMar"), "left", &page.Left, 0, 50000}} {
		if n := previewNumber(v.node.attr(v.key), v.min, v.max); n != nil {
			*v.value = *n
		}
	}
	return page
}

// Section properties belong to the section ending at their paragraph. The final
// section uses body/sectPr, including when it has no body blocks of its own.
func inspectSections(root *node, session string, revision uint64) []sectionTarget {
	body := root.child("body")
	total := 1
	for _, block := range body.children {
		if block.is("p") && block.child("pPr").child("sectPr") != nil {
			total++
		}
	}
	out := []sectionTarget{}
	start, index := 0, 0
	appendSection := func(props *node, end int) {
		columns, space := 1, 720
		if n := previewNumber(props.child("cols").attr("num"), 1, 12); n != nil {
			columns = *n
		}
		if n := previewNumber(props.child("cols").attr("space"), 0, 14400); n != nil {
			space = *n
		}
		orientation := "portrait"
		if props.child("pgSz").attr("orient") == "landscape" {
			orientation = "landscape"
		}
		kind := props.child("type").attr("val")
		if kind == "" {
			kind = "nextPage"
		}
		editable := total <= 128
		if props != nil && sectionChildCount(props.parent, "sectPr") > 1 {
			editable = false
		}
		if props != nil {
			for _, child := range props.children {
				if child.is("sectPrChange") {
					editable = false
				}
			}
		}
		out = append(out, sectionTarget{SectionInfo{ID: fmt.Sprintf("section-%s-%d-%d", session, revision, len(out)), StartBlock: start, EndBlock: end, Page: sectionPage(props), Orientation: orientation, Columns: columns, ColumnSpace: space, BreakType: kind, Editable: editable}, props, body})
		start = end + 1
	}
	for _, block := range body.children {
		if !block.is("p") && !block.is("tbl") {
			continue
		}
		if len(out) < 127 {
			if props := block.child("pPr").child("sectPr"); props != nil {
				appendSection(props, index)
			}
		}
		index++
	}
	appendSection(body.child("sectPr"), index-1)
	return out
}

func sectionReplacement(data []byte, root *node, session string, revision uint64, op Operation) (replacement, error) {
	if op.Section == nil || op.TargetID != "" || op.EndTargetID != "" || op.Paragraphs != nil || op.Text != "" || op.ExpectedText != "" || op.StyleDefinition != nil {
		return replacement{}, fmt.Errorf("invalid DOCX section operation fields")
	}
	var target *sectionTarget
	sections := inspectSections(root, session, revision)
	for i := range sections {
		if sections[i].info.ID == op.SectionID {
			target = &sections[i]
			break
		}
	}
	if target == nil || !target.info.Editable {
		return replacement{}, fmt.Errorf("stale or protected DOCX section")
	}
	if sectionChildCount(root.child("body"), "sectPr") > 1 {
		return replacement{}, fmt.Errorf("duplicate final DOCX section")
	}
	f := op.Section
	if *f == (SectionFormat{}) {
		return replacement{}, fmt.Errorf("DOCX section update requires a property")
	}
	page := target.info.Page
	for _, pair := range []struct {
		value    *int
		dest     *int
		min, max int
	}{{f.Width, &page.Width, 2000, 50000}, {f.Height, &page.Height, 2000, 50000}, {f.Top, &page.Top, 0, 50000}, {f.Right, &page.Right, 0, 50000}, {f.Bottom, &page.Bottom, 0, 50000}, {f.Left, &page.Left, 0, 50000}} {
		if pair.value != nil {
			if *pair.value < pair.min || *pair.value > pair.max {
				return replacement{}, fmt.Errorf("DOCX section dimensions exceed bounds")
			}
			*pair.dest = *pair.value
		}
	}
	if page.Width-page.Left-page.Right < 1200 || page.Height-page.Top-page.Bottom < 1200 {
		return replacement{}, fmt.Errorf("DOCX section margins leave insufficient content space")
	}
	if f.Orientation != "" && f.Orientation != "portrait" && f.Orientation != "landscape" {
		return replacement{}, fmt.Errorf("invalid DOCX orientation")
	}
	if f.Columns != nil && (*f.Columns < 1 || *f.Columns > 12) || f.ColumnSpace != nil && (*f.ColumnSpace < 0 || *f.ColumnSpace > 14400) {
		return replacement{}, fmt.Errorf("invalid DOCX section columns")
	}
	if f.BreakType != "" && !strings.Contains(" nextPage continuous evenPage oddPage nextColumn ", " "+f.BreakType+" ") {
		return replacement{}, fmt.Errorf("invalid DOCX section break")
	}
	props := target.props
	updates := map[string]string{}
	set := func(name string, values map[string]string) error {
		if sectionChildCount(props, name) > 1 {
			return fmt.Errorf("duplicate DOCX section property")
		}
		raw, err := patchProperty(props.child(name), data, name, values, nil)
		if err == nil {
			updates[name] = raw
		}
		return err
	}
	size := map[string]string{}
	if f.Width != nil {
		size["w"] = strconv.Itoa(*f.Width)
	}
	if f.Height != nil {
		size["h"] = strconv.Itoa(*f.Height)
	}
	if f.Orientation != "" {
		size["orient"] = f.Orientation
	}
	if len(size) > 0 {
		if err := set("pgSz", size); err != nil {
			return replacement{}, err
		}
	}
	margins := map[string]string{}
	for _, pair := range []struct {
		name  string
		value *int
	}{{"top", f.Top}, {"right", f.Right}, {"bottom", f.Bottom}, {"left", f.Left}} {
		if pair.value != nil {
			margins[pair.name] = strconv.Itoa(*pair.value)
		}
	}
	if len(margins) > 0 {
		if err := set("pgMar", margins); err != nil {
			return replacement{}, err
		}
	}
	if f.Columns != nil || f.ColumnSpace != nil {
		if cols := props.child("cols"); cols != nil && len(cols.children) > 0 {
			return replacement{}, fmt.Errorf("custom unequal DOCX columns are protected")
		}
		count, space := target.info.Columns, target.info.ColumnSpace
		if f.Columns != nil {
			count = *f.Columns
		}
		if f.ColumnSpace != nil {
			space = *f.ColumnSpace
		}
		if (page.Width-page.Left-page.Right)-(count-1)*space < count*240 {
			return replacement{}, fmt.Errorf("DOCX column spacing leaves insufficient width")
		}
		if err := set("cols", map[string]string{"num": strconv.Itoa(count), "space": strconv.Itoa(space), "equalWidth": "1"}); err != nil {
			return replacement{}, err
		}
	}
	if f.BreakType != "" {
		if err := set("type", map[string]string{"val": f.BreakType}); err != nil {
			return replacement{}, err
		}
	}
	raw := patchSectionChildren(props, data, updates)
	if props == nil {
		return replacement{target.body.endStart, target.body.endStart, []byte(raw)}, nil
	}
	return replacement{props.start, props.end, []byte(raw)}, nil
}

func patchSectionChildren(props *node, data []byte, updates map[string]string) string {
	order := strings.Fields("headerReference footerReference footnotePr endnotePr type pgSz pgMar paperSrc pgBorders lnNumType pgNumType cols formProt vAlign noEndnote titlePg textDirection bidi rtlGutter docGrid printerSettings sectPrChange")
	rank := map[string]int{}
	for i, name := range order {
		rank[name] = i
	}
	open, close := `<dws:sectPr xmlns:dws="`+wordNS+`">`, `</dws:sectPr>`
	if props != nil {
		open, close = scopedOpening(props, data, false)
	}
	var out strings.Builder
	out.WriteString(open)
	emitBefore := func(limit int) {
		for _, name := range order {
			if rank[name] < limit {
				if raw, ok := updates[name]; ok {
					out.WriteString(raw)
					delete(updates, name)
				}
			}
		}
	}
	if props != nil {
		for _, child := range props.children {
			if child.name.Space == wordNS {
				emitBefore(rank[child.name.Local])
				if raw, ok := updates[child.name.Local]; ok {
					out.WriteString(raw)
					delete(updates, child.name.Local)
					continue
				}
			}
			out.Write(data[child.start:child.end])
		}
	}
	emitBefore(len(order))
	out.WriteString(close)
	return out.String()
}

func sectionChildCount(n *node, name string) int {
	if n == nil {
		return 0
	}
	count := 0
	for _, child := range n.children {
		if child.is(name) {
			count++
		}
	}
	return count
}
