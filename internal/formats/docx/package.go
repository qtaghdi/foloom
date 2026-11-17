package docx

import (
	"archive/zip"
	"bytes"
	"fmt"
	"hash/crc32"
	"io"
	"path"
	"strings"
)

type part struct {
	header zip.FileHeader
	data   []byte
}

type documentPackage struct {
	parts         []part
	document      []byte
	styles        previewStyles
	stylesPath    string
	numberingPath string
}

func loadPackage(content []byte) (*documentPackage, error) {
	if len(content) > maxFileBytes {
		return nil, fmt.Errorf("DOCX exceeds 32 MiB compressed limit")
	}
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return nil, fmt.Errorf("open DOCX archive: %w", err)
	}
	if len(reader.File) > 2048 {
		return nil, fmt.Errorf("DOCX part count exceeds 2048")
	}
	p := &documentPackage{}
	names := map[string]bool{}
	total := 0
	for _, f := range reader.File {
		name := f.Name
		if name == "" || name == ".." || strings.HasPrefix(name, "../") || strings.ContainsAny(name, "\\:\x00") || strings.HasPrefix(name, "/") || path.Clean(name) != strings.TrimSuffix(name, "/") || names[name] {
			return nil, fmt.Errorf("unsafe or duplicate DOCX part")
		}
		names[name] = true
		if f.Flags&1 != 0 || f.UncompressedSize64 > maxPartBytes {
			return nil, fmt.Errorf("encrypted or oversized DOCX part")
		}
		if f.FileInfo().IsDir() {
			continue
		}
		if !f.Mode().IsRegular() {
			return nil, fmt.Errorf("non-regular DOCX part")
		}
		total += int(f.UncompressedSize64)
		if total > maxExpandedBytes {
			return nil, fmt.Errorf("DOCX exceeds 64 MiB expanded limit")
		}
		r, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("open DOCX part: %w", err)
		}
		data, readErr := io.ReadAll(io.LimitReader(r, maxPartBytes+1))
		closeErr := r.Close()
		if readErr != nil || closeErr != nil || len(data) > maxPartBytes {
			return nil, fmt.Errorf("read DOCX part failed")
		}
		lower := strings.ToLower(name)
		if strings.HasPrefix(lower, "_xmlsignatures/") || strings.Contains(lower, "vbaproject") {
			return nil, fmt.Errorf("signed or macro documents are unsupported")
		}
		if strings.HasSuffix(lower, ".xml") || strings.HasSuffix(lower, ".rels") {
			if _, err := parseXML(data); err != nil {
				return nil, fmt.Errorf("validate DOCX XML part: %w", err)
			}
		}
		p.parts = append(p.parts, part{header: f.FileHeader, data: data})
		if name == "word/document.xml" {
			p.document = data
		}
	}
	if p.document == nil || !names["[Content_Types].xml"] || !names["_rels/.rels"] {
		return nil, fmt.Errorf("missing DOCX package parts")
	}
	mainFound := false
	stylesPath := ""
	contentTypeFound := false
	for _, part := range p.parts {
		if part.header.Name == "[Content_Types].xml" {
			root, _ := parseXML(part.data)
			for _, c := range root.children {
				var partName, ct string
				for _, a := range c.attrs {
					if a.Name.Local == "PartName" {
						partName = a.Value
					}
					if a.Name.Local == "ContentType" {
						ct = a.Value
					}
				}
				if strings.Contains(strings.ToLower(ct), "macroenabled") {
					return nil, fmt.Errorf("macro-enabled document content is unsupported")
				}
				if partName == "/word/document.xml" && ct == "application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml" {
					contentTypeFound = true
				}
			}
		}
		if !strings.HasSuffix(part.header.Name, ".rels") {
			continue
		}
		root, _ := parseXML(part.data)
		if root.name.Local != "Relationships" || root.name.Space != "http://schemas.openxmlformats.org/package/2006/relationships" {
			return nil, fmt.Errorf("invalid relationship root")
		}
		ids := map[string]bool{}
		for _, c := range root.children {
			var target, mode, kind, id string
			for _, a := range c.attrs {
				switch a.Name.Local {
				case "Target":
					target = a.Value
				case "TargetMode":
					mode = a.Value
				case "Type":
					kind = a.Value
				case "Id":
					id = a.Value
				}
			}
			if id == "" || ids[id] || target == "" {
				return nil, fmt.Errorf("invalid or duplicate relationship")
			}
			ids[id] = true
			// External relationships are retained as data and never fetched.
			if mode == "External" {
				continue
			}
			if mode != "" && mode != "Internal" {
				return nil, fmt.Errorf("invalid relationship mode")
			}
			if strings.ContainsAny(target, "\\%?#:\x00") {
				return nil, fmt.Errorf("unsupported internal relationship target")
			}
			base := path.Dir(path.Dir(part.header.Name))
			resolved := strings.TrimPrefix(path.Clean(path.Join(base, target)), "/")
			if strings.HasPrefix(target, "/") {
				resolved = strings.TrimPrefix(path.Clean(target), "/")
			}
			if !names[resolved] {
				return nil, fmt.Errorf("missing internal relationship target")
			}
			if part.header.Name == "_rels/.rels" && kind == "http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" && resolved == "word/document.xml" {
				mainFound = true
			}
			if part.header.Name == "word/_rels/document.xml.rels" && kind == "http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" {
				stylesPath = resolved
			}
			if part.header.Name == "word/_rels/document.xml.rels" && kind == numberingRelationship {
				if p.numberingPath != "" && p.numberingPath != resolved {
					return nil, fmt.Errorf("ambiguous DOCX numbering relationship")
				}
				p.numberingPath = resolved
			}
		}
	}
	if !mainFound || !contentTypeFound {
		return nil, fmt.Errorf("unsupported DOCX main document mapping")
	}
	p.stylesPath = stylesPath
	for _, part := range p.parts {
		if part.header.Name == stylesPath {
			root, _ := parseXML(part.data)
			p.styles = readPreviewStyles(root)
			break
		}
	}
	return p, nil
}

func (p *documentPackage) serialize(document []byte) ([]byte, error) {
	var b bytes.Buffer
	writer := zip.NewWriter(&b)
	for _, part := range p.parts {
		header := part.header
		data := part.data
		if header.Name == "word/document.xml" {
			data = document
		}
		var w io.Writer
		var err error
		if header.Method == zip.Store {
			// LibreOffice Writer cannot reliably discover the end of stored data when the
			// local header omits its size in favor of a trailing descriptor.
			header.Flags &^= 1 << 3
			header.CRC32 = crc32.ChecksumIEEE(data)
			header.UncompressedSize64 = uint64(len(data))
			header.CompressedSize64 = header.UncompressedSize64
			w, err = writer.CreateRaw(&header)
		} else {
			w, err = writer.CreateHeader(&header)
		}
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(data); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
