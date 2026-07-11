---
id: plan-20260711-frame-exec-launcher
kind: plan
title: Frame launch sequencing owned by an in-container Go launcher — implementation plan
status: draft
created: '2026-07-11'
goal: shell composition を daemon 境界から排除し、container 内 Go 単一 owner (`bridge frame-exec`) で devcontainer preExec 評価 → pre-command 逐次実行 → main への syscall.Exec を行うことで、commit 28ad8999 と同型の shell semantics 回帰を構造的に不可能にする
scope_in:
- container 内 `bridge frame-exec` subcommand の新設 (preExec 評価 + pre-command 逐次実行 + main への syscall.Exec + timeout + T0/T2 test)
- `state.LaunchPlan` / `agentlaunch.LaunchPlan` / `sandbox.LaunchSpec` の argv-first 拡張 (`Argv []string` + `PreCommands [][]string` + `PreExec string` + `LoginShell string` + `PreCommandTimeout time.Duration`)
- devcontainer 経路 (`BuildLaunchCommand`) を `docker exec ... bridge frame-exec` (shell wrap 無し) に置換
- codex driver (backend.go) の trust step を PreCommands 経路 + Argv 経路に移行
- host DirectLauncher に frame-exec 経路を敷設
- 案 B'' の shell fragment 合成コード (`envelope.go` 丸ごと) と関連 test の廃棄
scope_out:
- claude / gemini / shell driver の全面 argv 化 (別 spec)
- pre-command spec の secret transport (NFR-005 で evolvability だけ確保)
- 現行 `Command string` の廃止 (別 PR で扱う)
- preExec 契約を shell fragment から env delta に変更する ADR (ADR-0082 Alt C で却下)
relations:
- {type: partOf, target: spec-20260711-frame-exec-launcher}
- {type: partOf, target: adr-20260711-0082-frame-exec-launcher}
- {type: partOf, target: adr-20260711-0083-launchplan-argv-primary}
- {type: partOf, target: adr-20260711-0084-frame-spec-transport}
milestones:
- id: m1
  title: bridge frame-exec + LaunchPlan argv 化 + devcontainer 経路移行 + 案 B'' 削除
  status: draft
---

## Goal

Codex Driver 起動不可 (commit 28ad8999 が起因) の hotfix 案 B'' を破棄し、案 B''' (in-container Go launcher による preExec + pre-command + main の統合 sequencing 所有) を 1 セッションで land する。

## Non-goals

- claude / gemini / shell driver の全面 argv 化
- secret を扱う pre-command 経路の実装
- devcontainer 経路以外の sandbox backend
- preExec 契約の env delta 化

## Preconditions (次セッションの clean state)

**working tree は clean (案 B'' 削除済)**。本 plan land 前に、前 session が以下を実施済:

```
cd /home/dev/dev/agent-grid
git checkout -- src/client/runtime/launcher.go \
                src/client/runtime/subsystem/stream/backend.go \
                src/client/runtime/subsystem/stream/launch_flow_test.go \
                src/client/state/driver_iface.go \
                src/platform/agentlaunch/devcontainer.go \
                src/platform/agentlaunch/types.go \
                src/platform/sandbox/devcontainer/manager.go \
                src/platform/sandbox/devcontainer/manager_test.go \
                src/platform/sandbox/manager.go
rm -f src/platform/sandbox/devcontainer/envelope.go
cd src && go build ./... && go vet ./...
```

これを実施した状態から chunks C-1 → C-6 を順に land する。

## Implementation Entry Points

次セッションが着手する順に、各 chunk で **どこに手を入れるか** を pointer 化する。

### chunk C-1: `frame-exec` の実装 (shared package + bridge/daemon 両方から dispatch)

**OQ-1 / OQ-3 resolution**: FrameExec 本体は shared package (`platform/framelaunch/`) に切り出し、container 側の `bridge` binary と host 側の daemon binary の両方が subcommand として呼ぶ。sandbox 機構 (container / host) だけが caller 側で異なり、sequencing 実装は共通。

LoginShell resolution は bridge/daemon binary 内 (framelaunch package) で自己解決する — `os/user` + `/etc/passwd` の SHELL フィールドを読む。FrameSpec.LoginShell は optional override として残し、daemon は基本空文字を送る (bridge が自己解決)。将来 caller が明示的な shell を指定したくなったら field 経由で override 可能。

**新規 file** `src/platform/framelaunch/frame_exec.go`:

```go
// Package framelaunch owns the "run pre-conditions, then exec main" sequencing
// used by every frame launch (container-sandboxed via `bridge frame-exec` and
// host-direct via `<self-bin> frame-exec`). Sequencing lives here so both
// callers share exactly one Go implementation; only the sandbox harness (docker
// exec vs direct spawn) differs upstream. See adr-20260711-0082.
package framelaunch

import (
    "bytes"
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "log/slog"
    "os"
    "os/exec"
    "os/user"
    "strings"
    "syscall"
    "time"
)

// FrameSpec is transported via AG_FRAME_SPEC env var (JSON).
// See adr-20260711-0084 for the transport rationale.
type FrameSpec struct {
    PreExec           string     `json:"pre_exec,omitempty"`            // devcontainer.json preExecCommand
    LoginShell        string     `json:"login_shell,omitempty"`         // optional override; empty → resolve from /etc/passwd
    PreCommands       [][]string `json:"pre_commands,omitempty"`        // argv列 (all must exit 0 before main)
    MainCommand       []string   `json:"main_command"`                  // argv, required
    PreCommandTimeout string     `json:"pre_command_timeout,omitempty"` // Go time.Duration string; default 10s
}

// DefaultTimeout is the per-preCommand / preExec deadline when the FrameSpec
// does not override it. OQ-2 resolution: 10s balances typical sub-second
// pre-commands with container cold-start filesystem I/O outliers.
const DefaultTimeout = 10 * time.Second

// Package-var seams for T0 tests. Production paths use the syscall / os
// implementations; unit tests replace these with capturing fakes.
var (
    execReplacer func(argv0 string, argv []string, envv []string) error = syscall.Exec
    now                                                                  = time.Now
)

// Run is the shared entry point for `<bin> frame-exec`. It reads
// AG_FRAME_SPEC, evaluates PreExec (env capture), runs PreCommands
// sequentially, then syscall.Execs MainCommand. Returns a non-nil error
// only on setup / gate failure; on success it does not return (the process
// is replaced by MainCommand).
func Run() error {
    spec, err := loadFrameSpec(os.Getenv("AG_FRAME_SPEC"))
    if err != nil {
        return err
    }
    timeout := DefaultTimeout
    if spec.PreCommandTimeout != "" {
        if d, perr := time.ParseDuration(spec.PreCommandTimeout); perr == nil {
            timeout = d
        }
    }
    loginShell := spec.LoginShell
    if loginShell == "" {
        loginShell = resolveLoginShell()
    }

    if spec.PreExec != "" {
        env, err := capturePreExecEnv(loginShell, spec.PreExec, timeout)
        if err != nil {
            return fmt.Errorf("frame-exec: preExec eval: %w", err)
        }
        for k, v := range env {
            _ = os.Setenv(k, v)
        }
    }
    for i, pre := range spec.PreCommands {
        if err := runPreCommand(pre, timeout); err != nil {
            return fmt.Errorf("frame-exec: preCommand[%d] %v: %w", i, pre, err)
        }
    }
    if len(spec.MainCommand) == 0 {
        return errors.New("frame-exec: MainCommand is empty")
    }
    bin, err := exec.LookPath(spec.MainCommand[0])
    if err != nil {
        return fmt.Errorf("frame-exec: lookup main: %w", err)
    }
    return execReplacer(bin, spec.MainCommand, os.Environ())
}

// resolveLoginShell reads the current process user's login shell from
// /etc/passwd, mirroring the existing envelope's `getent passwd | cut -d: -f7`
// behavior (so user zsh / bash / etc. dotfiles get sourced by preExec's shell).
// Falls back to /bin/sh if resolution fails.
func resolveLoginShell() string {
    u, err := user.Current()
    if err != nil {
        return "/bin/sh"
    }
    data, err := os.ReadFile("/etc/passwd")
    if err != nil {
        return "/bin/sh"
    }
    for _, line := range strings.Split(string(data), "\n") {
        // <user>:<passwd>:<uid>:<gid>:<gecos>:<home>:<shell>
        fields := strings.Split(line, ":")
        if len(fields) >= 7 && fields[0] == u.Username {
            if shell := strings.TrimSpace(fields[6]); shell != "" {
                return shell
            }
        }
    }
    return "/bin/sh"
}

func loadFrameSpec(raw string) (FrameSpec, error) {
    if raw == "" {
        return FrameSpec{}, errors.New("frame-exec: AG_FRAME_SPEC env is empty")
    }
    var s FrameSpec
    if err := json.Unmarshal([]byte(raw), &s); err != nil {
        return FrameSpec{}, fmt.Errorf("frame-exec: parse AG_FRAME_SPEC: %w", err)
    }
    return s, nil
}

// capturePreExecEnv runs `<loginShell> -lc '<preExec> && env -0'` and parses
// the NUL-delimited env dump. See FR-008.
func capturePreExecEnv(loginShell, preExec string, timeout time.Duration) (map[string]string, error) {
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()
    cmd := exec.CommandContext(ctx, loginShell, "-lc", preExec+" && env -0")
    var buf bytes.Buffer
    cmd.Stdout = &buf
    cmd.Stderr = os.Stderr
    if err := cmd.Run(); err != nil {
        return nil, err
    }
    return parseEnv0(buf.Bytes()), nil
}

func parseEnv0(b []byte) map[string]string {
    out := map[string]string{}
    for _, kv := range bytes.Split(b, []byte{0}) {
        if len(kv) == 0 {
            continue
        }
        if i := bytes.IndexByte(kv, '='); i > 0 {
            out[string(kv[:i])] = string(kv[i+1:])
        }
    }
    return out
}

// runPreCommand executes pre with per-command timeout, forwarding stdio.
// SIGTERM on ctx cancel, SIGKILL 5s later. Non-zero exit or timeout → error.
func runPreCommand(pre []string, timeout time.Duration) error {
    if len(pre) == 0 {
        return errors.New("empty preCommand argv")
    }
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()
    cmd := exec.CommandContext(ctx, pre[0], pre[1:]...)
    cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
    cmd.WaitDelay = 5 * time.Second
    if err := cmd.Run(); err != nil {
        slog.Warn("frame-exec: preCommand failed", "argv", pre, "err", err)
        return err
    }
    return nil
}
```

**Register `frame-exec` subcommand in TWO binaries** (両方から shared package を呼ぶ):

- **container 側**: `src/cmd/bridge/main.go` の subcommand dispatcher に:
  ```go
  case "frame-exec":
      if err := framelaunch.Run(); err != nil {
          fmt.Fprintln(os.Stderr, err)
          os.Exit(1)
      }
  ```
- **host 側**: daemon binary の main dispatcher に同じ 1 case を追加。実際の場所は `src/cmd/server/main.go` (agent-grid-server) の subcommand switch。既存の `event` / `hook-*` などの subcommand と同じ pattern で 1 case 追加

**T0 tests** in `src/platform/framelaunch/frame_exec_test.go`:
- `TestLoadFrameSpec_ErrorsOnEmpty` / `TestLoadFrameSpec_ParsesFullSpec`
- `TestParseEnv0_HandlesEmptyEntries` / `TestParseEnv0_HandlesEqualsInValue`
- `TestRun_MissingMainCommand` returns error
- `TestRun_AllPreCommandsSucceed_CallsExecReplacer` (execReplacer injected as a mock capturing argv/env)
- `TestRun_PreCommandFails_SkipsExecReplacer` (mock)
- `TestRun_PreCommandTimeout` (sleep script → context timeout → error, execReplacer not called)
- `TestRun_PreExecEnvIsApplied_BeforeExecReplacer` (fake shell writes FOO=bar\0 to stdout → assert os.Getenv("FOO") == "bar" at execReplacer call)
- `TestResolveLoginShell_ReadsEtcPasswd` (fake /etc/passwd read via injectable file read seam, or use current test env's user)

**T2 test** (contract, uses real subprocess):
- `TestRun_RealPtyIsInheritedByMain` — 実 pty で `<test-bin> frame-exec` を回し、child が isatty(0) == true を報告することを確認 (main を `sh -c 'test -t 0 && touch <marker>'` にして marker 確認)
- `TestCapturePreExecEnv_UsesActualShell` — 実 sh で `export FOO=bar; env -0` 相当を評価、FOO=bar を dump に含むことを確認

Acceptance:
```
cd src && go build ./... && go vet ./... && go test ./platform/framelaunch/... ./cmd/bridge/... ./cmd/server/...
```

### chunk C-2: LaunchPlan / LaunchSpec の argv-first 拡張

**`src/client/state/driver_iface.go`** — LaunchPlan に以下を追加 (`Command string` はそのまま legacy として残す):

```go
type LaunchPlan struct {
    // Command is the LEGACY simple-command shell string. New sandbox
    // consumers should use Argv + PreCommands. See adr-20260711-0083.
    Command string
    // Argv, when non-empty, is the argv passed directly to exec by the
    // in-container `bridge frame-exec` launcher. Mutually exclusive with
    // Command in sandbox mode (validated in sandbox.LaunchSpec boundary).
    Argv []string
    // PreCommands are argv列 run sequentially before Argv in the sandbox
    // envelope's `bridge frame-exec` launcher. Any non-zero exit aborts
    // the launch and Argv is not executed. Valid only when Argv is non-empty.
    PreCommands [][]string
    // PreCommandTimeout is per-command deadline (default 30s in the launcher).
    PreCommandTimeout time.Duration
    // ...existing fields unchanged...
    StartDir              string
    Project               string
    Sandbox               SandboxOverride
    Options               LaunchOptions
    Subsystem             LaunchSubsystem
    Stream                StreamLaunchOptions
    ManagedFrameMessaging bool
    Stdin                 []byte
}
```

**`src/platform/agentlaunch/types.go`** — 既存 `Argv []string` を再利用、`PreCommands` / `PreCommandTimeout` / `PreExec` / `LoginShell` を追加:

```go
type LaunchPlan struct {
    Command               string
    Argv                  []string
    PreCommands           [][]string
    PreExec               string        // devcontainer.json preExecCommand; forwarded to bridge frame-exec
    LoginShell            string        // absolute path; daemon resolves before dispatch
    PreCommandTimeout     time.Duration
    Env                   map[string]string
    StartDir              string
    Project               string
    ForceHost             bool
    ManagedFrameMessaging bool
}
```

**`src/platform/sandbox/manager.go`** — LaunchSpec に同じフィールドを追加:

```go
type LaunchSpec struct {
    Command            string      // legacy
    Argv               []string    // primary for frame-exec 経路
    PreCommands        [][]string
    PreExec            string
    LoginShell         string
    PreCommandTimeout  time.Duration
    StartDir           string
    TTY                bool
}
```

Boundary validation (`sandbox.LaunchSpec` consumer 内で行う。C-3 で `BuildLaunchCommand` 冒頭に実装):
- `len(Argv) > 0 && Command != ""` → error
- `len(Argv) == 0 && len(PreCommands) > 0` → error
- `len(Argv) == 0 && PreExec != ""` → error (PreExec は frame-exec 経路でのみ意味を持つ)

**FrameSpec JSON encoder** (`src/platform/agentlaunch/framespec.go` 新規):

DirectLauncher (C-5) と devcontainer.manager (C-3) の両方から呼ばれる single source of truth。`state.LaunchPlan` → `framelaunch.FrameSpec` の変換と JSON encoding を担う。

```go
package agentlaunch

import (
    "encoding/json"
    "github.com/takezoh/agent-grid/platform/framelaunch"
    "github.com/takezoh/agent-grid/platform/sandbox"
)

// EncodeFrameSpec builds an AG_FRAME_SPEC JSON payload from a LaunchPlan.
// LoginShell is intentionally left empty here — the framelaunch.Run consumer
// resolves it from /etc/passwd at runtime (see OQ-1 resolution).
func EncodeFrameSpec(plan LaunchPlan) (string, error) {
    spec := framelaunch.FrameSpec{
        PreExec:     plan.PreExec,
        LoginShell:  plan.LoginShell, // usually empty; caller override only
        PreCommands: plan.PreCommands,
        MainCommand: plan.Argv,
    }
    if plan.PreCommandTimeout > 0 {
        spec.PreCommandTimeout = plan.PreCommandTimeout.String()
    }
    b, err := json.Marshal(spec)
    if err != nil {
        return "", err
    }
    return string(b), nil
}

// EncodeFrameSpecFromLaunchSpec is the sandbox.LaunchSpec-oriented variant used
// by devcontainer.manager.BuildLaunchCommand. Same wire format as EncodeFrameSpec.
func EncodeFrameSpecFromLaunchSpec(spec sandbox.LaunchSpec) (string, error) {
    fs := framelaunch.FrameSpec{
        PreExec:     spec.PreExec,
        LoginShell:  spec.LoginShell,
        PreCommands: spec.PreCommands,
        MainCommand: spec.Argv,
    }
    if spec.PreCommandTimeout > 0 {
        fs.PreCommandTimeout = spec.PreCommandTimeout.String()
    }
    b, err := json.Marshal(fs)
    if err != nil {
        return "", err
    }
    return string(b), nil
}
```

Tests: `TestEncodeFrameSpec_RoundTrip`, `TestEncodeFrameSpec_EmptyPreExecOmittedFromJSON`。

Acceptance:
```
cd src && go build ./... && go vet ./...
```

### chunk C-3: devcontainer.manager.go を frame-exec 呼び出しに置換

**`src/platform/sandbox/devcontainer/manager.go`** の `BuildLaunchCommand` を、Argv 経路のときは shell wrap 無しに書き換える:

```go
func (m *Manager) BuildLaunchCommand(inst *sandbox.Instance[*ContainerState], spec sandbox.LaunchSpec, frameCtx sandbox.FrameContext, env map[string]string) (string, map[string]string, error) {
    // ... 既存の container ID / workdir 解決 ...

    if err := validateLaunchSpec(spec); err != nil { // C-2 の boundary validation
        return "", nil, err
    }

    // frame-exec 経路
    if len(spec.Argv) > 0 {
        // FrameSpec の JSON encoding は agentlaunch.EncodeFrameSpecFromLaunchSpec で
        // 一元化 (DirectLauncher と共有)。LoginShell は空文字を送り bridge 側で
        // /etc/passwd 経由の自己解決に委ねる (OQ-1 resolution)。
        specJSON, err := agentlaunch.EncodeFrameSpecFromLaunchSpec(spec)
        if err != nil {
            return "", nil, fmt.Errorf("devcontainer: encode FrameSpec: %w", err)
        }
        outEnv := cloneEnv(env)
        outEnv["AG_FRAME_SPEC"] = specJSON
        // docker exec argv は literal "bridge frame-exec" のみ (shell wrap 一切無し)
        return buildDockerExecCmd(containerID, spec, frameCtx, outEnv, "bridge frame-exec"), outEnv, nil
    }

    // legacy Command string 経路 (claude / gemini / shell driver 用) は現行のまま残す
    // ただし preConds / envelope 系のコードは全廃 (C-6 で)
    return buildLegacyCommand(spec, containerID, workDir, frameCtx, env)
}
```

**`src/platform/agentlaunch/devcontainer.go:100`** の LaunchSpec 構築で新フィールド全部 forward:

```go
cmd, outEnv, err := l.mgr.BuildLaunchCommand(inst, sandbox.LaunchSpec{
    Command:           plan.Command,
    Argv:              plan.Argv,
    PreCommands:       plan.PreCommands,
    PreExec:           plan.PreExec,
    LoginShell:        plan.LoginShell,
    PreCommandTimeout: plan.PreCommandTimeout,
    StartDir:          plan.StartDir,
    TTY:               l.tty,
}, frameCtx, plan.Env)
```

**devcontainer.Wrap の callsite** で PreExec を plan に埋める。現状 `cs.PreExec()` が `spec.PreExec` を返しているので、Wrap 内で `plan.PreExec = cs.PreExec()` を追加。

**LoginShell (OQ-1 resolution)**: **daemon 側の resolver は敷かない**。bridge (container 側) / daemon (host 側) の `framelaunch.Run()` が呼び出し時点で `os/user.Current()` + `/etc/passwd` を読んで自己解決する。daemon は FrameSpec.LoginShell を常に空文字で送り、caller 側で明示的に override したい場合 (test 等) のみ埋める。現行 `pre-exec.sh` (4 行、`mise trust --quiet` のみ) は shell 選択に非依存で verified。framelaunch の resolver が bash / zsh どちらを返しても現行 preExec は同じ動作。

Tests:
- `src/platform/sandbox/devcontainer/manager_test.go`
  - `TestBuildLaunchCommand_ArgvPath_DockerExecArgvContainsBridgeFrameExecOnly` — assertion: 生成された command 文字列に `sh`, `-c`, `-lc`, `&&`, `;` が **含まれない** こと、`bridge frame-exec` の literal が末尾に来ること
  - `TestBuildLaunchCommand_ArgvPath_SetsAgFrameSpecEnvWithJson` — outEnv["AG_FRAME_SPEC"] が有効な FrameSpec JSON であり、PreExec / MainCommand / PreCommands が正しく含まれること
  - `TestBuildLaunchCommand_LegacyCommandPath_UnchangedFromCurrent` — Argv 空 + Command 有り のとき従来通り

Acceptance:
```
cd src && go build ./... && go vet ./... && go test ./platform/sandbox/...
```

### chunk C-4: codex driver (backend.go) を Argv 経路に移行

**`src/client/runtime/subsystem/stream/backend.go:298`** 付近を書き換え:

```go
model, effort := b.bindingSettings(req.FrameID)
attachArgv := libcodex.RemoteAttachArgs(b.listenSock, persistedThreadID, startDir, model, effort)
result.Plan.Argv = attachArgv
result.Plan.Command = "" // legacy 経路を使わない
if b.sandboxed {
    result.Plan.PreCommands = append(result.Plan.PreCommands, []string{
        agentlaunch.ContainerBinaryPath, "codex-trust-project",
    })
}
```

**`src/client/runtime/subsystem/stream/launch_flow_test.go`** の既存 trust test を Argv / PreCommands assertion に書き換え:

```go
// Old assertion (case B''):
//   want := agentlaunch.ContainerBinaryPath + " codex-trust-project && exec " + wantAttach
//   if res.Plan.Command != want { … }
// New assertion:
wantArgv := libcodex.RemoteAttachArgs(listen, "", worktree, "", "")
if !reflect.DeepEqual(res.Plan.Argv, wantArgv) { t.Fatalf(...) }
wantPre := [][]string{{agentlaunch.ContainerBinaryPath, "codex-trust-project"}}
if !reflect.DeepEqual(res.Plan.PreCommands, wantPre) { t.Fatalf(...) }
if res.Plan.Command != "" { t.Errorf("legacy Command must be empty in argv path: %q", res.Plan.Command) }
```

Acceptance:
```
cd src && go build ./... && go vet ./... && go test ./client/runtime/subsystem/stream/...
```

### chunk C-5: DirectLauncher (host) を frame-exec 経路に統合

**OQ-3 resolution**: host launch 経路も frame-exec を通す。sandbox 機構 (container / host) だけが caller 側で異なり、sequencing 実装は完全に共通。C-1 で切り出した `platform/framelaunch` を daemon binary の subcommand として dispatch し、DirectLauncher が `<SelfBin> frame-exec` を spawn する。

**`src/client/runtime/launcher.go`** の `DirectLauncher.WrapLaunch` を以下に変更:

```go
func (d DirectLauncher) WrapLaunch(frameID state.FrameID, plan state.LaunchPlan, env map[string]string) (WrappedLaunch, error) {
    merged := stripContainerOnlyEnv(env, plan.ManagedFrameMessaging)
    var cleanup func() error
    if d.SockPath != "" {
        merged = cloneAndSet(merged, "AG_SOCKET", d.SockPath)
    }
    if plan.ManagedFrameMessaging {
        var rawCleanup func(context.Context) error
        var err error
        merged, rawCleanup, err = agentlaunch.PrepareManagedClaudeHome(string(frameID), d.SelfBin, d.SockPath, d.DataDir, merged)
        if err != nil {
            return WrappedLaunch{}, err
        }
        cleanup = adaptCleanup(rawCleanup)
    }

    // Argv 経路 (frame-exec 経由) と legacy Command 経路の disjoint 分岐
    if len(plan.Argv) > 0 {
        // Encode FrameSpec into AG_FRAME_SPEC env and spawn `<SelfBin> frame-exec`.
        // sandbox 機構が違うだけで sequencing 実装は container 側 bridge と共通。
        specJSON, err := agentlaunch.EncodeFrameSpec(plan) // C-1/C-2 で新設 helper
        if err != nil {
            return WrappedLaunch{}, fmt.Errorf("runtime: encode FrameSpec: %w", err)
        }
        merged = cloneAndSet(merged, "AG_FRAME_SPEC", specJSON)
        return WrappedLaunch{
            Command:  shellQuote(d.SelfBin) + " frame-exec",
            StartDir: plan.StartDir,
            Env:      merged,
            Cleanup:  cleanup,
        }, nil
    }

    // Legacy Command 経路: 現状の string Command を持つ driver (claude / gemini / shell) 用。
    // frame-exec を通さない (契約変更していない ため sandbox 機構だけ異なるという原則を維持)。
    return WrappedLaunch{
        Command:  plan.Command,
        StartDir: plan.StartDir,
        Env:      merged,
        Cleanup:  cleanup,
    }, nil
}
```

**agentlaunch.EncodeFrameSpec helper** (C-2 で新設): `state.LaunchPlan` から `framelaunch.FrameSpec` を組み立て JSON marshal する。docker exec 経路 (C-3) と DirectLauncher 経路 (C-5) の両方から呼ばれる single source of truth。

Tests: 
- `TestDirectLauncher_ArgvPath_SpawnsSelfBinFrameExecWithSpec` — Argv 経路で `merged["AG_FRAME_SPEC"]` が有効な JSON、Command が `<SelfBin> frame-exec`
- `TestDirectLauncher_LegacyCommandPath_Unchanged` — Argv 空のとき従来通り

Acceptance:
```
cd src && go build ./... && go vet ./... && go test ./client/runtime/...
```

### chunk C-6: 案 B'' 削除

Preconditions で reset 済みなら不要だが、念のため確認:

- `src/platform/sandbox/devcontainer/envelope.go` が **存在しないこと** (Preconditions で削除済み)
- `src/platform/sandbox/devcontainer/manager.go` から `PreConditions` 参照が消えていること (Preconditions で reset 済み)
- `src/client/state/driver_iface.go` / `src/platform/agentlaunch/types.go` / `src/platform/sandbox/manager.go` から `PreConditions []string` が消えていること (C-2 で `PreCommands [][]string` に置き換え済み)
- `src/platform/sandbox/devcontainer/manager_test.go` から `TestBuildEnvelopeFragment` / `TestBuildLaunchCommand_PreConditionsGateExec` / `TestValidateExecCommand_RejectsCompound` / `TestBuildLaunchCommand_RejectsCompoundCommand` が消えていること (Preconditions で reset 済み)

追加で:
- `src/platform/sandbox/devcontainer/manager.go` の legacy Command 経路から `<preExec>; exec <command>` の shell wrap を残す (claude / gemini / shell driver 用に必要)。これは案 B'' の shell composition **ではなく** commit 28ad8999 以前から在った envelope なので、削除しない
- `manager.go` に案 B'' で追加した `validateExecCommand` 呼び出しが残っていないことを確認 (Preconditions で reset 済み)

Acceptance (全体):
```
cd src && go build ./... && go vet ./... && go test ./... && \
    (cd /home/dev/dev/agent-grid && go tool -modfile=src/go.mod golangci-lint run ./src/...)
```

## Verification

### Tier T0 (pure Go unit)

- `bridge frame-exec` の spec parse / pre-command 逐次実行 / execReplacer 発火 / preExec env 適用 / timeout — すべて package-var seam (execReplacer, now) で injectable
- LaunchSpec boundary validation (Argv + Command 排他 / PreCommands は Argv 必須 / PreExec は Argv 必須) の table test
- `manager.go` `BuildLaunchCommand` の Argv path で生成される docker exec 文字列が `sh` を含まないことの grep-style assertion

### Tier T1 (wired, no external process)

- `runtime` + `fake sandbox launcher` で backend.go の Codex frame launch が Argv + PreCommands を正しく forward することを確認 (`launch_flow_test.go` 系列を拡張)

### Tier T2 (contract, uses real subprocess)

- `bridge frame-exec` を実プロセス spawn し、pre-command marker + main marker がファイルシステムに残ることを確認
- 実 pty を割り当てて `bridge frame-exec` を走らせ、syscall.Exec 後の main が isatty(0) == true を報告することを確認 (FR-003 verify)
- fake preExec script (`echo FOO=bar; env -0` 出力) を渡し、bridge が env FOO=bar を pre-command と main に伝搬させることを確認 (FR-008 verify)

### Tier T3 (fidelity — 出さない)

- 実 docker exec には依らない (別 e2e 化は本 plan の scope 外)

## Chunks summary

| chunk | file 変更概数 | acceptance | 依存 |
|---|---|---|---|
| C-1 | +2 file (`platform/framelaunch/frame_exec.go` + test) / +2 lines in cmd/bridge/main.go / +2 lines in cmd/server/main.go | build + test framelaunch + cmd/bridge + cmd/server | なし (単独 land 可) |
| C-2 | +4 file (state / agentlaunch types / sandbox の LaunchPlan / LaunchSpec + agentlaunch/framespec.go) | build ./... + test agentlaunch | C-1 (framelaunch.FrameSpec 参照のため) |
| C-3 | 2 file (manager.go / devcontainer.go) + test | build + test platform/sandbox | C-1, C-2 |
| C-4 | 2 file (backend.go / launch_flow_test.go) | build + test client/runtime/subsystem/stream | C-2, C-3 |
| C-5 | 1 file (launcher.go) + test | build + test client/runtime | C-1, C-2 |
| C-6 | (Preconditions 済なら no-op) | 全 build + test + lint | C-1〜C-5 |

各 chunk 完了時点で `go build ./...` + `go vet ./...` + `golangci-lint run` が pass することを acceptance に含める。

## Risks

- **R-1**: `frame-exec` subcommand を daemon binary (host) と bridge binary (container) の両方に register する必要 → C-1 で両者に同じ shared package (`platform/framelaunch`) を呼ばせることで実装乖離を防ぐ
- **R-2**: syscall.Exec の signal / session 継承挙動が試験環境で container と subtle に違う可能性 → C-1 の T2 contract test で実 pty attach を含めることで検出
- **R-3**: 案 B'' 破棄 → B''' land の期間中に daemon が再起動されないと、Codex Driver は動作不能のまま残る → 実装完了 + daemon restart までを 1 セッションで完了させる
- **R-4**: `env -0` が container の shell (`sh` / `bash` / `busybox`) で使えない可能性 → C-1 T2 test で実 sh 呼び出しを含めて事前検出。fallback として `/proc/self/exe --emit-env` 経由を用意する余地を framelaunch package の doc comment に残す
- **R-5**: framelaunch の login shell resolver が `/etc/passwd` の SHELL 列を読むが、host 側で `/etc/passwd` に user が居ない (LDAP / SSSD 環境) 可能性。resolver は fallback として `/bin/sh` を返す (defensive) → 実害は現行 preExec 契約 (shell 選択非依存) の下では無し

## Migration

案 B'' の working tree diff は本 plan の Preconditions で `git checkout -- <files>` + `rm envelope.go` により reset 済み。B'' の変更は commit されていないので history には残らない。

daemon 再起動: 本 plan 完了後 (chunks C-1〜C-6 land 後)、host 側の `agent-grid-server` プロセス (PID は現状 `pgrep -f agent-grid-server` で確認) を SIGTERM → 再起動して新 binary を適用する。Codex frame の起動不可症状はこれで消える。

## Resolved decisions

以下の 3 件は本 plan 確定時点で decision 済 (詳細と rationale は spec.md の FR / NFR、および ADR-0082 / 0083 を参照):

- **OQ-1 (LoginShell)**: framelaunch package が `/etc/passwd` の SHELL 列を自己解決。`FrameSpec.LoginShell` は optional override として残す。daemon は基本空文字を送る。現行 preExec は shell 選択非依存で verified
- **OQ-2 (PreCommandTimeout)**: default **10 秒**。framelaunch.DefaultTimeout 定数として定義。LaunchPlan.PreCommandTimeout > 0 のとき caller 側で override
- **OQ-3 (host DirectLauncher)**: host launch も frame-exec 経路。sequencing 実装は container / host で完全共通 (`platform/framelaunch` 経由)。sandbox 機構だけが caller 側で異なる (docker exec vs 直接 spawn)
