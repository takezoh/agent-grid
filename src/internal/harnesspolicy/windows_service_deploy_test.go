package harnesspolicy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWindowsServiceRegistrationAssets(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	script := readRegistrationAsset(t, filepath.Join(root, "deploy", "windows", "register-wsl-host-service.ps1"))
	guide := readRegistrationAsset(t, filepath.Join(root, "docs", "note", "note-20260726-user-windows-wsl-service.md"))

	for _, required := range []string{
		`$ServiceName = "AgentGridWSLHost"`,
		`[ValidateSet("Install", "Update", "Uninstall")]`,
		`"-windows-identity", $Credential.UserName`,
		`"-distribution", $Distribution`,
		`Stop-AgentGridService`,
		`New-Service`,
		`-StartupType Automatic`,
	} {
		if !strings.Contains(script, required) {
			t.Errorf("registration script missing contract %q", required)
		}
	}
	for _, forbidden := range []string{"wsl.exe --shutdown", "LocalSystem"} {
		if strings.Contains(script, forbidden) {
			t.Errorf("registration script contains forbidden operation %q", forbidden)
		}
	}
	for _, required := range []string{
		"register-wsl-host-service.ps1",
		"-Action Install",
		"-Action Update",
		"-Action Uninstall",
		"%ProgramData%\\AgentGrid\\logs\\wsl-host.log",
	} {
		if !strings.Contains(guide, required) {
			t.Errorf("user guide missing %q", required)
		}
	}
}

func readRegistrationAsset(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
