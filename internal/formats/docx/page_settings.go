package docx

import (
	"fmt"
	"path"
	"strconv"
	"strings"
)

const documentRelationships = "word/_rels/document.xml.rels"
const packageRelNS = "http://schemas.openxmlformats.org/package/2006/relationships"
const evenAndOddHeaders = "evenAndOddHeaders"
const defaultTabStopProperty = "defaultTabStop"

// Read the relationship rather than assuming the settings part uses its conventional name.
func (p *documentPackage) pageSettings(changes map[string][]byte) (string, []byte, *node, error) {
	rels := p.partData(documentRelationships, changes)
	if len(rels) == 0 {
		return "", nil, nil, nil
	}
	root, err := parseXML(rels)
	if err != nil || root.name.Space != packageRelNS || root.name.Local != "Relationships" {
		return "", nil, nil, fmt.Errorf("invalid DOCX document relationships")
	}
	filename := ""
	for _, rel := range root.children {
		if rel.name.Space != packageRelNS || rel.name.Local != "Relationship" || attribute(rel, "", "Type") != officeRelNS+"/settings" {
			continue
		}
		mode, target := attribute(rel, "", "TargetMode"), attribute(rel, "", "Target")
		if filename != "" || mode != "" && mode != "Internal" || target == "" || strings.ContainsAny(target, "\\?#:") {
			return "", nil, nil, fmt.Errorf("ambiguous or unsupported DOCX settings relationship")
		}
		if strings.HasPrefix(target, "/") {
			filename = strings.TrimPrefix(path.Clean(target), "/")
		} else {
			filename = path.Clean(path.Join("word", target))
		}
		if filename == "" || filename == "." || strings.HasPrefix(filename, "../") {
			return "", nil, nil, fmt.Errorf("invalid DOCX settings target")
		}
	}
	if filename == "" {
		return "", nil, nil, nil
	}
	data := p.partData(filename, changes)
	settings, err := parseXML(data)
	if err != nil || !settings.is("settings") || sectionChildCount(settings, evenAndOddHeaders) > 1 {
		return "", nil, nil, fmt.Errorf("invalid DOCX settings part")
	}
	if setting := settings.child(evenAndOddHeaders); setting != nil {
		if len(setting.children) > 0 || strings.TrimSpace(setting.text.String()) != "" {
			return "", nil, nil, fmt.Errorf("opaque DOCX header parity setting")
		}
		switch attribute(setting, wordNS, "val") {
		case "", "1", "0", "true", "false", "on", "off":
		default:
			return "", nil, nil, fmt.Errorf("unknown DOCX header parity setting")
		}
	}
	return filename, data, settings, nil
}

func headerParity(settings *node) bool {
	setting := settings.child(evenAndOddHeaders)
	return setting != nil && onOff(attribute(setting, wordNS, "val"), true)
}

// The OOXML settings sequence places this option before the following children.
// The non-Word namespaces and Office extensions at the end of the sequence are
// handled separately so the insertion leaves all unrelated bytes untouched.
const settingsAfterHeaderParity = " bookFoldRevPrinting bookFoldPrinting bookFoldPrintingSheets drawingGridHorizontalSpacing drawingGridVerticalSpacing displayHorizontalDrawingGridEvery displayVerticalDrawingGridEvery doNotUseMarginsForDrawingGridOrigin drawingGridHorizontalOrigin drawingGridVerticalOrigin doNotShadeFormData noPunctuationKerning characterSpacingControl printTwoOnOne strictFirstAndLastChars noLineBreaksAfter noLineBreaksBefore savePreviewPicture doNotValidateAgainstSchema saveInvalidXml ignoreMixedContent alwaysShowPlaceholderText doNotDemarcateInvalidXml saveXmlDataOnly useXSLTWhenSaving saveThroughXslt showXMLTags alwaysMergeEmptyNamespace updateFields hdrShapeDefaults footnotePr endnotePr compat docVars rsids uiCompat97To2003 attachedSchema themeFontLang clrSchemeMapping doNotIncludeSubdocsInStats doNotAutoCompressPictures forceUpgrade captions readModeInkLockDown shapeDefaults decimalSymbol listSeparator "

func (p *documentPackage) updateHeaderParity(enabled bool, changes map[string][]byte) error {
	filename, data, root, err := p.pageSettings(changes)
	if err != nil {
		return err
	}
	old := root.child(evenAndOddHeaders)
	if enabled == (old != nil && onOff(attribute(old, wordNS, "val"), true)) {
		return nil
	}
	val := "0"
	if enabled {
		val = "1"
	}
	return p.writePageSetting(changes, filename, data, root, old, evenAndOddHeaders, val, settingsAfterHeaderParity)
}

// Default tab stops precede automatic hyphenation and the later settings.
const settingsAfterDefaultTabStop = " autoHyphenation consecutiveHyphenLimit hyphenationZone doNotHyphenateCaps showEnvelope summaryLength clickAndTypeStyle defaultTableStyle evenAndOddHeaders" + settingsAfterHeaderParity

func (p *documentPackage) updateDefaultTabStop(value int, changes map[string][]byte) error {
	filename, data, root, err := p.pageSettings(changes)
	if err != nil {
		return err
	}
	if sectionChildCount(root, defaultTabStopProperty) > 1 {
		return fmt.Errorf("duplicate DOCX default tab stop")
	}
	old := root.child(defaultTabStopProperty)
	return p.writePageSetting(changes, filename, data, root, old, defaultTabStopProperty, strconv.Itoa(value), settingsAfterDefaultTabStop)
}

func (p *documentPackage) writePageSetting(changes map[string][]byte, filename string, data []byte, root, old *node, name, val, laterSettings string) error {
	raw, err := patchProperty(old, data, name, map[string]string{"val": val}, nil)
	if err != nil {
		return err
	}
	if root == nil {
		filename, err = p.addWordPart(changes, "settings", "rIdDocxSettings")
		if err != nil {
			return err
		}
		changes[filename] = []byte(`<w:settings xmlns:w="` + wordNS + `">` + raw + `</w:settings>`)
		return nil
	}
	start, end := root.endStart, root.endStart
	if old != nil {
		start, end = old.start, old.end
	} else {
		if strings.HasSuffix(string(data[root.start:root.inner]), "/>") {
			changes[filename], err = appendXMLChild(data, raw)
			return err
		}
		for _, child := range root.children {
			if child.name.Space != wordNS || strings.Contains(laterSettings, " "+child.name.Local+" ") {
				start, end = child.start, child.start
				break
			}
		}
	}
	changes[filename] = append(append(append([]byte(nil), data[:start]...), raw...), data[end:]...)
	return nil
}
