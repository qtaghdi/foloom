package docx

import (
	"fmt"
	"strconv"
)

// Copy the definition and the instance. Some editors combine counters of
// instances sharing a definition, even when the instances have distinct IDs.
func copyListWithStart(root *node, data []byte, restart ListDefinition, id, abstractID int) (string, string, error) {
	var source *node
	for _, child := range root.children {
		if child.is("num") && listInt(child.attr("numId"), -1) == restart.SourceID {
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
	var override *node
	suffixStart := abstractReference.end
	insertAt := suffixStart
	for _, child := range source.children {
		if !child.is("lvlOverride") {
			continue
		}
		level := listInt(child.attr("ilvl"), -1)
		if level == restart.Level {
			if override != nil {
				return "", "", fmt.Errorf("ambiguous DOCX level override")
			}
			override = child
		}
		if level >= 0 && level < restart.Level {
			insertAt = max(insertAt, child.end)
		}
	}
	if override != nil && override.start < suffixStart ||
		countChild(override, "startOverride") > 1 || countChild(override, "lvl") > 1 {
		return "", "", fmt.Errorf("ambiguous DOCX level override properties")
	}
	old := override.child("startOverride")
	field, err := patchProperty(old, data, "startOverride", map[string]string{"val": strconv.Itoa(restart.Start)}, nil)
	if err != nil {
		return "", "", err
	}

	open, close := scopedOpening(source, data, false)
	open, err = patchOpeningAttributes(open, wordNS, map[string]string{"numId": strconv.Itoa(id)}, nil)
	if err != nil {
		return "", "", err
	}
	prefix := open + string(data[source.inner:abstractReference.start]) + reference
	if override == nil {
		field = `<dwf:lvlOverride xmlns:dwf="` + wordNS + `" dwf:ilvl="` + strconv.Itoa(restart.Level) + `">` + field + `</dwf:lvlOverride>`
		return copiedDefinition, prefix + string(data[suffixStart:insertAt]) + field + string(data[insertAt:source.endStart]) + close, nil
	}
	if old != nil {
		return copiedDefinition, prefix + string(data[suffixStart:old.start]) + field + string(data[old.end:source.endStart]) + close, nil
	}
	overrideOpen, overrideClose := scopedOpening(override, data, false)
	instance := prefix + string(data[suffixStart:override.start]) + overrideOpen + field + string(data[override.inner:override.endStart]) +
		overrideClose + string(data[override.end:source.endStart]) + close
	return copiedDefinition, instance, nil
}
