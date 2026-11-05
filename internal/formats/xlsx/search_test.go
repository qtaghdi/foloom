package xlsx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestSearchStoredValuesAcrossSheetsWithoutMutation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "search.xlsx")
	f := excelize.NewFile()
	f.SetCellValue("Sheet1", "A1", "Revenue 매출")
	f.SetCellValue("Sheet1", "Z2000", "매출 상세")
	f.NewSheet("다음")
	f.SetCellValue("다음", "B3", "REVENUE")
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	f.Close()
	before, _ := os.ReadFile(path)
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	revision := s.Snapshot().Revision
	result, err := s.Search(context.Background(), "revenue")
	if err != nil || len(result.Hits) != 2 || result.Hits[1].Sheet != "다음" || result.Truncated {
		t.Fatalf("result: %+v, %v", result, err)
	}
	result, err = s.Search(context.Background(), "상세")
	if err != nil || len(result.Hits) != 1 || result.Hits[0].Cell != "Z2000" {
		t.Fatalf("distant result: %+v, %v", result, err)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) || s.Snapshot().Revision != revision {
		t.Fatal("search changed source or revision")
	}
	for _, query := range []string{"", "  ", strings.Repeat("가", 201)} {
		if _, err := s.Search(context.Background(), query); err == nil {
			t.Fatal("accepted invalid query")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Search(ctx, "revenue"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
}

func TestSearchBoundsResults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "many.xlsx")
	f := excelize.NewFile()
	for i := 1; i <= 201; i++ {
		f.SetCellValue("Sheet1", fmt.Sprintf("A%d", i), strings.Repeat("매출", 300))
	}
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	f.Close()
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	result, err := s.Search(context.Background(), "매출")
	if err != nil || !result.Truncated || len(result.Hits) != 200 || len([]rune(result.Hits[0].Text)) != 241 {
		t.Fatalf("bounds: %+v %v", result, err)
	}
}
