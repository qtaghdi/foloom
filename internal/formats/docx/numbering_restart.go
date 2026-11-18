package docx

import (
	"fmt"
	"strconv"
)

// Copy the definition and the instance. Some editors combine counters of
// instances sharing a definition, even when the instances have distinct IDs.
func copyListWithStart(root *node, data []byte, sourceID, id, abstractID, start int) (string, string, error) {
	var source *node
	for _, child := range root.children {
		if child.is("num") && listInt(child.attr("numId"), -1) == sourceID {
			if source != nil {
				return "", "", fmt.Errorf("ambiguous DOCX numbering instance")
			}
			source = child
		}
	}
	if source == nil || countChild(source, "abstractNumId") != 1 {
		return "", "", fmt.Errorf("invalid DOCX numbering instance")
	}
	abstractReference := source.child("abstractNumId")
	sourceAbstractID := listInt(abstractReference.attr("val"), -1)
	var definition *node
	for _, child := range root.children {
		if child.is("abstractNum") && listInt(child.attr("abstractNumId"), -1) == sourceAbstractID {
			if definition != nil {
				return "", "", fmt.Errorf("ambiguous DOCX abstract numbering")
			}
			definition = child
		}
	}
	if sourceAbstractID < 0 || definition == nil || definition.child("numStyleLink") != nil {
		return "", "", fmt.Errorf("unknown or indirectly linked DOCX abstract numbering")
	}
	idValue := strconv.Itoa(abstractID)
	definitionOpen, definitionClose := scopedOpening(definition, data, false)
	definitionOpen, err := patchOpeningAttributes(definitionOpen, wordNS, map[string]string{"abstractNumId": idValue}, nil)
	if err != nil {
		return "", "", err
	}
	copiedDefinition := definitionOpen + string(data[definition.inner:definition.endStart]) + definitionClose
	reference, err := patchProperty(abstractReference, data, "abstractNumId", map[string]string{"val": idValue}, nil)
	if err != nil {
		return "", "", err
	}
	var top *node
	for _, child := range source.children {
		if child.is("lvlOverride") && listInt(child.attr("ilvl"), -1) == 0 {
			if top != nil {
				return "", "", fmt.Errorf("ambiguous DOCX top-level override")
			}
			top = child
		}
	}
	if top != nil && top.start < abstractReference.end ||
		countChild(top, "startOverride") > 1 || countChild(top, "lvl") > 1 {
		return "", "", fmt.Errorf("ambiguous DOCX top-level override properties")
	}
	startOverride := top.child("startOverride")
	restart, err := patchProperty(startOverride, data, "startOverride", map[string]string{"val": strconv.Itoa(start)}, nil)
	if err != nil {
		return "", "", err
	}

	open, close := scopedOpening(source, data, false)
	open, err = patchOpeningAttributes(open, wordNS, map[string]string{"numId": strconv.Itoa(id)}, nil)
	if err != nil {
		return "", "", err
	}
	prefix := open + string(data[source.inner:abstractReference.start]) + reference
	suffixStart := abstractReference.end
	if top == nil {
		restart = `<dwf:lvlOverride xmlns:dwf="` + wordNS + `" dwf:ilvl="0">` + restart + `</dwf:lvlOverride>`
		return copiedDefinition, prefix + restart + string(data[suffixStart:source.endStart]) + close, nil
	}
	if startOverride != nil {
		return copiedDefinition, prefix + string(data[suffixStart:startOverride.start]) + restart + string(data[startOverride.end:source.endStart]) + close, nil
	}
	topOpen, topClose := scopedOpening(top, data, false)
	instance := prefix + string(data[suffixStart:top.start]) + topOpen + restart + string(data[top.inner:top.endStart]) +
		topClose + string(data[top.end:source.endStart]) + close
	return copiedDefinition, instance, nil
}
