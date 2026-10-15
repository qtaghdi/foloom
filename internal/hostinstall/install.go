package hostinstall

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	serverName       = "document-workspace"
	codexServerName  = "document_workspace"
	managedStart     = "# document-workspace managed start"
	managedEnd       = "# document-workspace managed end"
	installedBinMode = 0o755
)

//go:embed assets/document-xlsx/SKILL.md
var documentXLSXSkill []byte

type Installer struct {
	Home             string
	SourceExecutable string
	GOOS             string
}

type Status struct {
	BinaryPath  string
	Binary      bool
	ClaudeMCP   bool
	CodexMCP    bool
	ClaudeSkill bool
	CodexSkill  bool
}

func New() (*Installer, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("사용자 홈 디렉터리 확인: %w", err)
	}
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("실행 파일 경로 확인: %w", err)
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return nil, fmt.Errorf("실행 파일 실제 경로 확인: %w", err)
	}
	return &Installer{Home: home, SourceExecutable: executable, GOOS: runtime.GOOS}, nil
}

func (i *Installer) Install() (Status, error) {
	if err := i.validate(); err != nil {
		return Status{}, err
	}
	binaryPath := i.binaryPath()
	claudePath := i.claudeConfigPath()
	codexPath := i.codexConfigPath()

	claudeCurrent, err := readOptionalFile(claudePath)
	if err != nil {
		return Status{}, err
	}
	claudeNext, err := mergeClaudeConfig(claudeCurrent, binaryPath)
	if err != nil {
		return Status{}, fmt.Errorf("Claude Desktop 설정 준비: %w", err)
	}
	codexCurrent, err := readOptionalFile(codexPath)
	if err != nil {
		return Status{}, err
	}
	if err := validateCodexManagedBlock(codexCurrent); err != nil {
		return Status{}, err
	}
	codexNext := mergeCodexConfig(codexCurrent, binaryPath)

	if err := copyExecutable(i.SourceExecutable, binaryPath); err != nil {
		return Status{}, fmt.Errorf("실행 파일 설치: %w", err)
	}
	if err := writeChangedConfig(claudePath, claudeCurrent, claudeNext); err != nil {
		return Status{}, fmt.Errorf("Claude Desktop 설정 저장: %w", err)
	}
	if err := writeChangedConfig(codexPath, codexCurrent, codexNext); err != nil {
		return Status{}, fmt.Errorf("Codex 설정 저장: %w", err)
	}
	for _, path := range []string{i.claudeSkillPath(), i.codexSkillPath()} {
		current, err := readOptionalFile(path)
		if err != nil {
			return Status{}, fmt.Errorf("기존 document-xlsx 스킬 읽기: %w", err)
		}
		if err := writeChangedConfig(path, current, documentXLSXSkill); err != nil {
			return Status{}, fmt.Errorf("document-xlsx 스킬 설치: %w", err)
		}
	}
	return i.Status()
}

func (i *Installer) Status() (Status, error) {
	if err := i.validate(); err != nil {
		return Status{}, err
	}
	status := Status{BinaryPath: i.binaryPath()}
	status.Binary = isRegularFile(status.BinaryPath)
	claude, err := readOptionalFile(i.claudeConfigPath())
	if err != nil {
		return Status{}, err
	}
	status.ClaudeMCP = claudeConfigMatches(claude, status.BinaryPath)
	codex, err := readOptionalFile(i.codexConfigPath())
	if err != nil {
		return Status{}, err
	}
	if err := validateCodexManagedBlock(codex); err != nil {
		return Status{}, err
	}
	status.CodexMCP = codexConfigMatches(codex, status.BinaryPath)
	status.ClaudeSkill = fileEquals(i.claudeSkillPath(), documentXLSXSkill)
	status.CodexSkill = fileEquals(i.codexSkillPath(), documentXLSXSkill)
	return status, nil
}

func (i *Installer) Uninstall() (Status, error) {
	if err := i.validate(); err != nil {
		return Status{}, err
	}
	claudePath := i.claudeConfigPath()
	claudeCurrent, err := readOptionalFile(claudePath)
	if err != nil {
		return Status{}, err
	}
	claudeNext, err := removeClaudeConfig(claudeCurrent)
	if err != nil {
		return Status{}, fmt.Errorf("Claude Desktop 설정 준비: %w", err)
	}
	if err := writeChangedConfig(claudePath, claudeCurrent, claudeNext); err != nil {
		return Status{}, fmt.Errorf("Claude Desktop 설정 저장: %w", err)
	}
	codexPath := i.codexConfigPath()
	codexCurrent, err := readOptionalFile(codexPath)
	if err != nil {
		return Status{}, err
	}
	if err := validateCodexManagedBlock(codexCurrent); err != nil {
		return Status{}, err
	}
	if err := writeChangedConfig(codexPath, codexCurrent, removeCodexConfig(codexCurrent)); err != nil {
		return Status{}, fmt.Errorf("Codex 설정 저장: %w", err)
	}
	for _, path := range []string{i.claudeSkillPath(), i.codexSkillPath(), i.binaryPath()} {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return Status{}, fmt.Errorf("설치 파일 제거: %w", err)
		}
	}
	return i.Status()
}

func (i *Installer) validate() error {
	if i.GOOS != "darwin" {
		return fmt.Errorf("MCP 자동 설치는 현재 macOS만 지원합니다")
	}
	if strings.TrimSpace(i.Home) == "" || !filepath.IsAbs(i.Home) {
		return fmt.Errorf("사용자 홈 디렉터리는 절대 경로여야 합니다")
	}
	if strings.TrimSpace(i.SourceExecutable) == "" || !filepath.IsAbs(i.SourceExecutable) {
		return fmt.Errorf("실행 파일은 절대 경로여야 합니다")
	}
	return nil
}

func (i *Installer) binaryPath() string {
	return filepath.Join(i.Home, "Library", "Application Support", serverName, "bin", serverName)
}

func (i *Installer) claudeConfigPath() string {
	return filepath.Join(i.Home, "Library", "Application Support", "Claude", "claude_desktop_config.json")
}

func (i *Installer) codexConfigPath() string {
	return filepath.Join(i.Home, ".codex", "config.toml")
}

func (i *Installer) claudeSkillPath() string {
	return filepath.Join(i.Home, ".claude", "skills", "document-xlsx", "SKILL.md")
}

func (i *Installer) codexSkillPath() string {
	return filepath.Join(i.Home, ".codex", "skills", "document-xlsx", "SKILL.md")
}

func mergeClaudeConfig(current []byte, binaryPath string) ([]byte, error) {
	root, servers, err := parseClaudeConfig(current)
	if err != nil {
		return nil, err
	}
	entry, err := json.Marshal(map[string]any{
		"command": binaryPath,
		"args":    []string{"-transport", "stdio"},
	})
	if err != nil {
		return nil, err
	}
	servers[serverName] = entry
	encodedServers, err := json.Marshal(servers)
	if err != nil {
		return nil, err
	}
	root["mcpServers"] = encodedServers
	return json.MarshalIndent(root, "", "  ")
}

func removeClaudeConfig(current []byte) ([]byte, error) {
	if len(bytes.TrimSpace(current)) == 0 {
		return current, nil
	}
	root, servers, err := parseClaudeConfig(current)
	if err != nil {
		return nil, err
	}
	delete(servers, serverName)
	encodedServers, err := json.Marshal(servers)
	if err != nil {
		return nil, err
	}
	root["mcpServers"] = encodedServers
	return json.MarshalIndent(root, "", "  ")
}

func parseClaudeConfig(current []byte) (map[string]json.RawMessage, map[string]json.RawMessage, error) {
	root := make(map[string]json.RawMessage)
	if len(bytes.TrimSpace(current)) != 0 {
		if err := json.Unmarshal(current, &root); err != nil {
			return nil, nil, fmt.Errorf("기존 JSON이 올바르지 않아 변경하지 않았습니다: %w", err)
		}
	}
	servers := make(map[string]json.RawMessage)
	if raw, ok := root["mcpServers"]; ok && string(raw) != "null" {
		if err := json.Unmarshal(raw, &servers); err != nil {
			return nil, nil, fmt.Errorf("mcpServers가 객체가 아닙니다: %w", err)
		}
	}
	return root, servers, nil
}

func claudeConfigMatches(current []byte, binaryPath string) bool {
	_, servers, err := parseClaudeConfig(current)
	if err != nil {
		return false
	}
	var entry struct {
		Command string   `json:"command"`
		Args    []string `json:"args"`
	}
	if err := json.Unmarshal(servers[serverName], &entry); err != nil {
		return false
	}
	return entry.Command == binaryPath && len(entry.Args) == 2 && entry.Args[0] == "-transport" && entry.Args[1] == "stdio"
}

func mergeCodexConfig(current []byte, binaryPath string) []byte {
	base := strings.TrimSpace(string(removeCodexConfig(current)))
	block := fmt.Sprintf("%s\n[mcp_servers.%s]\ncommand = %s\nargs = [\"-transport\", \"stdio\"]\n%s", managedStart, codexServerName, quoteTOML(binaryPath), managedEnd)
	if base == "" {
		return []byte(block + "\n")
	}
	return []byte(base + "\n\n" + block + "\n")
}

func removeCodexConfig(current []byte) []byte {
	lines := strings.Split(strings.ReplaceAll(string(current), "\r\n", "\n"), "\n")
	result := make([]string, 0, len(lines))
	managed := false
	legacy := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == managedStart {
			managed = true
			continue
		}
		if managed {
			if trimmed == managedEnd {
				managed = false
			}
			continue
		}
		if isCodexServerHeader(trimmed) {
			legacy = true
			continue
		}
		if legacy && strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			if isCodexServerChildHeader(trimmed) {
				continue
			}
			legacy = false
		}
		if !legacy {
			result = append(result, line)
		}
	}
	return []byte(strings.TrimSpace(strings.Join(result, "\n")) + "\n")
}

func isCodexServerHeader(line string) bool {
	return line == "[mcp_servers.document_workspace]" || line == "[mcp_servers.\"document-workspace\"]"
}

func isCodexServerChildHeader(line string) bool {
	return strings.HasPrefix(line, "[mcp_servers.document_workspace.") || strings.HasPrefix(line, "[mcp_servers.\"document-workspace\".")
}

func validateCodexManagedBlock(current []byte) error {
	text := strings.ReplaceAll(string(current), "\r\n", "\n")
	startCount := strings.Count(text, managedStart)
	endCount := strings.Count(text, managedEnd)
	if startCount != endCount || startCount > 1 {
		return fmt.Errorf("Codex 설정의 document-workspace 관리 구간이 손상되어 변경하지 않았습니다")
	}
	if startCount == 1 && strings.Index(text, managedStart) > strings.Index(text, managedEnd) {
		return fmt.Errorf("Codex 설정의 document-workspace 관리 구간 순서가 올바르지 않아 변경하지 않았습니다")
	}
	return nil
}

func codexConfigMatches(current []byte, binaryPath string) bool {
	want := fmt.Sprintf("%s\n[mcp_servers.%s]\ncommand = %s\nargs = [\"-transport\", \"stdio\"]\n%s", managedStart, codexServerName, quoteTOML(binaryPath), managedEnd)
	return strings.Contains(strings.ReplaceAll(string(current), "\r\n", "\n"), want)
}

func quoteTOML(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func readOptionalFile(path string) ([]byte, error) {
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("설정 읽기: %w", err)
	}
	return content, nil
}

func writeChangedConfig(path string, current, next []byte) error {
	if bytes.Equal(bytes.TrimSpace(current), bytes.TrimSpace(next)) {
		return nil
	}
	if len(current) != 0 {
		if err := writeFileAtomic(path+".document-workspace.bak", current, 0o600); err != nil {
			return fmt.Errorf("기존 설정 백업: %w", err)
		}
	}
	return writeFileAtomic(path, append(bytes.TrimRight(next, "\n"), '\n'), 0o600)
}

func copyExecutable(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".document-workspace-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(installedBinMode); err != nil {
		temporary.Close()
		return err
	}
	if _, err := io.Copy(temporary, input); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, destination)
}

func writeFileAtomic(path string, content []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".document-workspace-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(mode); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(content); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func isRegularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func fileEquals(path string, expected []byte) bool {
	content, err := os.ReadFile(path)
	return err == nil && bytes.Equal(content, expected)
}
