---
id: adr-20260705-fakedocker-path-injection
kind: adr
title: 'fakedocker: PATH-injected docker CLI fake with real-docker backstop'
status: accepted
created: '2026-07-05'
decision_makers:
- Takehito Gondo
tags:
- testing
- sandbox
owners: []
relations:
- {type: partOf, target: plan-20260705-test-harness}
- {type: references, target: adr-20260704-cli-fake-validated-by-real-cli-e2e}
- {type: referencedBy, target: note-20260624-agent-testing}
- {type: referencedBy, target: note-20260624-technical-code-enforcement}
- {type: references, target: note-20260624-agent-testing}
- {type: references, target: note-20260624-technical-code-enforcement}
source_paths:
- src/platform/sandbox/devcontainer/
- scripts/coverage-floors.txt
summary: devcontainer lifecycle を PATH 注入 fake docker で常時テストし、opt-in real docker e2e
  で fake 忠実性を保証する (ADR-20260704 パターンの docker 適用)
updated: '2026-07-05'
---

# fakedocker: PATH-injected docker CLI fake with real-docker backstop

## Context

{% context %}
外部プラットフォーム依存 4 種 (claude / codex / pty / docker) のうち docker だけが fake も real e2e も
持たない。`platform/sandbox/devcontainer` は `exec.CommandContext(ctx, "docker", …)` を直接呼び、テストは
純粋な parse / translate ロジックに限定されている。coverage-floors.txt にも「requires Docker …
structurally untestable in unit tests」と明記され、lifecycle (create → hook → run → remove) の
協調は保証の空白になっている。testing note の「When Coverage Can't Be Reached」は「Integration tests,
not coverage adjustments, are the answer」と結論している。
{% /context %}

## Decision

{% decision %}
adr-20260704-cli-fake-validated-by-real-cli-e2e のパターンを docker に適用することにする:

1. **fakedocker (T1)** — `platform/sandbox/devcontainer/fakedocker` に PATH 注入型の fake `docker`
   実行ファイル (テスト時に build する小さな Go binary) を新設する。argv を記録し、`ps` / `inspect` /
   `run` / `exec` / `rm` に canned 応答を返す。devcontainer manager のテストは `PATH` 先頭に fakedocker
   を置いて lifecycle 全体を常時駆動する。
2. **real-docker backstop (T3)** — `AG_E2E_DOCKER_BIN` (未設定なら Skip) で同一 lifecycle
   シナリオを real docker に流し、fakedocker の canned 応答が real docker の出力形と一致することを
   `FakeVsRealDocker` で照合する。`//go:build e2e`。
3. exec 呼び出しを Runner interface に抽象化する改修は行わない — 検証したいのは「実際の argv 組み立てと
   プロセス起動を含む lifecycle」であり、PATH 注入はそれを production コード無改変で駆動できる。
{% /decision %}

## Consequences

{% consequence kind="positive" %}
devcontainer の「structurally untestable」が解消され、coverage floor を引き上げられる。docker CLI の
出力形変更 (docker バージョンアップ) が T3 backstop で検出される。
{% /consequence %}

{% consequence kind="negative" %}
fake binary の build がテスト実行に追加され、テスト起動が僅かに遅くなる。canned 応答の保守という fake 一般の
負債を新たに 1 つ抱える (backstop がその負債の保険)。
{% /consequence %}

## Alternatives

- **Runner interface (lib/github.Runner 方式) への抽象化** — 却下。interface 化は argv 組み立てより
  上流で切ってしまい、「docker への実引数と応答 parse」という検証したい層が fake の外に出る。PATH 注入は
  exec 境界そのものを検証対象に含む。
- **testcontainers 等による real docker 常時テスト** — 却下。CI 環境に docker daemon を常時要求し、
  T1 の hermetic 性 (NFR-002) を壊す。real docker は T3 opt-in に隔離する。
- **現状維持 (parse のみ)** — 却下。lifecycle 協調の regress (引数順・翻訳・失敗処理) が本番でしか
  発覚しない。


{% transition from="proposed" to="accepted" date="2026-07-05" %}
fakedocker lifecycle T1 + AG_E2E_DOCKER_BIN backstop 実装済み
{% /transition %}
