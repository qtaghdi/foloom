package xlsx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"regexp"
	"sort"
	"strings"
)

var validationDisplayAttributeNames = []string{
	"showDropDown",
	"showInputMessage",
	"showErrorMessage",
}

var validationDisplayAttributePatterns = map[string]*regexp.Regexp{
	"showDropDown":     regexp.MustCompile(`\s+showDropDown\s*=\s*(?:"[^"]*"|'[^']*')`),
	"showInputMessage": regexp.MustCompile(`\s+showInputMessage\s*=\s*(?:"[^"]*"|'[^']*')`),
	"showErrorMessage": regexp.MustCompile(`\s+showErrorMessage\s*=\s*(?:"[^"]*"|'[^']*')`),
}

type validationDisplayAttributes map[string]string

type validationDisplayState map[string][]validationXMLNode

type validationXMLNode struct {
	identity    string
	start       int
	startTagEnd int
	display     validationDisplayAttributes
}

type validationXMLFields struct {
	formula1 string
	formula2 string
	sqref    string
}

func captureValidationDisplayState(content []byte) (validationDisplayState, error) {
	worksheets, err := worksheetParts(content)
	if err != nil {
		return nil, err
	}
	state := make(validationDisplayState)
	for name, worksheet := range worksheets {
		nodes, parseErr := validationNodes(worksheet)
		if parseErr != nil {
			return nil, fmt.Errorf("inspect validation XML in %s: %w", name, parseErr)
		}
		hasExplicitDisplay := false
		for _, node := range nodes {
			hasExplicitDisplay = hasExplicitDisplay || len(node.display) > 0
		}
		if hasExplicitDisplay {
			state[name] = nodes
		}
	}
	return state, nil
}

// preserveValidationDisplayAttributes restores the explicit presence and value
// of display attributes that Excelize omits when their decoded value is false.
// Nodes are matched by validation semantics before any worksheet XML is changed.
func preserveValidationDisplayAttributes(state validationDisplayState, candidate []byte) ([]byte, error) {
	if len(candidate) > maxWorkbookBytes {
		return nil, fmt.Errorf("serialized workbook exceeds %d bytes", maxWorkbookBytes)
	}
	if len(state) == 0 {
		return candidate, nil
	}
	candidateWorksheets, err := selectedWorksheetParts(candidate, state)
	if err != nil {
		return nil, fmt.Errorf("inspect candidate validation XML: %w", err)
	}
	replacements := make(map[string][]byte)
	for name, sourceNodes := range state {
		candidateXML, ok := candidateWorksheets[name]
		if !ok {
			return nil, fmt.Errorf("preserve validation display attributes: candidate worksheet %s is missing", name)
		}
		restored, changed, restoreErr := restoreWorksheetValidationAttributes(sourceNodes, candidateXML)
		if restoreErr != nil {
			return nil, fmt.Errorf("preserve validation display attributes in %s: %w", name, restoreErr)
		}
		if changed {
			replacements[name] = restored
		}
	}
	if len(replacements) == 0 {
		return candidate, nil
	}
	restored, err := replaceZIPParts(candidate, replacements)
	if err != nil {
		return nil, fmt.Errorf("write preserved validation XML: %w", err)
	}
	return restored, nil
}

func worksheetParts(content []byte) (map[string][]byte, error) {
	return selectedWorksheetParts(content, nil)
}

func selectedWorksheetParts(content []byte, wanted validationDisplayState) (map[string][]byte, error) {
	archive, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return nil, err
	}
	result := make(map[string][]byte)
	for _, entry := range archive.File {
		name := path.Clean(entry.Name)
		if path.Dir(name) != "xl/worksheets" || path.Ext(name) != ".xml" {
			continue
		}
		if wanted != nil {
			if _, ok := wanted[name]; !ok {
				continue
			}
		}
		if entry.UncompressedSize64 > maxWorkbookPartBytes {
			return nil, fmt.Errorf("worksheet part %s exceeds %d bytes", name, maxWorkbookPartBytes)
		}
		reader, openErr := entry.Open()
		if openErr != nil {
			return nil, openErr
		}
		part, readErr := io.ReadAll(io.LimitReader(reader, int64(maxWorkbookPartBytes)+1))
		closeErr := reader.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if len(part) > maxWorkbookPartBytes {
			return nil, fmt.Errorf("worksheet part %s exceeds %d bytes", name, maxWorkbookPartBytes)
		}
		result[name] = part
	}
	return result, nil
}

func validationNodes(content []byte) ([]validationXMLNode, error) {
	decoder := xml.NewDecoder(bytes.NewReader(content))
	result := make([]validationXMLNode, 0)
	for {
		start := decoder.InputOffset()
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		element, ok := token.(xml.StartElement)
		if !ok || element.Name.Local != "dataValidation" {
			continue
		}
		node, parseErr := readValidationNode(decoder, element, int(start), int(decoder.InputOffset()))
		if parseErr != nil {
			return nil, parseErr
		}
		result = append(result, node)
	}
	return result, nil
}

func readValidationNode(decoder *xml.Decoder, element xml.StartElement, start, startTagEnd int) (validationXMLNode, error) {
	display := make(validationDisplayAttributes)
	attributes := make(map[string]string)
	for _, attribute := range element.Attr {
		if attribute.Name.Space == "" && isValidationDisplayAttribute(attribute.Name.Local) {
			if _, err := parseXMLBoolean(attribute.Value); err != nil {
				return validationXMLNode{}, fmt.Errorf("invalid %s value %q", attribute.Name.Local, attribute.Value)
			}
			display[attribute.Name.Local] = attribute.Value
			continue
		}
		if attribute.Name.Space == "" {
			attributes[attribute.Name.Local] = attribute.Value
		}
	}
	fields := validationXMLFields{}
	depth := 1
	capture := ""
	captureDepth := 0
	var captured strings.Builder
	for depth > 0 {
		token, err := decoder.Token()
		if err != nil {
			return validationXMLNode{}, err
		}
		switch typed := token.(type) {
		case xml.StartElement:
			depth++
			if capture == "" && (typed.Name.Local == "formula1" || typed.Name.Local == "formula2" || typed.Name.Local == "sqref") {
				capture = typed.Name.Local
				captureDepth = depth
				captured.Reset()
			}
		case xml.CharData:
			if capture != "" {
				captured.Write([]byte(typed))
			}
		case xml.EndElement:
			if capture != "" && depth == captureDepth && typed.Name.Local == capture {
				switch capture {
				case "formula1":
					fields.formula1 = captured.String()
				case "formula2":
					fields.formula2 = captured.String()
				case "sqref":
					fields.sqref = captured.String()
				}
				capture = ""
			}
			depth--
		}
	}
	return validationXMLNode{
		identity:    validationIdentity(element.Name, attributes, fields),
		start:       start,
		startTagEnd: startTagEnd,
		display:     display,
	}, nil
}

func validationIdentity(name xml.Name, attributes map[string]string, fields validationXMLFields) string {
	allowBlank := "false"
	if value, ok := attributes["allowBlank"]; ok {
		parsed, err := parseXMLBoolean(value)
		if err != nil {
			allowBlank = "invalid:" + value
		} else {
			allowBlank = fmt.Sprintf("%t", parsed)
		}
	}
	parts := []string{
		name.Space,
		name.Local,
		attributes["sqref"],
		fields.sqref,
		attributes["type"],
		attributes["operator"],
		allowBlank,
		attributes["errorStyle"],
		attributes["error"],
		attributes["errorTitle"],
		attributes["prompt"],
		attributes["promptTitle"],
		fields.formula1,
		fields.formula2,
	}
	return strings.Join(parts, "\x00")
}

func restoreWorksheetValidationAttributes(sourceNodes []validationXMLNode, candidateXML []byte) ([]byte, bool, error) {
	candidateNodes, err := validationNodes(candidateXML)
	if err != nil {
		return nil, false, err
	}
	sourceByIdentity := make(map[string][]validationDisplayAttributes)
	for _, node := range sourceNodes {
		sourceByIdentity[node.identity] = append(sourceByIdentity[node.identity], node.display)
	}
	matched := make(map[string]int)
	type patch struct {
		start int
		end   int
		data  []byte
	}
	patches := make([]patch, 0)
	for _, node := range candidateNodes {
		index := matched[node.identity]
		sources := sourceByIdentity[node.identity]
		if index >= len(sources) {
			return nil, false, fmt.Errorf("candidate validation does not match a source rule")
		}
		display := sources[index]
		matched[node.identity] = index + 1
		if equalValidationDisplayAttributes(display, node.display) {
			continue
		}
		startTag, replaceErr := replaceValidationStartTag(candidateXML[node.start:node.startTagEnd], display)
		if replaceErr != nil {
			return nil, false, replaceErr
		}
		patches = append(patches, patch{start: node.start, end: node.startTagEnd, data: startTag})
	}
	for identity, source := range sourceByIdentity {
		if matched[identity] != len(source) {
			return nil, false, fmt.Errorf("source validation does not match a candidate rule")
		}
	}
	if len(patches) == 0 {
		return candidateXML, false, nil
	}
	sort.Slice(patches, func(i, j int) bool { return patches[i].start > patches[j].start })
	result := append([]byte(nil), candidateXML...)
	for _, item := range patches {
		updated := make([]byte, 0, len(result)-(item.end-item.start)+len(item.data))
		updated = append(updated, result[:item.start]...)
		updated = append(updated, item.data...)
		updated = append(updated, result[item.end:]...)
		result = updated
	}
	return result, true, nil
}

func replaceValidationStartTag(startTag []byte, display validationDisplayAttributes) ([]byte, error) {
	result := append([]byte(nil), startTag...)
	for _, name := range validationDisplayAttributeNames {
		result = validationDisplayAttributePatterns[name].ReplaceAll(result, nil)
	}
	closing := bytes.LastIndexByte(result, '>')
	if closing < 0 {
		return nil, fmt.Errorf("validation start tag is incomplete")
	}
	insertAt := closing
	if insertAt > 0 && result[insertAt-1] == '/' {
		insertAt--
	}
	var attributes bytes.Buffer
	for _, name := range validationDisplayAttributeNames {
		value, ok := display[name]
		if !ok {
			continue
		}
		attributes.WriteByte(' ')
		attributes.WriteString(name)
		attributes.WriteString(`="`)
		if err := xml.EscapeText(&attributes, []byte(value)); err != nil {
			return nil, err
		}
		attributes.WriteByte('"')
	}
	updated := make([]byte, 0, len(result)+attributes.Len())
	updated = append(updated, result[:insertAt]...)
	updated = append(updated, attributes.Bytes()...)
	updated = append(updated, result[insertAt:]...)
	return updated, nil
}

func equalValidationDisplayAttributes(left, right validationDisplayAttributes) bool {
	if len(left) != len(right) {
		return false
	}
	for name, leftValue := range left {
		rightValue, ok := right[name]
		if !ok {
			return false
		}
		if leftValue != rightValue {
			return false
		}
	}
	return true
}

func isValidationDisplayAttribute(name string) bool {
	for _, candidate := range validationDisplayAttributeNames {
		if name == candidate {
			return true
		}
	}
	return false
}

func parseXMLBoolean(value string) (bool, error) {
	switch value {
	case "1", "true":
		return true, nil
	case "0", "false":
		return false, nil
	default:
		return false, fmt.Errorf("invalid XML boolean %q", value)
	}
}

func replaceZIPParts(content []byte, replacements map[string][]byte) ([]byte, error) {
	archive, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return nil, err
	}
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	if err := writer.SetComment(archive.Comment); err != nil {
		return nil, err
	}
	for _, entry := range archive.File {
		name := path.Clean(entry.Name)
		replacement, replace := replacements[name]
		if !replace {
			if err := writer.Copy(entry); err != nil {
				_ = writer.Close()
				return nil, err
			}
			continue
		}
		header := entry.FileHeader
		header.CRC32 = 0
		header.CompressedSize = 0
		header.CompressedSize64 = 0
		header.UncompressedSize = 0
		header.UncompressedSize64 = 0
		part, createErr := writer.CreateHeader(&header)
		if createErr != nil {
			_ = writer.Close()
			return nil, createErr
		}
		if _, writeErr := part.Write(replacement); writeErr != nil {
			_ = writer.Close()
			return nil, writeErr
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}
