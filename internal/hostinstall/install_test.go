package hostinstall

import (
	"bytes"
	"encoding/json"
	"fmt"
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

	legacySkill := filepath.Join(home, ".codex", "skills", "document-xlsx", "SKILL.md")
	writeTestFile(t, legacySkill, []byte("user's XLSX skill"))
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

	writeTestFile(t, installer.codexSkillPath(), []byte("custom workspace skill"))
	removed, err := installer.Uninstall()
	if err != nil {
		t.Fatal(err)
	}
	if removed.Binary || removed.ClaudeMCP || removed.CodexMCP || removed.ClaudeSkill || removed.CodexSkill {
		t.Fatalf("status after uninstall = %#v", removed)
	}
	if got := string(readTestFile(t, installer.codexSkillPath())); got != "custom workspace skill" {
		t.Fatalf("custom workspace skill was changed: %q", got)
	}
	if got := string(readTestFile(t, legacySkill)); got != "user's XLSX skill" {
		t.Fatalf("legacy skill was changed: %q", got)
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
	path := filepath.Join("..", "..", ".agents", "skills", "foloom", "SKILL.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(content, foloomSkill) {
		t.Fatal("repository and installed document-workspace skills differ")
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
	if strings.Contains(content, "/old") || strings.Contains(content, "OLD") || strings.Count(content, "[mcp_servers.foloom]") != 1 || !strings.Contains(content, "[features]") {
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

func TestFoloomMigrationPreservesUserFilesAndReplacesLegacyRegistration(t *testing.T) {
	for _, customized := range []bool{false, true} {
		t.Run(fmt.Sprint(customized), func(t *testing.T) {
			source := filepath.Join(t.TempDir(), "foloom")
			writeTestFile(t, source, []byte("foloom executable"))
			i := &Installer{Home: t.TempDir(), SourceExecutable: source, GOOS: "darwin"}
			writeTestFile(t, i.claudeConfigPath(), []byte(`{"mcpServers":{"document-workspace":{"command":"/old"},"other":{"command":"/keep"}},"theme":"dark"}`))
			writeTestFile(t, i.codexConfigPath(), []byte("model = \"keep\"\n"+legacyStart+"\n[mcp_servers.document_workspace]\ncommand = \"/old\"\n"+legacyEnd+"\n[mcp_servers.other]\ncommand = \"/keep\"\n"))
			oldPath := filepath.Join(i.Home, ".codex", "skills", "document-workspace", "SKILL.md")
			oldContent := legacySkill
			if customized {
				oldContent = append(append([]byte{}, legacySkill...), []byte("\nUser workflow notes\n")...)
			}
			writeTestFile(t, oldPath, oldContent)
			status, err := i.Install()
			if err != nil {
				t.Fatal(err)
			}
			assertFullyInstalled(t, status)
			_, servers, err := parseClaudeConfig(readTestFile(t, i.claudeConfigPath()))
			if err != nil {
				t.Fatal(err)
			}
			if _, exists := servers["document-workspace"]; exists {
				t.Fatal("legacy Claude server remains active")
			}
			if !bytes.Contains(servers["other"], []byte("/keep")) {
				t.Fatal("unrelated Claude server changed")
			}
			codex := string(readTestFile(t, i.codexConfigPath()))
			if strings.Contains(codex, "/old") || strings.Contains(codex, legacyStart) || !strings.Contains(codex, "[mcp_servers.foloom]") || !strings.Contains(codex, "/keep") {
				t.Fatalf("migration = %s", codex)
			}
			if !bytes.Contains(readTestFile(t, i.codexConfigPath()+".document-workspace.bak"), []byte("/old")) {
				t.Fatal("previous config was not backed up")
			}
			if customized {
				if !bytes.Equal(readTestFile(t, oldPath), oldContent) {
					t.Fatal("custom legacy skill changed")
				}
			} else if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
				t.Fatalf("managed legacy skill remains: %v", err)
			}
			if _, err := i.Install(); err != nil {
				t.Fatalf("reinstall: %v", err)
			}
		})
	}
}

func TestFoloomMigrationRejectsBrokenLegacyBlockBeforeAnyWrite(t *testing.T) {
	for _, block := range []string{legacyStart, legacyEnd, legacyStart + "\n" + managedEnd, legacyStart + "\n" + managedStart + "\n" + managedEnd + "\n" + legacyEnd} {
		i := &Installer{Home: t.TempDir(), SourceExecutable: "/tmp/foloom", GOOS: "darwin"}
		writeTestFile(t, i.codexConfigPath(), []byte(block))
		if _, err := i.Install(); err == nil {
			t.Fatalf("accepted malformed block %q", block)
		}
		if _, err := os.Stat(i.binaryPath()); !os.IsNotExist(err) {
			t.Fatal("binary was installed before validation")
		}
		if got := string(readTestFile(t, i.codexConfigPath())); got != block {
			t.Fatal("malformed config changed")
		}
	}
}

func TestFoloomInstallDoesNotOverwriteCustomCanonicalSkill(t *testing.T) {
	source := filepath.Join(t.TempDir(), "foloom")
	writeTestFile(t, source, []byte("executable"))
	i := &Installer{Home: t.TempDir(), SourceExecutable: source, GOOS: "darwin"}
	writeTestFile(t, i.codexSkillPath(), []byte("user custom instructions"))
	if _, err := i.Install(); err == nil {
		t.Fatal("custom skill was not protected")
	}
	if got := string(readTestFile(t, i.codexSkillPath())); got != "user custom instructions" {
		t.Fatal("custom skill changed")
	}
	if _, err := os.Stat(i.binaryPath()); !os.IsNotExist(err) {
		t.Fatal("binary was installed before validation")
	}
}
