package docx

import (
	"fmt"
	"strings"
	"time"
)

const (
	dateFieldInstruction = `DATE \@ "yyyy-MM-dd"`
	dateFieldLayout      = "2006-01-02"
)

func supportedDateInstruction(field *node) bool {
	if !field.is("fldSimple") {
		return false
	}
	parts := strings.Fields(field.attr("instr"))
	return len(parts) == 3 && strings.EqualFold(parts[0], "DATE") && parts[1] == `\@` && parts[2] == `"yyyy-MM-dd"`
}

func dateFieldXML(now time.Time) string {
	return `<w:fldSimple xmlns:w="` + wordNS + `" w:instr="` + escapeText(dateFieldInstruction) + `"><w:r><w:t>` +
		now.UTC().Format(dateFieldLayout) + `</w:t></w:r></w:fldSimple>`
}

func dateFieldCache(field *node, data []byte) *node {
	if !supportedDateInstruction(field) || field.parent == nil || !field.parent.is("p") ||
		field.parent.parent == nil || !field.parent.parent.is("body") || len(field.children) != 1 {
		return nil
	}
	for _, name := range []string{"fldChar", "instrText", "delInstrText", "ins", "del", "moveFrom", "moveTo", "pPrChange", "rPrChange"} {
		if field.parent.has(name) {
			return nil
		}
	}
	for _, attr := range field.attrs {
		if !isNamespaceAttribute(attr) && (attr.Name.Space != wordNS || attr.Name.Local != "instr" && attr.Name.Local != "dirty") {
			return nil
		}
	}
	run := field.children[0]
	cache := run.child("t")
	if !run.is("r") || cache == nil || len(cache.children) != 0 || !plainTextAttributes(cache) ||
		len(run.children) != 1 && !(len(run.children) == 2 && run.children[0].is("rPr")) {
		return nil
	}
	// Replace just the canonical numeric text so container formatting and XML
	// comments outside the date are preserved.
	value := cache.text.String()
	if len(value) != len(dateFieldLayout) || string(data[cache.inner:cache.endStart]) != value {
		return nil
	}
	if _, err := time.Parse(dateFieldLayout, value); err != nil {
		return nil
	}
	return cache
}

func refreshDateField(data []byte, target target, now time.Time) ([]replacement, error) {
	if target.kind != "date_field" {
		return nil, fmt.Errorf("protected DOCX date field")
	}
	cache := dateFieldCache(target.node, data)
	if cache == nil {
		return nil, fmt.Errorf("protected DOCX date field")
	}
	value := now.UTC().Format(dateFieldLayout)
	if value == cache.text.String() {
		return nil, nil
	}
	return []replacement{{start: cache.inner, end: cache.endStart, data: []byte(value)}}, nil
}
