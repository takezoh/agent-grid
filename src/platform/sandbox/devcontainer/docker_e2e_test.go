//go:build e2e

package devcontainer

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/takezoh/agent-reactor/platform/sandbox/devcontainer/fakedocker"
)

func TestE2E_FakeVsRealDocker(t *testing.T) {
	realBin := e2eDockerBin(t)
	name := "reactor-e2e-" + time.Now().UTC().Format("20060102150405")
	createArgs := []string{
		"create",
		"--name", name,
		"--label", "reactor-managed=1",
		"--label", "reactor-mount-hash=fake-mount-hash",
		"alpine:3.20",
		"sh", "-c", "sleep 30",
	}
	out, err := exec.Command(realBin, createArgs...).CombinedOutput()
	if err != nil {
		t.Fatalf("docker create: %v\n%s", err, out)
	}
	containerID := stringTrim(out)
	t.Cleanup(func() {
		_, _ = exec.Command(realBin, "rm", "-f", containerID).CombinedOutput()
	})

	if out, err := exec.Command(realBin, "start", containerID).CombinedOutput(); err != nil {
		t.Fatalf("docker start: %v\n%s", err, out)
	}

	realPSLine, err := exec.Command(realBin, "ps", "-a",
		"--filter", "name="+name,
		"--format", psFormatFor(""),
	).CombinedOutput()
	if err != nil {
		t.Fatalf("docker ps: %v\n%s", err, realPSLine)
	}
	realInfo, err := parsePsLine(stringTrim(realPSLine))
	if err != nil {
		t.Fatalf("parse real ps: %v", err)
	}

	realInspectRaw, err := exec.Command(realBin, "inspect", containerID).CombinedOutput()
	if err != nil {
		t.Fatalf("docker inspect: %v\n%s", err, realInspectRaw)
	}
	realInspect := decodeInspectShape(t, realInspectRaw)

	fakeDir := t.TempDir()
	if err := fakedocker.WriteConfig(filepath.Join(fakeDir, "config.json"), fakedocker.Config{
		Responses: map[string][]fakedocker.Response{
			"ps":      {{Stdout: fakedocker.DefaultPSLine + "\n"}},
			"inspect": {{Stdout: fakedocker.DefaultInspectJSON}},
		},
	}); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}
	recordPath := filepath.Join(fakeDir, "records.jsonl")
	fakeBin := buildFakeDocker(t, filepath.Join(fakeDir, "docker"))
	fakeEnv := append(os.Environ(),
		fakedocker.EnvConfigPath+"="+filepath.Join(fakeDir, "config.json"),
		fakedocker.EnvRecordPath+"="+recordPath,
	)

	fakePSLine, err := commandOutput(fakeEnv, fakeBin, "ps", "-a", "--format", psFormatFor(""))
	if err != nil {
		t.Fatalf("fake docker ps: %v", err)
	}
	fakeInfo, err := parsePsLine(stringTrim(fakePSLine))
	if err != nil {
		t.Fatalf("parse fake ps: %v", err)
	}

	fakeInspectRaw, err := commandOutput(fakeEnv, fakeBin, "inspect", fakedocker.DefaultContainerID)
	if err != nil {
		t.Fatalf("fake docker inspect: %v", err)
	}
	fakeInspect := decodeInspectShape(t, fakeInspectRaw)

	if realInfo.ID == "" || realInfo.State == "" || realInfo.MountHash == "" {
		t.Fatalf("real ps line missing fields: %+v", realInfo)
	}
	if fakeInfo.ID == "" || fakeInfo.State == "" || fakeInfo.MountHash == "" {
		t.Fatalf("fake ps line missing fields: %+v", fakeInfo)
	}
	if realInspect[0].State.Status == "" || fakeInspect[0].State.Status == "" {
		t.Fatalf("inspect status missing: real=%+v fake=%+v", realInspect[0], fakeInspect[0])
	}
}

func e2eDockerBin(t *testing.T) string {
	t.Helper()
	bin := os.Getenv("REACTOR_E2E_DOCKER_BIN")
	if bin == "" {
		t.Skip("REACTOR_E2E_DOCKER_BIN is not set — skipping real-docker e2e")
	}
	if _, err := exec.LookPath(bin); err != nil {
		t.Skipf("REACTOR_E2E_DOCKER_BIN=%q is not executable: %v", bin, err)
	}
	return bin
}

type inspectShape []struct {
	ID    string `json:"Id"`
	State struct {
		Status string `json:"Status"`
	} `json:"State"`
}

func decodeInspectShape(t *testing.T, raw []byte) inspectShape {
	t.Helper()
	var out inspectShape
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode inspect json: %v\n%s", err, raw)
	}
	if len(out) == 0 {
		t.Fatalf("inspect returned no containers: %s", raw)
	}
	return out
}

func commandOutput(env []string, bin string, args ...string) ([]byte, error) {
	cmd := exec.Command(bin, args...)
	cmd.Env = env
	return cmd.CombinedOutput()
}

func stringTrim(b []byte) string {
	return string(bytesTrimSpace(b))
}

func bytesTrimSpace(b []byte) []byte {
	for len(b) > 0 && (b[0] == ' ' || b[0] == '\n' || b[0] == '\t' || b[0] == '\r') {
		b = b[1:]
	}
	for len(b) > 0 {
		last := b[len(b)-1]
		if last != ' ' && last != '\n' && last != '\t' && last != '\r' {
			return b
		}
		b = b[:len(b)-1]
	}
	return b
}
