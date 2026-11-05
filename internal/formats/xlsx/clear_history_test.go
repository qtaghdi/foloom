package xlsx

import (
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestClearContentsHistoryPreservesFormulaStyleAndOtherCells(t *testing.T) {
	const sheet = "Sheet1"
	path := filepath.Join(t.TempDir(), "clear-history.xlsx")
	source := excelize.NewFile()
	style, err := source.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		t.Fatal(err)
	}
	for _, err := range []error{
		source.SetCellValue(sheet, "B1", "Amount"),
		source.SetCellFormula(sheet, "C1", "=2+3"),
		source.SetCellValue(sheet, "D1", "keep"),
		source.SetCellStyle(sheet, "B1", "C1", style),
		source.SaveAs(path),
		source.Close(),
	} {
		if err != nil {
			t.Fatal(err)
		}
	}
	session, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	check := func(cleared bool) {
		t.Helper()
		file, err := excelize.OpenFile(path)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		text, err := file.GetCellValue(sheet, "B1")
		if err != nil || (cleared && text != "") || (!cleared && text != "Amount") {
			t.Fatalf("B1 = %q, cleared = %v, error = %v", text, cleared, err)
		}
		formula, err := file.GetCellFormula(sheet, "C1")
		if err != nil || (cleared && formula != "") || (!cleared && formula != "=2+3") {
			t.Fatalf("C1 formula = %q, cleared = %v, error = %v", formula, cleared, err)
		}
		neighbor, err := file.GetCellValue(sheet, "D1")
		if err != nil || neighbor != "keep" {
			t.Fatalf("D1 = %q, error = %v", neighbor, err)
		}
		for _, cell := range []string{"B1", "C1"} {
			got, err := file.GetCellStyle(sheet, cell)
			if err != nil || got != style {
				t.Fatalf("%s style = %d, error = %v", cell, got, err)
			}
		}
	}
	deleted, err := session.Apply(1, "human", []Operation{
		{Type: "set_cell", Sheet: sheet, Cell: "B1"},
		{Type: "set_cell", Sheet: sheet, Cell: "C1"},
	})
	if err != nil || deleted.Revision != 2 || !deleted.CanUndo {
		t.Fatalf("clear revision = %d, error = %v", deleted.Revision, err)
	}
	check(true)
	undone, err := session.RestoreHistory(2, "human", "undo")
	if err != nil || undone.Revision != 3 || !undone.CanRedo {
		t.Fatalf("undo revision = %d, error = %v", undone.Revision, err)
	}
	check(false)
	redone, err := session.RestoreHistory(3, "human", "redo")
	if err != nil || redone.Revision != 4 {
		t.Fatalf("redo revision = %d, error = %v", redone.Revision, err)
	}
	check(true)
}
