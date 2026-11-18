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
	BoldTrackable    bool         `json:"boldTrackable,omitempty"`
	ProofingLang     string       `json:"proofingLanguage,omitempty"`
	EastAsiaLang     string       `json:"eastAsiaProofingLanguage,omitempty"`
	Trackable        bool         `json:"trackable,omitempty"`
	Object           *ObjectInfo  `json:"object,omitempty"`
	Link             *LinkInfo    `json:"link,omitempty"`
	Picture          *PictureInfo `json:"picture,omitempty"`
	CharacterStyleID string       `json:"characterStyleId,omitempty"`
	BaseStyle        *InlineStyle `json:"baseStyle,omitempty"`
	Underline        bool         `json:"underline,omitempty"`
	Strike           bool         `json:"strike,omitempty"`
	Highlight        string       `json:"highlight,omitempty"`
	Vertical         string       `json:"vertical,omitempty"`
	ID               string       `json:"id"`
	Text             string       `json:"text"`
	Editable         bool         `json:"editable"`
	Bold             bool         `json:"bold"`
	Italic           bool         `json:"italic"`
	SizeHalfPoints   int          `json:"sizeHalfPoints,omitempty"`
	Color            string       `json:"color,omitempty"`
	Font             string       `json:"font,omitempty"`
	EastAsiaFont     string       `json:"eastAsiaFont,omitempty"`
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

type ListDefinition struct {
	Key   string `json:"key" jsonschema:"Batch-local reference key for new list paragraphs"`
	Kind  string `json:"kind" jsonschema:"bullet, decimal, upperRoman, lowerRoman, upperLetter, or lowerLetter"`
	Start int    `json:"start,omitempty" jsonschema:"Start number from 1 to 9999, default 1"`
}

type ParagraphList struct {
	ID    int    `json:"id,omitempty" jsonschema:"Existing list ID or zero to remove numbering"`
	Key   string `json:"key,omitempty" jsonschema:"Reference a define_list key from the same batch instead of ID"`
	Level int    `json:"level,omitempty" jsonschema:"Zero-based level from 0 to 8"`
}

type ParagraphFormat struct {
	Tabs          []TabStop      `json:"tabs,omitempty"`
	List          *ParagraphList `json:"list,omitempty"`
	StyleID       *string        `json:"styleId,omitempty"`
	Align         string         `json:"align,omitempty"`
	Left          *int           `json:"left,omitempty"`
	Right         *int           `json:"right,omitempty"`
	FirstLine     *int           `json:"firstLine,omitempty"`
	Hanging       *int           `json:"hanging,omitempty"`
	Before        *int           `json:"before,omitempty"`
	After         *int           `json:"after,omitempty"`
	Line          *int           `json:"line,omitempty"`
	LineRule      string         `json:"lineRule,omitempty"`
	Bidi          *bool          `json:"bidi,omitempty"`
	TextDirection string         `json:"textDirection,omitempty"`
}
type ParagraphLayout struct {
	Tabs          []TabStop `json:"tabs,omitempty"`
	Left          *int      `json:"left,omitempty"`
	Right         *int      `json:"right,omitempty"`
	FirstLine     *int      `json:"firstLine,omitempty"`
	Hanging       *int      `json:"hanging,omitempty"`
	Line          *int      `json:"line,omitempty"`
	LineRule      string    `json:"lineRule,omitempty"`
	Bidi          bool      `json:"bidi,omitempty"`
	TextDirection string    `json:"textDirection,omitempty"`
	Align         string    `json:"align,omitempty"`
	Before        *int      `json:"before,omitempty"`
	After         *int      `json:"after,omitempty"`
}

type ListInfo struct {
	ID     int             `json:"id"`
	Levels []ListLevelInfo `json:"levels"`
}
type ListLevelInfo struct {
	Level   int    `json:"level"`
	Kind    string `json:"kind"`
	Text    string `json:"text"`
	Start   int    `json:"start"`
	Legal   bool   `json:"legal,omitempty"`
	Restart *int   `json:"restart,omitempty"`
}
type Paragraph struct {
	MoveBookmarkUp    bool             `json:"moveBookmarkUp,omitempty"`
	MoveBookmarkDown  bool             `json:"moveBookmarkDown,omitempty"`
	List              *ParagraphList   `json:"list,omitempty"`
	ListLabel         string           `json:"listLabel,omitempty"`
	Anchor            string           `json:"anchor,omitempty"`
	OutlineLevel      *int             `json:"outlineLevel,omitempty"`
	OutlineOverride   *int             `json:"outlineOverride,omitempty"`
	SectionID         string           `json:"sectionId,omitempty"`
	StyleID           string           `json:"styleId,omitempty"`
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
	Height int    `json:"height,omitempty"`
	Cells  []Cell `json:"cells"`
}

type Block struct {
	TableEditable bool       `json:"tableEditable,omitempty"`
	ID            string     `json:"id"`
	Type          string     `json:"type"`
	Paragraph     *Paragraph `json:"paragraph,omitempty"`
	Rows          []Row      `json:"rows,omitempty"`
}

type HistoryState struct {
	Undo      int    `json:"undo"`
	Redo      int    `json:"redo"`
	UndoActor string `json:"undoActor,omitempty"`
	RedoActor string `json:"redoActor,omitempty"`
}
type HistoryInput struct {
	SessionID    string `json:"sessionId"`
	BaseRevision uint64 `json:"baseRevision"`
	Direction    string `json:"direction" jsonschema:"undo or redo"`
}
type Snapshot struct {
	Lists             []ListInfo      `json:"lists,omitempty"`
	Comments          []ReviewComment `json:"comments,omitempty"`
	TrackedChanges    bool            `json:"trackedChanges,omitempty"`
	Bookmarks         []BookmarkInfo  `json:"bookmarks,omitempty"`
	EvenAndOddHeaders *bool           `json:"evenAndOddHeaders,omitempty"`
	Sections          []SectionInfo   `json:"sections"`
	Styles            []StyleInfo     `json:"styles"`
	History           HistoryState    `json:"history"`
	ID                string          `json:"id"`
	Name              string          `json:"name"`
	Revision          uint64          `json:"revision"`
	Sequence          uint64          `json:"sequence"`
	Blocks            []Block         `json:"blocks"`
	Page              *PageLayout     `json:"page,omitempty"`
	Warnings          []string        `json:"warnings"`
}

// ParagraphContent references formatting in the inspected revision. Repeated
// source paragraphs create new identities rather than cloning OOXML IDs.
type ParagraphContent struct {
	Format   *ParagraphFormat `json:"format,omitempty"`
	SourceID string           `json:"sourceId"`
	Runs     []RunContent     `json:"runs"`
}

// InlineStyle contains explicit, bounded clipboard formatting overrides. Source
// run properties not named here remain unchanged.
type InlineStyle struct {
	ProofingLang   string  `json:"proofingLanguage,omitempty" jsonschema:"BCP 47 tag for Latin-script spelling and grammar"`
	EastAsiaLang   string  `json:"eastAsiaProofingLanguage,omitempty" jsonschema:"BCP 47 tag for East Asian spelling and grammar"`
	StyleID        *string `json:"styleId,omitempty"`
	Reset          bool    `json:"reset,omitempty"`
	Underline      *bool   `json:"underline,omitempty"`
	Strike         *bool   `json:"strike,omitempty"`
	Highlight      string  `json:"highlight,omitempty"`
	Vertical       string  `json:"vertical,omitempty"`
	Bold           *bool   `json:"bold,omitempty"`
	Italic         *bool   `json:"italic,omitempty"`
	Color          string  `json:"color,omitempty"`
	Font           string  `json:"font,omitempty"`
	EastAsiaFont   string  `json:"eastAsiaFont,omitempty" jsonschema:"Override only the East Asian typeface; takes precedence over font for that slot"`
	SizeHalfPoints *int    `json:"sizeHalfPoints,omitempty"`
}
type RunContent struct {
	Style    *InlineStyle `json:"style,omitempty"`
	SourceID string       `json:"sourceId"`
	Text     string       `json:"text"`
}

type StyleDefinition struct {
	ID        string           `json:"id"`
	Name      string           `json:"name,omitempty"`
	Kind      string           `json:"kind"`
	BasedOn   *string          `json:"basedOn,omitempty"`
	Run       *InlineStyle     `json:"run,omitempty"`
	Paragraph *ParagraphFormat `json:"paragraph,omitempty"`
}
type StyleInfo struct {
	OutlineLevel *int             `json:"outlineLevel,omitempty"`
	ID           string           `json:"id"`
	Name         string           `json:"name"`
	Kind         string           `json:"kind"`
	BasedOn      string           `json:"basedOn,omitempty"`
	Editable     bool             `json:"editable"`
	Run          *InlineStyle     `json:"run,omitempty"`
	Paragraph    *ParagraphFormat `json:"paragraph,omitempty"`
}
type Operation struct {
	Bold              *bool              `json:"bold,omitempty" jsonschema:"Set bold for track_run_bold"`
	ListDefinition    *ListDefinition    `json:"listDefinition,omitempty"`
	Table             *TableEdit         `json:"table,omitempty"`
	Math              *MathEdit          `json:"math,omitempty"`
	Comment           *CommentEdit       `json:"comment,omitempty"`
	Reference         *ReferenceEdit     `json:"reference,omitempty"`
	Image             *ImageEdit         `json:"image,omitempty"`
	Boundary          string             `json:"boundary,omitempty" jsonschema:"page_before, no_page_before, no_page_after, section_next_page, section_continuous, remove_section, move_section_earlier, or move_section_later"`
	EvenAndOddHeaders *bool              `json:"evenAndOddHeaders,omitempty" jsonschema:"Document-wide odd/even header and footer setting for update_header_parity"`
	SectionID         string             `json:"sectionId,omitempty"`
	Section           *SectionFormat     `json:"section,omitempty"`
	StyleDefinition   *StyleDefinition   `json:"styleDefinition,omitempty"`
	EndTargetID       string             `json:"endTargetId,omitempty" jsonschema:"Last contiguous paragraph ID for replace_paragraphs"`
	Paragraphs        []ParagraphContent `json:"paragraphs,omitempty" jsonschema:"Ordered replacement paragraphs with source paragraph and run formatting IDs"`
	Type              string             `json:"type" jsonschema:"replace_run_text, replace_cell_text, replace_paragraphs, define_style, update_section, update_header_parity, update_boundary, update_image, update_reference, add_comment, resolve_comment, reopen_comment, update_math, define_list, update_table, move_bookmark_up, move_bookmark_down, accept_revision, reject_revision, track_run_text, track_run_bold, update_control_text, update_textbox_text, insert_textbox, reply_comment, or insert_picture_caption"`
	TargetID          string             `json:"targetId,omitempty" jsonschema:"Editable run or paragraph ID from the current get_docx snapshot"`
	ExpectedText      string             `json:"expectedText,omitempty" jsonschema:"Exact current text, including whitespace"`
	Text              string             `json:"text,omitempty" jsonschema:"Replacement text without tabs. LF is an inline Word line break. Use replace_paragraphs to split paragraphs"`
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
	TextOnly bool   `json:"textOnly,omitempty"`
	Sequence uint64 `json:"sequence"`
	Revision uint64 `json:"revision"`
	Actor    string `json:"actor"`
	Type     string `json:"type"`
}
