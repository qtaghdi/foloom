package docx

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Create installs a complete file by exclusive hard link. An existing destination,
// including a symlink, is never replaced or exposed to partially written content.
func Create(ctx context.Context, filename string, blocks []InitialBlock) (*Session, error) {
	if !filepath.IsAbs(filename) || !strings.EqualFold(filepath.Ext(filename), ".docx") {
		return nil, fmt.Errorf("DOCX path must be absolute and end in .docx")
	}
	if len(blocks) == 0 || len(blocks) > 1000 {
		return nil, fmt.Errorf("create DOCX requires 1 to 1000 blocks")
	}
	var document strings.Builder
	document.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><w:document xmlns:w="` + wordNS + `"><w:body>`)
	count := 0
	paragraph := func(text string) error {
		count++
		if count > 3000 || document.Len()+len(text)*6 > 4<<20 {
			return fmt.Errorf("initial DOCX content limit exceeded")
		}
		if !validText(text) {
			return fmt.Errorf("invalid paragraph or table text")
		}
		document.WriteString(`<w:p><w:r><w:t xml:space="preserve">` + escapeText(text) + `</w:t></w:r></w:p>`)
		return nil
	}
	for _, block := range blocks {
		switch block.Type {
		case "paragraph":
			if len(block.Rows) != 0 {
				return nil, fmt.Errorf("paragraph cannot contain rows")
			}
			if err := paragraph(block.Text); err != nil {
				return nil, err
			}
		case "table":
			if block.Text != "" || len(block.Rows) == 0 || len(block.Rows) > 100 {
				return nil, fmt.Errorf("table requires 1 to 100 rows")
			}
			columns := len(block.Rows[0])
			if columns == 0 || columns > 50 {
				return nil, fmt.Errorf("table requires 1 to 50 columns")
			}
			document.WriteString(`<w:tbl><w:tblPr><w:tblW w:w="0" w:type="auto"/></w:tblPr><w:tblGrid>`)
			for range columns {
				document.WriteString(`<w:gridCol w:w="2400"/>`)
			}
			document.WriteString(`</w:tblGrid>`)
			for _, row := range block.Rows {
				if len(row) != columns {
					return nil, fmt.Errorf("table rows must have equal column counts")
				}
				document.WriteString(`<w:tr>`)
				for _, text := range row {
					document.WriteString(`<w:tc><w:tcPr><w:tcW w:w="2400" w:type="dxa"/></w:tcPr>`)
					if err := paragraph(text); err != nil {
						return nil, err
					}
					document.WriteString(`</w:tc>`)
				}
				document.WriteString(`</w:tr>`)
			}
			document.WriteString(`</w:tbl>`)
		default:
			return nil, fmt.Errorf("unsupported initial DOCX block type")
		}
		if count > 3000 || document.Len() > 4<<20 {
			return nil, fmt.Errorf("initial DOCX content limit exceeded")
		}
	}
	document.WriteString(`<w:sectPr><w:pgSz w:w="11906" w:h="16838"/><w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440"/></w:sectPr></w:body></w:document>`)
	parts := []struct{ name, data string }{
		{"[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`},
		{"_rels/.rels", `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`},
		{"word/document.xml", document.String()},
	}
	var b bytes.Buffer
	writer := zip.NewWriter(&b)
	for _, part := range parts {
		w, err := writer.Create(part.name)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write([]byte(part.data)); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	pkg, err := loadPackage(b.Bytes())
	if err != nil {
		return nil, err
	}
	if _, _, _, err := inspect(pkg.document, "create", 1); err != nil {
		return nil, err
	}
	directory, err := filepath.EvalSymlinks(filepath.Dir(filename))
	if err != nil {
		return nil, fmt.Errorf("resolve DOCX destination directory: %w", err)
	}
	filename = filepath.Join(directory, filepath.Base(filename))
	session, err := newSession(filename, b.Bytes(), pkg)
	if err != nil {
		return nil, err
	}
	name, err := prepareFile(ctx, filename, b.Bytes(), 0o600)
	if err != nil {
		return nil, err
	}
	defer os.Remove(name)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := os.Link(name, filename); err != nil {
		return nil, fmt.Errorf("create DOCX without overwrite: %w", err)
	}
	return session, nil
}
