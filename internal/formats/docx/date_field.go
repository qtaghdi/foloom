package docx

import (
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
