package httpapi

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/qtaghdi/document-workspace/internal/formats/xlsx"
)

//go:embed static
var staticFS embed.FS

const (
	browserLaunchTTL = 2 * time.Minute
	maxLaunchTokens  = 128
)

type Server struct {
	openMu         sync.Mutex
	sessionMu      sync.RWMutex
	session        *xlsx.Session
	browserToken   string
	mcpToken       string
	mux            *http.ServeMux
	launchMu       sync.Mutex
	browserBaseURL string
	launchTokens   map[string]time.Time
}

var errWorkbookNotOpen = fmt.Errorf("workbook is not open; call open_workbook with an absolute XLSX path first")

func New(session *xlsx.Session, token string) *Server {
	return NewWithTokens(session, token, token)
}

func NewWithTokens(session *xlsx.Session, browserToken, mcpToken string) *Server {
	s := &Server{
		session:      session,
		browserToken: browserToken,
		mcpToken:     mcpToken,
		mux:          http.NewServeMux(),
		launchTokens: make(map[string]time.Time),
	}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return securityHeaders(s.mux)
}

func (s *Server) acquireSession() (*xlsx.Session, func(), error) {
	s.sessionMu.RLock()
	if s.session == nil {
		s.sessionMu.RUnlock()
		return nil, func() {}, errWorkbookNotOpen
	}
	return s.session, s.sessionMu.RUnlock, nil
}

func (s *Server) replaceSession(next *xlsx.Session) error {
	s.sessionMu.Lock()
	previous := s.session
	s.session = next
	s.sessionMu.Unlock()
	if previous != nil {
		_ = previous.Close()
	}
	return nil
}

// Close releases the active workbook, if any. It is safe to call when the
// server was started without an initial workbook.
func (s *Server) Close() error {
	s.openMu.Lock()
	defer s.openMu.Unlock()
	s.sessionMu.Lock()
	previous := s.session
	s.session = nil
	s.sessionMu.Unlock()
	if previous == nil {
		return nil
	}
	return previous.Close()
}

func (s *Server) routes() {
	assets, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(fmt.Sprintf("prepare embedded UI: %v", err))
	}
	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	s.mux.HandleFunc("GET /launch/{token}", s.launchBrowserSession)
	s.mux.Handle("/mcp", s.requireBearer(s.mcpHandler()))
	s.mux.HandleFunc("GET /{$}", s.index)
	s.mux.Handle("GET /assets/", s.requireSession(http.FileServer(http.FS(assets))))
	s.mux.Handle("GET /api/workbook", s.requireSession(http.HandlerFunc(s.getWorkbook)))
	s.mux.Handle("GET /api/range", s.requireSession(http.HandlerFunc(s.getRange)))
	s.mux.Handle("GET /api/sheet-objects", s.requireSession(http.HandlerFunc(s.getSheetObjects)))
	s.mux.Handle("POST /api/operations", s.requireSession(http.HandlerFunc(s.applyOperations)))
	s.mux.Handle("POST /api/history/{direction}", s.requireSession(http.HandlerFunc(s.restoreHistory)))
	s.mux.Handle("POST /api/presence", s.requireSession(http.HandlerFunc(s.updatePresence)))
	s.mux.Handle("GET /api/events", s.requireSession(http.HandlerFunc(s.events)))
}

func (s *Server) issueBrowserLaunchURL() (string, error) {
	s.launchMu.Lock()
	defer s.launchMu.Unlock()
	if s.browserBaseURL == "" {
		return "", nil
	}
	now := time.Now()
	for token, expiresAt := range s.launchTokens {
		if !expiresAt.After(now) {
			delete(s.launchTokens, token)
		}
	}
	if len(s.launchTokens) >= maxLaunchTokens {
		return "", fmt.Errorf("too many outstanding browser launch links")
	}
	var value [24]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("create browser launch token: %w", err)
	}
	token := hex.EncodeToString(value[:])
	s.launchTokens[token] = now.Add(browserLaunchTTL)
	return strings.TrimRight(s.browserBaseURL, "/") + "/launch/" + token, nil
}
