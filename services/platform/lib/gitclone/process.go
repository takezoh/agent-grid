package gitclone

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/takezoh/agent-grid/services/platform/procgroup"
)

type CloneRequest struct {
	Owner, Name, DestDir, GitBin string
	Cred                         Credential
	Timeout                      time.Duration
	Runner                       ProcessRunner
}
type CloneResult struct {
	Code, Diagnostic, Remote string
	Argv, Env                []string
}
type ProcessRunner interface {
	Run(ctx context.Context, bin string, argv, env []string, extra []*os.File) ([]byte, []byte, int, error)
}
type ScriptedRunner struct {
	Argv, Env []string
	Code      int
	Err       error
	Out, Errb []byte
}

func (s *ScriptedRunner) Run(_ context.Context, _ string, argv, env []string, _ []*os.File) ([]byte, []byte, int, error) {
	s.Argv, s.Env = append([]string(nil), argv...), append([]string(nil), env...)
	return s.Out, s.Errb, s.Code, s.Err
}

type Cloner interface {
	Clone(ctx context.Context, req CloneRequest) CloneResult
}
type Service struct{}

func (Service) Clone(ctx context.Context, req CloneRequest) CloneResult {
	remote, err := CanonicalHTTPSRemote(req.Owner, req.Name)
	if err != nil || req.DestDir == "" {
		return CloneResult{Code: CodeInvalidIdentity, Remote: remote}
	}
	if req.Cred.Token == "" {
		return CloneResult{Code: CodeAuthorityUnavailable, Remote: remote}
	}
	bin, timeout := req.GitBin, req.Timeout
	if bin == "" {
		bin = "git"
	}
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	broker, prep, err := StartBroker(req.Cred)
	if err != nil {
		code := CodeCloneFailed
		if strings.Contains(err.Error(), CodeAuthorityUnavailable) {
			code = CodeAuthorityUnavailable
		}
		return CloneResult{Code: code, Diagnostic: "broker_start_failed", Remote: remote}
	}
	defer prep.Cleanup()
	env := append([]string{"PATH=" + os.Getenv("PATH"), "HOME=" + prep.Dir, "XDG_CONFIG_HOME=" + prep.Dir}, prep.Env...)
	argv := []string{"clone", "--", remote, req.DestDir}
	_ = os.MkdirAll(filepath.Dir(req.DestDir), 0o755)
	var stdout, stderr []byte
	var code int
	var runErr error
	if req.Runner != nil {
		stdout, stderr, code, runErr = req.Runner.Run(cctx, bin, argv, env, prep.ExtraFiles)
	} else {
		stdout, stderr, code, runErr = runGit(cctx, broker, prep, bin, argv, env)
	}
	surfaces := []string{remote, strings.Join(argv, " "), strings.Join(env, "\n"), string(stdout), string(stderr)}
	if ContainsSentinel(surfaces, req.Cred.Token) {
		return CloneResult{Code: CodeCloneFailed, Diagnostic: CodeCredentialSurfaceLeak, Remote: remote, Argv: argv, Env: env}
	}
	diag := SanitizeDiagnostic(string(stderr)+"\n"+string(stdout), req.Cred.Token)
	res := CloneResult{Diagnostic: diag, Remote: remote, Argv: argv, Env: env}
	if cctx.Err() != nil {
		res.Code = CodeCancelled
		return res
	}
	if runErr != nil || code != 0 {
		if c := broker.LastCode(); c == CodePeerMismatch || c == CodeReplay {
			res.Code = c
			return res
		}
		res.Code = CodeCloneFailed
		return res
	}
	res.Code = CodeOK
	return res
}
func runGit(ctx context.Context, broker *Broker, prep *BrokerPrepare, bin string, argv, env []string) ([]byte, []byte, int, error) {
	cmd := procgroup.Command(procgroup.Spec{Ctx: ctx, Bin: bin, Args: argv, Env: env})
	cmd.ExtraFiles = prep.ExtraFiles
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &outBuf, &errBuf
	if err := cmd.Start(); err != nil {
		return nil, nil, -1, err
	}
	broker.BindGit(cmd.Process.Pid, cmd.Process.Pid)
	err := cmd.Wait()
	code := 0
	if err != nil {
		code = 1
		// Avoid importing os/exec (external process boundary); ExitCode is on the
		// concrete process error from the shared procgroup seam.
		type exitCoder interface{ ExitCode() int }
		if ee, ok := err.(exitCoder); ok {
			code = ee.ExitCode()
		}
	}
	return outBuf.Bytes(), errBuf.Bytes(), code, err
}

type FakeGitClone struct {
	Calls    []CloneRequest
	Result   CloneResult
	Hook     func(CloneRequest) CloneResult
	Captured struct{ Argv, Env, Surfaces []string }
}

func (f *FakeGitClone) Clone(_ context.Context, req CloneRequest) CloneResult {
	f.Calls = append(f.Calls, CloneRequest{Owner: req.Owner, Name: req.Name, DestDir: req.DestDir})
	remote, err := CanonicalHTTPSRemote(req.Owner, req.Name)
	if err != nil {
		return CloneResult{Code: CodeInvalidIdentity}
	}
	if req.Cred.Token == "" {
		return CloneResult{Code: CodeAuthorityUnavailable, Remote: remote}
	}
	res := f.Result
	if f.Hook != nil {
		res = f.Hook(req)
	}
	if res.Code == "" {
		res.Code = CodeOK
	}
	res.Remote, res.Argv = remote, []string{"clone", "--", remote, req.DestDir}
	res.Env = []string{"GIT_TERMINAL_PROMPT=0", EnvAskpassSock + "=/tmp/fake.sock"}
	if res.Code == CodeOK && req.DestDir != "" {
		_ = os.MkdirAll(filepath.Join(req.DestDir, ".git"), 0o755)
	}
	f.Captured.Argv, f.Captured.Env = res.Argv, res.Env
	f.Captured.Surfaces = []string{remote, strings.Join(res.Argv, " "), strings.Join(res.Env, "\n"), res.Diagnostic}
	if ContainsSentinel(f.Captured.Surfaces, req.Cred.Token) {
		res.Code, res.Diagnostic = CodeCloneFailed, CodeCredentialSurfaceLeak
	}
	return res
}
