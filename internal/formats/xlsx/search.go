package xlsx

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"
)

const maxSearchResults = 200
const maxSearchCells = 1_000_000

type SearchHit struct {
	Sheet string `json:"sheet"`
	Cell  string `json:"cell"`
	Text  string `json:"text"`
}

type SearchResult struct {
	ID        string      `json:"id"`
	Revision  uint64      `json:"revision"`
	Hits      []SearchHit `json:"hits"`
	Truncated bool        `json:"truncated"`
}

// Search reads stored display values, including cached formula results. It never
// calculates formulas or changes the revision. Bound both work and response size.
func (s *Session) Search(ctx context.Context, query string) (SearchResult, error) {
	if strings.TrimSpace(query) == "" || utf8.RuneCountInString(query) > 200 {
		return SearchResult{}, fmt.Errorf("search query must contain 1 to 200 characters")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := SearchResult{ID: s.id, Revision: s.revision, Hits: []SearchHit{}}
	query = strings.ToLower(query)
	scanned := 0
	for _, sheet := range s.file.GetSheetList() {
		rows, err := s.file.Rows(sheet)
		if err != nil {
			return result, fmt.Errorf("search worksheet: %w", err)
		}
		err = func() error {
			defer rows.Close()
			for row := 1; rows.Next(); row++ {
				if err := ctx.Err(); err != nil {
					return err
				}
				// Empty rows count too, so sparse sheets cannot hold the session lock indefinitely.
				scanned++
				if scanned > maxSearchCells {
					result.Truncated = true
					return nil
				}
				values, err := rows.Columns()
				if err != nil {
					return fmt.Errorf("read search row: %w", err)
				}
				for col, value := range values {
					scanned++
					if scanned > maxSearchCells {
						result.Truncated = true
						return nil
					}
					if !strings.Contains(strings.ToLower(value), query) {
						continue
					}
					if len(result.Hits) == maxSearchResults {
						result.Truncated = true
						return nil
					}
					cell, _ := excelize.CoordinatesToCellName(col+1, row)
					preview := []rune(value)
					if len(preview) > 240 {
						value = string(preview[:240]) + "…"
					}
					result.Hits = append(result.Hits, SearchHit{Sheet: sheet, Cell: cell, Text: value})
				}
			}
			return rows.Error()
		}()
		if err != nil {
			return result, err
		}
		if result.Truncated {
			break
		}
	}
	return result, nil
}
