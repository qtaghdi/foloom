package docx

import (
	"fmt"
	"strings"
)

func boundaryReplacements(root *node, data []byte, session string, revision uint64, p *node, mode string) ([]replacement, error) {
	body := root.child("body")
	if p == nil || p.parent != body || sectionChildCount(p, "pPr") > 1 {
		return nil, fmt.Errorf("DOCX boundary requires a top-level paragraph")
	}
	props := p.child("pPr")
	if props.child("pPrChange") != nil || p.has("ins") || p.has("del") || p.has("moveFrom") || p.has("moveTo") {
		return nil, fmt.Errorf("tracked DOCX paragraph boundaries are protected")
	}
	switch mode {
	case "remove_section":
		closing := props.child("sectPr")
		if closing == nil || sectionChildCount(body, "sectPr") > 1 ||
			strings.Contains(string(data[closing.start:closing.end]), "<!--") {
			return nil, fmt.Errorf("DOCX section boundary is unavailable")
		}
		sections := inspectSections(root, session, revision)
		for i, section := range sections[:len(sections)-1] {
			next := sections[i+1]
			if section.props == closing && section.info.Editable && next.info.Editable && next.info.StartBlock <= next.info.EndBlock {
				return []replacement{{start: closing.start, end: closing.end}}, nil
			}
		}
		return nil, fmt.Errorf("tracked, ambiguous or final DOCX section boundary is protected")
	case "no_page_after":
		trailing := trailingPageBreak(p)
		if trailing == nil || len(trailing.children) != 0 || strings.TrimSpace(trailing.text.String()) != "" ||
			strings.Contains(string(data[trailing.start:trailing.end]), "<!--") {
			return nil, fmt.Errorf("DOCX trailing page break is unavailable")
		}
		for _, attr := range trailing.attrs {
			if !isNamespaceAttribute(attr) && !(attr.Name.Space == wordNS && attr.Name.Local == "type") {
				return nil, fmt.Errorf("opaque DOCX page break is protected")
			}
		}
		return []replacement{{start: trailing.start, end: trailing.end}}, nil
	case "page_before", "no_page_before":
		if sectionChildCount(props, "pageBreakBefore") > 1 {
			return nil, fmt.Errorf("duplicate DOCX page break property")
		}
		value := "1"
		if mode == "no_page_before" {
			value = "0"
		}
		old := props.child("pageBreakBefore")
		raw, err := patchProperty(old, data, "pageBreakBefore", map[string]string{"val": value}, nil)
		if err != nil {
			return nil, err
		}
		if old != nil {
			return []replacement{{old.start, old.end, []byte(raw)}}, nil
		}
		if props == nil {
			return []replacement{newBoundaryProperties(p, data, raw)}, nil
		}
		if strings.HasSuffix(string(data[props.start:props.inner]), "/>") {
			open, close := scopedOpening(props, data, false)
			return []replacement{{props.start, props.end, []byte(open + raw + close)}}, nil
		}
		insert := props.inner
		for _, child := range props.children {
			if child.is("pStyle") || child.is("keepNext") || child.is("keepLines") {
				insert = child.end
			} else {
				break
			}
		}
		return []replacement{{insert, insert, []byte(raw)}}, nil
	case "section_next_page", "section_continuous":
	default:
		return nil, fmt.Errorf("invalid DOCX boundary mode")
	}
	sections := inspectSections(root, session, revision)
	if len(sections) >= 128 || sectionChildCount(body, "sectPr") > 1 || props.child("sectPr") != nil {
		return nil, fmt.Errorf("existing or ambiguous DOCX section boundary")
	}
	index := 0
	for _, block := range body.children {
		if block == p {
			break
		}
		if block.is("p") || block.is("tbl") {
			index++
		}
	}
	for _, section := range sections {
		if section.info.StartBlock > index || index > section.info.EndBlock {
			continue
		}
		if !section.info.Editable || index == section.info.EndBlock {
			return nil, fmt.Errorf("DOCX section split requires body content on both sides")
		}
		kind := "nextPage"
		if mode == "section_continuous" {
			kind = "continuous"
		}
		left, right, err := splitSectionProperties(section.props, data, kind)
		if err != nil {
			return nil, err
		}
		sectionEdit := replacement{body.endStart, body.endStart, []byte(right)}
		if section.props != nil {
			sectionEdit.start, sectionEdit.end = section.props.start, section.props.end
		}
		if props == nil {
			return []replacement{newBoundaryProperties(p, data, left), sectionEdit}, nil
		}
		if strings.HasSuffix(string(data[props.start:props.inner]), "/>") {
			open, close := scopedOpening(props, data, false)
			return []replacement{{props.start, props.end, []byte(open + left + close)}, sectionEdit}, nil
		}
		return []replacement{{props.endStart, props.endStart, []byte(left)}, sectionEdit}, nil
	}
	return nil, fmt.Errorf("DOCX paragraph section unavailable")
}

func newBoundaryProperties(p *node, data []byte, content string) replacement {
	props := `<dwb:pPr xmlns:dwb="` + wordNS + `">` + content + `</dwb:pPr>`
	if strings.HasSuffix(string(data[p.start:p.inner]), "/>") {
		open, close := scopedOpening(p, data, false)
		return replacement{p.start, p.end, []byte(open + props + close)}
	}
	return replacement{p.inner, p.inner, []byte(props)}
}

func splitSectionProperties(section *node, data []byte, kind string) (string, string, error) {
	left := patchSectionChildren(nil, data, map[string]string{})
	updates := map[string]string{}
	if section != nil {
		if sectionChildCount(section, "type") > 1 || sectionChildCount(section, "pgNumType") > 1 || strings.Contains(string(data[section.start:section.end]), "<!--") {
			return "", "", fmt.Errorf("ambiguous DOCX section properties")
		}
		// References remain shared. Editing either header later creates a private part.
		for _, child := range section.children {
			if child.name.Space != wordNS || !strings.Contains(" headerReference footerReference type pgSz pgMar pgNumType cols titlePg docGrid ", " "+child.name.Local+" ") {
				return "", "", fmt.Errorf("unsupported DOCX section property cannot be duplicated")
			}
		}
		open, close := scopedOpening(section, data, false)
		left = open + close
		if section.inner < section.endStart {
			left = open + string(data[section.inner:section.endStart]) + close
		}
		if old := section.child("pgNumType"); attribute(old, wordNS, "start") != "" {
			raw, err := patchProperty(old, data, "pgNumType", nil, []string{"start"})
			if err != nil {
				return "", "", err
			}
			updates["pgNumType"] = raw
		}
	}
	raw, err := patchProperty(section.child("type"), data, "type", map[string]string{"val": kind}, nil)
	if err != nil {
		return "", "", err
	}
	updates["type"] = raw
	return left, patchSectionChildren(section, data, updates), nil
}
