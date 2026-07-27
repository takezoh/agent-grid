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
		`Get-Credential -UserName $env:USERNAME`,
		"whoami",
		`$env:COMPUTERNAME\$($script:Credential.UserName)`,
		"function Grant-ServiceLogonRight",
		"SeServiceLogonRight",
		"secedit.exe",
		"Grant-ServiceLogonRight -AccountName $Credential.UserName",
		`Get-Content -LiteralPath $cfgPath -Encoding Unicode`,
		`Set-Content -LiteralPath $cfgPath -Value $lines -Encoding Unicode`,
		`throw "secedit export failed with exit code ${LASTEXITCODE}: $exportOutput"`,
		`throw "secedit configure failed with exit code ${LASTEXITCODE}: $configureOutput"`,
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

func TestWindowsServiceLauncherBat(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	launcher := readRegistrationAsset(t, filepath.Join(root, "deploy", "windows", "register-wsl-host-service.bat"))
	guide := readRegistrationAsset(t, filepath.Join(root, "docs", "note", "note-20260726-user-windows-wsl-service.md"))

	for _, required := range []string{
		`register-wsl-host-service.ps1'`,
		`-Verb RunAs -Wait`,
		`%*`,
		`Start-Transcript`,
		`register-wsl-host-service-elevate.log`,
	} {
		if !strings.Contains(launcher, required) {
			t.Errorf("launcher script missing contract %q", required)
		}
	}
	for _, forbidden := range []string{"ShellExecute", "Shell.Application"} {
		if strings.Contains(launcher, forbidden) {
			t.Errorf("launcher script must not use fire-and-forget elevation (%q) -- it reports success before the elevated process (UAC prompt, Get-Credential) has actually finished; use Start-Process -Verb RunAs -Wait instead", forbidden)
		}
	}
	for _, required := range []string{
		"register-wsl-host-service.bat",
		"ダブルクリック",
		"管理者として実行",
	} {
		if !strings.Contains(guide, required) {
			t.Errorf("user guide missing %q (Explorer launch has no way to pass arguments to the .bat)", required)
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
