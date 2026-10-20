package xlsx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestUnrelatedEditPreservesExplicitValidationDisplayAttributes(t *testing.T) {
	workbookPath := filepath.Join(t.TempDir(), "validation-display-attributes.xlsx")
	file := excelize.NewFile()
	for _, ref := range []string{"A1:A2", "B1:B2", "C1:C2", "D1:D2"} {
		validation := excelize.NewDataValidation(false)
		validation.Sqref = ref
		validation.SetDropList([]string{"Alpha", "Beta"})
		if err := file.AddDataValidation("Sheet1", validation); err != nil {
			t.Fatal(err)
		}
	}
	if err := file.SaveAs(workbookPath); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(workbookPath)
	if err != nil {
		t.Fatal(err)
	}
	parts, err := worksheetParts(content)
	if err != nil {
		t.Fatal(err)
	}
	worksheet := parts["xl/worksheets/sheet1.xml"]
	worksheet = injectValidationDisplayAttributes(t, worksheet, "A1:A2", ` showDropDown="0" showInputMessage="1" showErrorMessage="0"`)
	worksheet = injectValidationDisplayAttributes(t, worksheet, "B1:B2", ` showDropDown="1" showInputMessage="0" showErrorMessage="1"`)
	worksheet = injectValidationDisplayAttributes(t, worksheet, "D1:D2", ` showDropDown="1" showInputMessage="1" showErrorMessage="1"`)
	content, err = replaceZIPParts(content, map[string][]byte{"xl/worksheets/sheet1.xml": worksheet})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(workbookPath, content, 0o600); err != nil {
		t.Fatal(err)
	}

	before := inspectValidationDisplayAttributes(t, workbookPath)
	want := map[string]validationDisplayAttributes{
		"A1:A2": {"showDropDown": "0", "showInputMessage": "1", "showErrorMessage": "0"},
		"B1:B2": {"showDropDown": "1", "showInputMessage": "0", "showErrorMessage": "1"},
		"C1:C2": {},
		"D1:D2": {"showDropDown": "1", "showInputMessage": "1", "showErrorMessage": "1"},
	}
	if !reflect.DeepEqual(before, want) {
		t.Fatalf("initial display attributes = %#v, want %#v", before, want)
	}

	session, err := Open(workbookPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.Apply(1, "validation-preservation-test", []Operation{{
		Type: "set_cell", Sheet: "Sheet1", Cell: "Z50", Value: "unrelated edit",
	}}); err != nil {
		_ = session.Close()
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}

	after := inspectValidationDisplayAttributes(t, workbookPath)
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("display attributes changed after unrelated edit: before=%#v after=%#v", before, after)
	}
	reopened, err := excelize.OpenFile(workbookPath)
	if err != nil {
		t.Fatalf("reopen preserved workbook with Excelize: %v", err)
	}
	defer reopened.Close()
	validations, err := reopened.GetDataValidations("Sheet1")
	if err != nil {
		t.Fatal(err)
	}
	if len(validations) != 4 {
		t.Fatalf("validation count = %d, want 4", len(validations))
	}
	byRange := make(map[string]*excelize.DataValidation, len(validations))
	for _, validation := range validations {
		byRange[validation.Sqref] = validation
	}
	if item := byRange["A1:A2"]; item == nil || item.ShowDropDown || !item.ShowInputMessage || item.ShowErrorMessage {
		t.Fatalf("A1:A2 validation = %#v", item)
	}
	if item := byRange["B1:B2"]; item == nil || !item.ShowDropDown || item.ShowInputMessage || !item.ShowErrorMessage {
		t.Fatalf("B1:B2 validation = %#v", item)
	}
	if item := byRange["D1:D2"]; item == nil || !item.ShowDropDown || !item.ShowInputMessage || !item.ShowErrorMessage {
		t.Fatalf("D1:D2 validation = %#v", item)
	}
	marker, err := reopened.GetCellValue("Sheet1", "Z50")
	if err != nil || marker != "unrelated edit" {
		t.Fatalf("marker = %q, err = %v", marker, err)
	}
}

func TestValidationDisplayAttributePreservationRejectsIdentityMismatch(t *testing.T) {
	source := []byte(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><dataValidations count="1"><dataValidation type="list" sqref="A1" showDropDown="0"><formula1>"A,B"</formula1></dataValidation></dataValidations></worksheet>`)
	candidate := []byte(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><dataValidations count="1"><dataValidation type="list" sqref="B1"><formula1>"A,B"</formula1></dataValidation></dataValidations></worksheet>`)
	sourceNodes, err := validationNodes(source)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := restoreWorksheetValidationAttributes(sourceNodes, candidate); err == nil {
		t.Fatal("expected mismatched validation identity to fail")
	}
}

func injectValidationDisplayAttributes(t *testing.T, worksheet []byte, ref, attributes string) []byte {
	t.Helper()
	needle := `sqref="` + ref + `"`
	index := bytes.Index(worksheet, []byte(needle))
	if index < 0 {
		t.Fatalf("validation %s not found in worksheet XML", ref)
	}
	closing := bytes.IndexByte(worksheet[index:], '>')
	if closing < 0 {
		t.Fatalf("validation %s start tag is incomplete", ref)
	}
	insertAt := index + closing
	result := make([]byte, 0, len(worksheet)+len(attributes))
	result = append(result, worksheet[:insertAt]...)
	result = append(result, attributes...)
	result = append(result, worksheet[insertAt:]...)
	return result
}

func inspectValidationDisplayAttributes(t *testing.T, workbookPath string) map[string]validationDisplayAttributes {
	t.Helper()
	archive, err := zip.OpenReader(workbookPath)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	var worksheet []byte
	for _, entry := range archive.File {
		if entry.Name != "xl/worksheets/sheet1.xml" {
			continue
		}
		reader, openErr := entry.Open()
		if openErr != nil {
			t.Fatal(openErr)
		}
		worksheet, err = io.ReadAll(reader)
		if closeErr := reader.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			t.Fatal(err)
		}
		break
	}
	if len(worksheet) == 0 {
		t.Fatal("worksheet XML not found")
	}
	result := make(map[string]validationDisplayAttributes)
	decoder := xml.NewDecoder(bytes.NewReader(worksheet))
	for {
		token, tokenErr := decoder.Token()
		if tokenErr == io.EOF {
			break
		}
		if tokenErr != nil {
			t.Fatal(tokenErr)
		}
		element, ok := token.(xml.StartElement)
		if !ok || element.Name.Local != "dataValidation" {
			continue
		}
		ref := ""
		display := make(validationDisplayAttributes)
		for _, attribute := range element.Attr {
			if attribute.Name.Space != "" {
				continue
			}
			if attribute.Name.Local == "sqref" {
				ref = attribute.Value
			}
			if isValidationDisplayAttribute(attribute.Name.Local) {
				display[attribute.Name.Local] = attribute.Value
			}
		}
		result[strings.TrimSpace(ref)] = display
	}
	return result
}
