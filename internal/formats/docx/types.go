// Package docx edits bounded, plain text runs without reserializing unrelated OOXML.
package docx

import "errors"

var (
	ErrRevisionConflict = errors.New("DOCX revision conflict; inspect the document again")
	ErrExternalChange   = errors.New("DOCX changed outside this session; reopen it before editing")
	ErrWriterBusy       = errors.New("DOCX writer lock exists; another writer or interrupted save requires attention")
)

const (
	wordNS           = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	maxFileBytes     = 32 << 20
	maxPartBytes     = 16 << 20
	maxExpandedBytes = 64 << 20
	maxTextBytes     = 64 << 10
	maxRuns          = 10000
	maxOperations    = 1000
)

type Run struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	Editable bool   `json:"editable"`
	Bold     bool   `json:"bold"`
	Italic   bool   `json:"italic"`
}

type Paragraph struct {
	ID   string `json:"id"`
	Runs []Run  `json:"runs"`
}

type Cell struct {
	ID         string      `json:"id"`
	Paragraphs []Paragraph `json:"paragraphs"`
}

type Row struct {
	Cells []Cell `json:"cells"`
}

type Block struct {
	ID        string     `json:"id"`
	Type      string     `json:"type"`
	Paragraph *Paragraph `json:"paragraph,omitempty"`
	Rows      []Row      `json:"rows,omitempty"`
}

type Snapshot struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Revision uint64   `json:"revision"`
	Sequence uint64   `json:"sequence"`
	Blocks   []Block  `json:"blocks"`
	Warnings []string `json:"warnings"`
}

type Operation struct {
	Type         string `json:"type" jsonschema:"replace_run_text for body runs or replace_cell_text for simple table runs"`
	TargetID     string `json:"targetId" jsonschema:"Editable run ID from the current get_docx snapshot"`
	ExpectedText string `json:"expectedText" jsonschema:"Exact current text, including whitespace"`
	Text         string `json:"text" jsonschema:"Replacement plain text without tabs or line breaks"`
}

type ApplyInput struct {
	SessionID    string      `json:"sessionId" jsonschema:"Document session ID from get_docx"`
	BaseRevision uint64      `json:"baseRevision" jsonschema:"Revision from get_docx"`
	Operations   []Operation `json:"operations"`
}

// InitialBlock is deliberately separate from an imported document's model.
// Creating simple content must never normalize arbitrary imported XML.
type InitialBlock struct {
	Type string     `json:"type" jsonschema:"paragraph or table"`
	Text string     `json:"text,omitempty"`
	Rows [][]string `json:"rows,omitempty"`
}

type Event struct {
	Sequence uint64 `json:"sequence"`
	Revision uint64 `json:"revision"`
	Actor    string `json:"actor"`
	Type     string `json:"type"`
}
