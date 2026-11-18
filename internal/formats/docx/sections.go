package docx

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	defaultHeaderFooterDistance = 720
	maxHeaderFooterDistance     = 31680
)

type SectionFormat struct {
	// A start of -1 removes the explicit start so numbering continues.
	PageNumberStart  *int                 `json:"pageNumberStart,omitempty"`
	PageNumberFormat string               `json:"pageNumberFormat,omitempty"`
	Gutter           *int                 `json:"gutter,omitempty" jsonschema:"Fixed left binding space in twips, separate from the left page margin"`
	HeaderDistance   *int                 `json:"headerDistance,omitempty" jsonschema:"Distance from the top edge of the page to the header in twips"`
	FooterDistance   *int                 `json:"footerDistance,omitempty" jsonschema:"Distance from the bottom edge of the page to the footer in twips"`
	HeaderFooters    []HeaderFooterUpdate `json:"headerFooters,omitempty"`
	FirstPage        *bool                `json:"firstPage,omitempty"`
	Width            *int                 `json:"width,omitempty"`
	Height           *int                 `json:"height,omitempty"`
	Orientation      string               `json:"orientation,omitempty"`
	Top              *int                 `json:"top,omitempty"`
	Right            *int                 `json:"right,omitempty"`
	Bottom           *int                 `json:"bottom,omitempty"`
	Left             *int                 `json:"left,omitempty"`
	Columns          *int                 `json:"columns,omitempty"`
	ColumnSpace      *int                 `json:"columnSpace,omitempty"`
	BreakType        string               `json:"breakType,omitempty"`
}
type SectionInfo struct {
	PageNumberStart  *int               `json:"pageNumberStart,omitempty"`
	PageNumberFormat string             `json:"pageNumberFormat,omitempty"`
	Gutter           *int               `json:"gutter,omitempty"`
	HeaderDistance   *int               `json:"headerDistance,omitempty"`
	FooterDistance   *int               `json:"footerDistance,omitempty"`
	HeaderFooters    []HeaderFooterInfo `json:"headerFooters"`
	FirstPage        bool               `json:"firstPage"`
	ID               string             `json:"id"`
	StartBlock       int                `json:"startBlock"`
	EndBlock         int                `json:"endBlock"`
	Page             PageLayout         `json:"page"`
	Orientation      string             `json:"orientation"`
	Columns          int                `json:"columns"`
	ColumnSpace      int                `json:"columnSpace"`
	BreakType        string             `json:"breakType"`
	Editable         bool               `json:"editable"`
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
		margins := props.child("pgMar")
		out = append(out, sectionTarget{SectionInfo{
			ID:               fmt.Sprintf("section-%s-%d-%d", session, revision, len(out)),
			PageNumberStart:  previewNumber(attribute(props.child("pgNumType"), wordNS, "start"), 0, 2147483647),
			PageNumberFormat: attribute(props.child("pgNumType"), wordNS, "fmt"),
			HeaderDistance:   previewNumber(attribute(margins, wordNS, "header"), 0, maxHeaderFooterDistance),
			FooterDistance:   previewNumber(attribute(margins, wordNS, "footer"), 0, maxHeaderFooterDistance),
			StartBlock:       start,
			EndBlock:         end,
			Page:             sectionPage(props),
			Orientation:      orientation,
			Columns:          columns,
			ColumnSpace:      space,
			BreakType:        kind,
			Editable:         editable,
		}, props, body})
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

func (p *documentPackage) sectionReplacement(data []byte, root *node, session string, revision uint64, op Operation, changes map[string][]byte) (replacement, error) {
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
	if f.Width == nil && f.Height == nil && f.Top == nil && f.Right == nil && f.Bottom == nil && f.Left == nil &&
		f.Gutter == nil && f.HeaderDistance == nil && f.FooterDistance == nil && f.Columns == nil && f.ColumnSpace == nil &&
		f.Orientation == "" && f.BreakType == "" && f.FirstPage == nil && f.PageNumberStart == nil &&
		f.PageNumberFormat == "" && len(f.HeaderFooters) == 0 {
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
	props := target.props
	bodyWidth := page.Width - page.Left - page.Right
	if f.Gutter != nil || f.Width != nil || f.Left != nil || f.Right != nil || f.Columns != nil || f.ColumnSpace != nil {
		_, _, settings, err := p.pageSettings(changes)
		gutter := sectionGutter(props, err == nil && fixedLeftGutters(settings))
		if f.Gutter != nil {
			if gutter == nil || *f.Gutter < 0 || *f.Gutter > maxSectionGutter {
				return replacement{}, fmt.Errorf("invalid or protected DOCX binding gutter")
			}
			gutter = f.Gutter
		}
		if gutter != nil {
			bodyWidth -= *gutter
		}
	}
	if bodyWidth < 1200 || page.Height-page.Top-page.Bottom < 1200 {
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
	for _, distance := range []*int{f.HeaderDistance, f.FooterDistance} {
		if distance != nil && (*distance < 0 || *distance > maxHeaderFooterDistance) {
			return replacement{}, fmt.Errorf("DOCX header or footer distance exceeds bounds")
		}
	}
	margins := map[string]string{}
	for _, pair := range []struct {
		name  string
		value *int
	}{{"top", f.Top}, {"right", f.Right}, {"bottom", f.Bottom}, {"left", f.Left}, {"gutter", f.Gutter}, {"header", f.HeaderDistance}, {"footer", f.FooterDistance}} {
		if pair.value != nil {
			margins[pair.name] = strconv.Itoa(*pair.value)
		}
	}
	if len(margins) > 0 {
		if props.child("pgMar") == nil {
			// A newly authored pgMar must contain all seven required attributes.
			for name, fallback := range map[string]int{
				"top": page.Top, "right": page.Right, "bottom": page.Bottom, "left": page.Left,
				"header": defaultHeaderFooterDistance, "footer": defaultHeaderFooterDistance, "gutter": 0,
			} {
				if _, changed := margins[name]; !changed {
					margins[name] = strconv.Itoa(fallback)
				}
			}
		}
		if err := set("pgMar", margins); err != nil {
			return replacement{}, err
		}
	}
	if f.Columns != nil || f.ColumnSpace != nil || f.Gutter != nil {
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
		if bodyWidth-(count-1)*space < count*240 {
			return replacement{}, fmt.Errorf("DOCX column spacing leaves insufficient width")
		}
		if f.Columns != nil || f.ColumnSpace != nil {
			if err := set("cols", map[string]string{"num": strconv.Itoa(count), "space": strconv.Itoa(space), "equalWidth": "1"}); err != nil {
				return replacement{}, err
			}
		}
	}
	if f.BreakType != "" {
		if err := set("type", map[string]string{"val": f.BreakType}); err != nil {
			return replacement{}, err
		}
	}
	if f.FirstPage != nil {
		value := "0"
		if *f.FirstPage {
			value = "1"
		}
		if err := set("titlePg", map[string]string{"val": value}); err != nil {
			return replacement{}, err
		}
	}
	if f.PageNumberStart != nil || f.PageNumberFormat != "" {
		if sectionChildCount(props, "pgNumType") > 1 {
			return replacement{}, fmt.Errorf("duplicate DOCX page numbering property")
		}
		values := map[string]string{}
		var remove []string
		if f.PageNumberStart != nil {
			switch n := *f.PageNumberStart; {
			case n == -1:
				remove = []string{"start"}
			case n >= 0 && n <= 2147483647:
				values["start"] = strconv.Itoa(n)
			default:
				return replacement{}, fmt.Errorf("invalid DOCX page number start")
			}
		}
		switch f.PageNumberFormat {
		case "":
		case "decimal", "upperRoman", "lowerRoman", "upperLetter", "lowerLetter":
			values["fmt"] = f.PageNumberFormat
		default:
			return replacement{}, fmt.Errorf("invalid DOCX page number format")
		}
		if old := props.child("pgNumType"); old != nil || len(values) > 0 {
			raw, err := patchProperty(old, data, "pgNumType", values, remove)
			if err != nil {
				return replacement{}, err
			}
			updates["pgNumType"] = raw
		}
	}
	raw := patchSectionChildren(props, data, updates)
	if len(f.HeaderFooters) > 0 {
		index := 0
		for i := range sections {
			if sections[i].info.ID == op.SectionID {
				index = i
				break
			}
		}
		headers := p.headerFooterCatalog(sections)[index]
		for _, update := range f.HeaderFooters {
			if !update.LinkPrevious {
				for _, info := range headers {
					if info.Kind == update.Kind && info.Variant == update.Variant && !info.Editable {
						return replacement{}, fmt.Errorf("protected inherited DOCX header/footer cannot be flattened")
					}
				}
			}
		}
		var err error
		raw, err = p.patchHeaderFooters(raw, f.HeaderFooters, index, changes)
		if err != nil {
			return replacement{}, err
		}
	}
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
