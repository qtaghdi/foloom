package hostinstall

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallStatusAndUninstallPreserveOtherHostSettings(t *testing.T) {
	home := t.TempDir()
	source := filepath.Join(t.TempDir(), "document-workspace")
	if err := os.WriteFile(source, []byte("test executable"), 0o755); err != nil {
		t.Fatal(err)
	}
	installer := &Installer{Home: home, SourceExecutable: source, GOOS: "darwin"}

	claudePath := installer.claudeConfigPath()
	writeTestFile(t, claudePath, []byte(`{
  "theme": "dark",
  "mcpServers": {
    "other": {"command": "/other"}
  }
}`))
	codexPath := installer.codexConfigPath()
	writeTestFile(t, codexPath, []byte("model = \"gpt-test\"\n\n[mcp_servers.other]\ncommand = \"/other\"\n"))

	status, err := installer.Install()
	if err != nil {
		t.Fatal(err)
	}
	assertFullyInstalled(t, status)
	installedBinary, err := os.ReadFile(status.BinaryPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(installedBinary, []byte("test executable")) {
		t.Fatalf("installed binary = %q", installedBinary)
	}
	info, err := os.Stat(status.BinaryPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("binary mode = %o", info.Mode().Perm())
	}

	claude := readTestFile(t, claudePath)
	var claudeConfig map[string]json.RawMessage
	if err := json.Unmarshal(claude, &claudeConfig); err != nil {
		t.Fatal(err)
	}
	if string(claudeConfig["theme"]) != `"dark"` || !bytes.Contains(claudeConfig["mcpServers"], []byte(`"other"`)) {
		t.Fatalf("Claude config lost unrelated values: %s", claude)
	}
	codex := string(readTestFile(t, codexPath))
	for _, expected := range []string{`model = "gpt-test"`, "[mcp_servers.other]", managedStart, status.BinaryPath} {
		if !strings.Contains(codex, expected) {
			t.Fatalf("Codex config is missing %q: %s", expected, codex)
		}
	}

	second, err := installer.Install()
	if err != nil {
		t.Fatal(err)
	}
	assertFullyInstalled(t, second)
	if count := strings.Count(string(readTestFile(t, codexPath)), managedStart); count != 1 {
		t.Fatalf("managed Codex block count = %d", count)
	}

	removed, err := installer.Uninstall()
	if err != nil {
		t.Fatal(err)
	}
	if removed.Binary || removed.ClaudeMCP || removed.CodexMCP || removed.ClaudeSkill || removed.CodexSkill {
		t.Fatalf("status after uninstall = %#v", removed)
	}
	claude = readTestFile(t, claudePath)
	if bytes.Contains(claude, []byte(serverName)) || !bytes.Contains(claude, []byte(`"other"`)) {
		t.Fatalf("Claude uninstall changed unrelated server: %s", claude)
	}
	codex = string(readTestFile(t, codexPath))
	if strings.Contains(codex, managedStart) || !strings.Contains(codex, "[mcp_servers.other]") {
		t.Fatalf("Codex uninstall changed unrelated config: %s", codex)
	}
}

func TestInstallRejectsInvalidClaudeConfigBeforeWriting(t *testing.T) {
	home := t.TempDir()
	source := filepath.Join(t.TempDir(), "document-workspace")
	if err := os.WriteFile(source, []byte("test executable"), 0o755); err != nil {
		t.Fatal(err)
	}
	installer := &Installer{Home: home, SourceExecutable: source, GOOS: "darwin"}
	writeTestFile(t, installer.claudeConfigPath(), []byte(`{"mcpServers":`))

	if _, err := installer.Install(); err == nil || !strings.Contains(err.Error(), "기존 JSON") {
		t.Fatalf("Install error = %v", err)
	}
	if _, err := os.Stat(installer.binaryPath()); !os.IsNotExist(err) {
		t.Fatalf("binary was written before config validation: %v", err)
	}
}

func TestInstallRejectsMalformedManagedCodexBlockBeforeWriting(t *testing.T) {
	home := t.TempDir()
	source := filepath.Join(t.TempDir(), "document-workspace")
	if err := os.WriteFile(source, []byte("test executable"), 0o755); err != nil {
		t.Fatal(err)
	}
	installer := &Installer{Home: home, SourceExecutable: source, GOOS: "darwin"}
	writeTestFile(t, installer.codexConfigPath(), []byte(managedStart+"\n[mcp_servers.document_workspace]\n"))

	if _, err := installer.Install(); err == nil || !strings.Contains(err.Error(), "손상") {
		t.Fatalf("Install error = %v", err)
	}
	if _, err := os.Stat(installer.binaryPath()); !os.IsNotExist(err) {
		t.Fatalf("binary was written before config validation: %v", err)
	}
}

func TestRepositorySkillMatchesInstalledSkill(t *testing.T) {
	path := filepath.Join("..", "..", ".agents", "skills", "document-xlsx", "SKILL.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(content, documentXLSXSkill) {
		t.Fatal("repository and installed document-xlsx skills differ")
	}
}

func TestInstallMigratesLegacyCodexSection(t *testing.T) {
	home := t.TempDir()
	source := filepath.Join(t.TempDir(), "document-workspace")
	if err := os.WriteFile(source, []byte("test executable"), 0o755); err != nil {
		t.Fatal(err)
	}
	installer := &Installer{Home: home, SourceExecutable: source, GOOS: "darwin"}
	writeTestFile(t, installer.codexConfigPath(), []byte("[mcp_servers.document_workspace]\ncommand = \"/old\"\nargs = [\"-file\", \"/old.xlsx\"]\n\n[mcp_servers.document_workspace.env]\nOLD = \"value\"\n\n[features]\napps = true\n"))

	if _, err := installer.Install(); err != nil {
		t.Fatal(err)
	}
	content := string(readTestFile(t, installer.codexConfigPath()))
	if strings.Contains(content, "/old") || strings.Contains(content, "OLD") || strings.Count(content, "[mcp_servers.document_workspace]") != 1 || !strings.Contains(content, "[features]") {
		t.Fatalf("legacy Codex config migration = %s", content)
	}
}

func TestInstallerRejectsUnsupportedOperatingSystem(t *testing.T) {
	installer := &Installer{Home: t.TempDir(), SourceExecutable: "/tmp/document-workspace", GOOS: "linux"}
	if _, err := installer.Status(); err == nil || !strings.Contains(err.Error(), "macOS") {
		t.Fatalf("Status error = %v", err)
	}
}

func assertFullyInstalled(t *testing.T, status Status) {
	t.Helper()
	if !status.Binary || !status.ClaudeMCP || !status.CodexMCP || !status.ClaudeSkill || !status.CodexSkill {
		t.Fatalf("installation status = %#v", status)
	}
}

func writeTestFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
}

func readTestFile(t *testing.T, path string) []byte {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return content
}
