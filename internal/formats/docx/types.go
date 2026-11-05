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
	ID             string `json:"id"`
	Text           string `json:"text"`
	Editable       bool   `json:"editable"`
	Bold           bool   `json:"bold"`
	Italic         bool   `json:"italic"`
	SizeHalfPoints int    `json:"sizeHalfPoints,omitempty"`
	Color          string `json:"color,omitempty"`
	Font           string `json:"font,omitempty"`
}

// Layout fields are bounded preview hints. They never participate in writes.
type PageLayout struct {
	Width  int `json:"width"`
	Height int `json:"height"`
	Top    int `json:"top"`
	Right  int `json:"right"`
	Bottom int `json:"bottom"`
	Left   int `json:"left"`
}

type ParagraphLayout struct {
	Align  string `json:"align,omitempty"`
	Before *int   `json:"before,omitempty"`
	After  *int   `json:"after,omitempty"`
}

type Paragraph struct {
	StructureEditable bool             `json:"structureEditable"`
	PageBreakBefore   bool             `json:"pageBreakBefore,omitempty"`
	PageBreakAfter    bool             `json:"pageBreakAfter,omitempty"`
	ID                string           `json:"id"`
	Runs              []Run            `json:"runs"`
	Layout            *ParagraphLayout `json:"layout,omitempty"`
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
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Revision uint64      `json:"revision"`
	Sequence uint64      `json:"sequence"`
	Blocks   []Block     `json:"blocks"`
	Page     *PageLayout `json:"page,omitempty"`
	Warnings []string    `json:"warnings"`
}

// ParagraphContent references formatting in the inspected revision. Repeated
// source paragraphs create new identities rather than cloning OOXML IDs.
type ParagraphContent struct {
	SourceID string       `json:"sourceId"`
	Runs     []RunContent `json:"runs"`
}

// InlineStyle contains explicit, bounded clipboard formatting overrides. Source
// run properties not named here remain unchanged.
type InlineStyle struct {
	Bold           *bool  `json:"bold,omitempty"`
	Italic         *bool  `json:"italic,omitempty"`
	Color          string `json:"color,omitempty"`
	Font           string `json:"font,omitempty"`
	SizeHalfPoints *int   `json:"sizeHalfPoints,omitempty"`
}
type RunContent struct {
	Style    *InlineStyle `json:"style,omitempty"`
	SourceID string       `json:"sourceId"`
	Text     string       `json:"text"`
}

type Operation struct {
	EndTargetID  string             `json:"endTargetId,omitempty" jsonschema:"Last contiguous paragraph ID for replace_paragraphs"`
	Paragraphs   []ParagraphContent `json:"paragraphs,omitempty" jsonschema:"Ordered replacement paragraphs with source paragraph and run formatting IDs"`
	Type         string             `json:"type" jsonschema:"replace_run_text, replace_cell_text, or replace_paragraphs for an editable contiguous paragraph range"`
	TargetID     string             `json:"targetId" jsonschema:"Editable run ID from the current get_docx snapshot"`
	ExpectedText string             `json:"expectedText" jsonschema:"Exact current text, including whitespace"`
	Text         string             `json:"text" jsonschema:"Replacement text without tabs. LF is an inline Word line break. Use replace_paragraphs to split paragraphs"`
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
