---
id: adr-20260715-credproxy-fork-remote-replace
kind: adr
title: "ADR — credproxy: local dev via go.work (gitignored), CI via require pseudo-version to github.com/takezoh/credproxy"
status: accepted
created: '2026-07-15'
tags:
- adr
- credproxy
- go-mod
- fork-wiring
- migration-compatibility
owners:
- take.gn
relations:
- {type: partOf, target: plan-20260715-credproxy-materialization-contract}
- {type: references, target: spec-20260715-credproxy-materialization-contract}
- {type: references, target: adr-20260715-credproxy-materialize-method}
- {type: references, target: adr-20260715-credproxy-retry-owner-caller-side}
- {type: references, target: adr-20260715-credproxy-metadata-handler-async-materialize}
source_paths:
- src/go.mod
decision_makers:
- take.gn
summary: "credproxy fork の配線は 2 層。(1) local dev: repo root の gitignored な `go.work` に `./src` と `../credproxy` を並べ、contributor が local で iterate する。(2) CI/shared: src/go.mod の plain `require github.com/takezoh/credproxy <pseudo-version>` (takezoh の fork account が canonical) を fork commit ごとに bump する。charmbracelet 系のような `replace` ディレクティブは不要 — credproxy は takezoh account 直下で fork ではなく effectively upstream。repo-local `forks/` ディレクトリや絶対パスは採らない。"
---

## Context

credproxy fork の修正 (`adr-20260715-credproxy-materialize-method`, `adr-20260715-credproxy-retry-owner-caller-side`, `adr-20260715-credproxy-metadata-handler-async-materialize`) を agent-grid の build に取り込むには、`src/go.mod` の `require github.com/takezoh/credproxy` を最新 fork HEAD の pseudo-version へ向ける必要がある。credproxy の module path は `github.com/takezoh/credproxy` — takezoh account 直下で **`replace` ディレクティブを介さない plain require で参照済み** (src/go.mod:22)。したがって配線問題の本質は「local で iterate 中の未 push commit をどうやって local build に取り込むか」に絞られる。

configuration の候補:

- **(A) `go.work` (gitignored)** — repo root に `go.work` を置き、`./src` と `../credproxy` を並べる。`.gitignore` に既に `go.work` / `go.work.sum` が入っている (baseline)。CI では workspace ファイル不在 = 通常の go.mod 解決に fallback。**contributor 毎に独立、CI に漏れない、iteration が即座**。
- **(B) `replace ... => ../credproxy` を go.mod に足す** — charmbracelet の 2 本と同じ shape だが、commit すると contributor のホームディレクトリ前提が CI に漏れる。commit しないと go.work と同じ用途になる (go.work のほうが本来の tooling)。
- **(C) `replace ... => github.com/takezoh/credproxy <pseudo-version>` を go.mod に足す** — charmbracelet の 2 本と同じ shape だが、credproxy は既に takezoh account 下なので `replace` は冗長 (require の pseudo-version bump のみで足りる)。
- **(D) 絶対 dev-machine path replace** — CI で unresolvable、却下。

## Decision

**(A) `go.work` (gitignored) + `require` pseudo-version bump の 2 層** を採用する。

### Local iteration (contributor 毎)

1. repo root に `go.work` を置く:
   ```
   go 1.24
   use ./src
   use ../credproxy
   ```
2. `.gitignore` は既に `go.work` / `go.work.sum` を除外している (変更不要)。
3. `/home/dev/dev/credproxy` を local で編集 → 即座に `cd src && go build ./...` / `go test ./...` に反映される。

### CI / shared state (fork commit を published state に fix する)

1. iteration が纏まったら `/home/dev/dev/credproxy` を `github.com/takezoh/credproxy` に push。
2. Go が生成する content-addressed pseudo-version (`v0.0.0-<utc-timestamp>-<commit-hash-12>`) を取得。
3. `src/go.mod` の `require github.com/takezoh/credproxy ...` (line 22) をその pseudo-version に bump し、`cd src && go mod tidy` で go.sum を更新して commit。
4. CI runner は go.work が gitignored なので commit された `require` から build する — local iteration state が漏れない。

`replace` ディレクティブは追加しない (credproxy は既に takezoh account 直下)。`forks/` ディレクトリは導入しない。絶対パスも導入しない。

## Consequences

- **local iteration の速さ**: `go.work` により fork edit の後に push を待たずに build/test が回る。chunk 03-05 (credproxy 側の Materialize 実装) と chunk 06-07 (agent-grid 側の caller / triple test) を interleave して iterate できる。
- **CI portable**: go.work は gitignored、CI は go.mod の pseudo-version だけを見る。他 contributor は go.work を自分で作れば同じ workflow に入れる (追加ドキュメントは README または CONTRIBUTING に 1 行の指示で足りる)。
- **Content-addressed for shared state**: `go.sum` が fork commit を hash pin。investigation-2.json harness_gaps が指摘した "fork の内容についての mental model が stale" リスクは remote-fork push のたびに解消される。
- **将来 upstream に剥がせる**: credproxy fork が安定した後は、`require` を最終 pseudo-version に固定し、local `go.work` を消せば通常の module 依存に戻る。
- **注意**: `go.work` を作らずに `cd src && go build` すると、当然 fork の local HEAD ではなく go.mod pinned version を build する。この行き違いは chunk-07 の contract test triple (T2 fake + T3 FakeVsReal) が振る舞いを固定することで顕在化する — local HEAD がテストで期待する `Materialize` method を持たなければ compile error になる。

## Alternatives

### (B) `replace ... => ../credproxy` を go.mod に足す

**却下**: commit すると contributor のホームディレクトリ前提が CI に漏れる。commit しないなら go.work と同じ用途 (むしろ go.work のほうが workspace mode という Go 1.18+ の正式 tooling で、`GOWORK=off` で無効化できる CI 制御も clean)。

### (C) `replace github.com/takezoh/credproxy => github.com/takezoh/credproxy <pseudo-version>` を go.mod に足す

**却下**: credproxy は既に takezoh account 直下 (src/go.mod:22 の plain require)。charmbracelet 系と違い upstream の別 owner が居ないので、`replace` は "指す先を切り替える" 意味を持たず、単に require pseudo-version を書く二度手間になる。

### (D) 絶対 dev-machine path (`/home/dev/dev/credproxy`) への replace

**却下**: contributor 以外のマシンで build が失敗する。CI で unresolvable module error になる。

### replace 無し・go.work 無しで plain require を毎回 bump

**却下**: 各 chunk が fork の追加 push を要求するので、その都度 push → pseudo-version 取得 → go.mod 更新 → tidy の cycle を回すのは iteration が遅い。go.work で local iteration を吸収し、纏まった変更を single push で fix するほうが自然。
