# A1-ε Implementation Plan: cleanup + `server/session` removal + gateway documentation

- **作成日**: 2026-06-20
- **ブランチ**: `feat/tmux-free-web-server`(A1-α/β/γ/δ 完了、`make build-all` 緑、Go 76/76、TS 154/154、lint 0)
- **親計画**: [Master Plan(`plans-cheerful-thompson.md`)](../../plans/arc-server-client-split.md)
- **前段**: A1-δ で persistence + connector + notification が稼働、A1 機能スコープは完成

## Goal

機能凍結 PR。A1-α で build tag `legacy_session` 隔離した `server/session/` を `git rm` で完全削除し、その legacy 参照を持つ tests とコメント、wire.go の互換層を整理する。新規ドキュメント `docs/technical/web-gateway.md` で α 〜 δ の wire vocabulary と sequence diagram を記録、`docs/user/web-server.md` の Architecture 章を daemon 所有モデルに更新。`plans/arc-server-client-split.md` ヘッダを A1 完了に書き換える。

## Scope

### In scope

#### Deletion(`git rm`)
- **`src/server/session/`** ディレクトリ全体(`service.go`, `service_test.go`)
- **`src/server/web/gateway_test.go`**(build tag `legacy_session`、旧 termvt 直叩きベース)
- **`src/server/web/inbound_test.go`**(build tag `legacy_session`、旧テスト)
- **`src/server/web/mux_test.go`**(build tag `legacy_session`、`server/session` import)

これら 4 ファイルは A1-α 時点で隔離タグが付いているので、削除しても本体ビルドに影響はない。

#### Cleanup
- **`src/server/web/wire.go`**(必要なら): α で `outputFrame` / `controlFrame` 互換 helper を gateway.go に退避させた経緯がある。γ/δ で `encodeServerEvent` が中心になったので、もう使われていない退避コードを整理。grep で参照ゼロを確認した上で削除。
- **`src/server/web/mux.go`** のコメント: 「former server/session.Info JSON shape so existing browser UI keeps working」「mirrors old server/session.Spec」等の歴史コメントを、現状の wire 形を述べる形に整える(歴史言及は ADR や本 spec から辿れる)。
- **`src/server/web/gateway.go`**(必要なら): α 時点で残した互換 adapter があれば整理。
- **`src/cmd/server/main.go`**: `agentlaunch.DirectDispatcher` 等の dead import が残っていれば削除。

#### Documentation
- **`docs/technical/web-gateway.md`**(新設、~300 行): A1-α 〜 δ の完成形 wire vocabulary を 1 箇所に集約
  - 全 server → browser frame (`o`/`c`/`h`/`v`/`tt`/`et`/`n`/`cu`/`r`/`e`)の shape と意味
  - 全 browser → server frame (`i`/`r`/subscribe via REST など)の shape
  - REST endpoints (`/api/sessions`, `/api/sessions/{id}/transcript`, `/api/sessions/{id}/event-log`, `/api/ws-ticket`, `/healthz`)の仕様
  - Sequence diagram (ASCII art): session 作成 → WS attach → terminal subscribe → output stream → daemon disconnect の 2 段 close
  - ADR 0005 / 0006 / 0011 / 0012 / 0023 / 0025 / 0027 への 相互参照
- **`docs/user/web-server.md`**(修正): Architecture 章を `cmd/server` = `arc daemon の HTTP/WS gateway` モデルに書き換え。前の termvt 直叩きモデルの説明を削除。
- **`plans/arc-server-client-split.md`**(修正): ヘッダ status を A1 完了に、§7 次アクションを「C: tmux 実装の残り削除」に更新。
- **`docs/adr/0014-server-session-legacy-build-tag.md`**(状態更新): Status を `accepted` → `superseded`(ε で削除完了)に、Superseded-by として ε commit を参照(ADR ファイル末尾に追記、または Status を accepted のまま「superseded by removal in A1-ε」とコメント追記)。
- **`ARCHITECTURE.md`**(必要なら): A1-α で追加した「Server gateway (server/*)」節を最終形に整理(δ 経路を含めて更新)。

### Out of scope
- 新機能(view 拡張、connector action、warm restart 等)
- tmux 実装の削除(phase C — 別 PR)
- arc proto の TCP/TLS 化(phase D、optional)
- `client/web/dist/` の bundle 最適化(`vite build` の treeshaking 強化等)
- biome / golangci-lint ルールの強化

### Deletion 確認
- `grep -rn 'server/session' src/` = 0 件(コメント・doc 文字列含む)
- `grep -rn 'legacy_session' src/` = 0 件(build tag が全て消えた)
- `grep -rn 'legacy_session' .` = 0 件(ADR 0014 の言及は本文中に残るが、build directive としての出現は 0)

## EARS Requirements

| ID | Type | Statement | Rationale |
|---|---|---|---|
| **FR-ε01** | ubiquitous | システムは `src/server/session/` ディレクトリと build tag `legacy_session` を含む全 test ファイルをリポジトリから削除しなければならない | A1-α の隔離決定の完了 |
| **FR-ε02** | unwanted | もし `grep -rn 'server/session' src/` や `grep -rn 'legacy_session' src/` が非ゼロの結果を返すなら、CI は失敗しなければならない(または手動 verify で 0 件を確認) | regression 防止 |
| **FR-ε03** | ubiquitous | システムは `docs/technical/web-gateway.md` に A1-α 〜 δ の wire vocabulary、REST endpoints、sequence diagram、関連 ADR への相互参照を含めなければならない | knowledge 集約 |
| **FR-ε04** | ubiquitous | システムは `docs/user/web-server.md` の Architecture 章を `cmd/server` = `arc daemon gateway` モデルに書き換えなければならない | user-facing doc 整合 |
| **FR-ε05** | ubiquitous | システムは `plans/arc-server-client-split.md` のヘッダ status を A1 完了に、§7 次アクションを「C: tmux 実装の残り削除」に更新しなければならない | planning doc 整合 |
| **FR-ε06** | ubiquitous | システムは `ADR 0014`(server/session を build tag で隔離)の status を `superseded` または同等のマーカーで更新しなければならない | ADR history |
| **FR-ε07** | ubiquitous | `make build-all && cd src && go test ./... -race` および `cd src/client/web && npx vitest --run` が本 PR 適用後も緑でなければならない | regression test gate |
| **FR-ε08** | ubiquitous | `cd src && go tool golangci-lint run ./...` が 0 issues、`cd src/client/web && npm run lint` が 0 issues でなければならない | lint gate |

## ADR(本 ε で追加)
なし。ε は ADR 0014 の status 更新のみ(機能追加なし)。

## Verification

```sh
# Deletion proof
grep -rn 'server/session' /home/dev/dev/agent-reactor-new/src/   # → 0 件
grep -rn 'legacy_session' /home/dev/dev/agent-reactor-new/src/   # → 0 件

# Build & tests
make build-all
cd src && go test ./... -race -count=1
cd src && go vet ./...
cd src && go tool golangci-lint run ./...

# Frontend
cd src/client/web && npm ci
cd src/client/web && npm run typecheck
cd src/client/web && npm run lint
cd src/client/web && npx vitest --run

# Manual smoke
make run-dev
# arc daemon + cmd/server + browser で session 作成 / 操作 / OSC notification / transcript tail がすべて動作
```

## 削除影響リスト(必須確認)

| ファイル | A1-α 時点の status | ε 後 |
|---|---|---|
| `src/server/session/service.go` | `//go:build legacy_session` 隔離 | 削除 |
| `src/server/session/service_test.go` | `//go:build legacy_session` 隔離 | 削除 |
| `src/server/web/gateway_test.go` | `//go:build legacy_session` 隔離 | 削除(γ/δ で gateway_view_update_test, gateway_persist_test, gateway_terminal_test が代替) |
| `src/server/web/inbound_test.go` | `//go:build legacy_session` 隔離 | 削除(対応する inbound test は gateway 系に統合済み) |
| `src/server/web/mux_test.go` | `//go:build legacy_session` 隔離、`server/session` import | 削除(mux_daemon_test.go が代替) |

## Open Questions(実装直前に決める)

1. `src/server/web/wire.go` で α 時代に gateway.go に退避させた legacy `outputFrame` / `controlFrame` / `encodeEvent` helper が残っているか実装直前に確認。残っていれば削除。
2. ADR 0014 の status 更新は `Status: Accepted (superseded by removal in A1-ε commit <SHA>)` 形式で。
3. `docs/technical/web-gateway.md` の sequence diagram を ASCII art にするか Mermaid にするか — Mermaid は GitHub render で読みやすいので Mermaid を採用。

## Traceability

- **親 plan**: Master Plan(`plans-cheerful-thompson.md`)
- **前段**: A1-α(commits 615b55f..1d7cb8b)で wire 層・gateway 化、A1-β(React+TS)、A1-γ(view-update)、A1-δ(persist + connector + notification)
- **次の作業**: **C(tmux 実装の残り削除)**。56 ファイルから漸減、`tmux_real.go` / `tmux_pipe_tap.go` / `panetap.go` / `tmux_injector.go` 等を削除し client-side layout に置換。Phase D(arc proto TCP+TLS+token 化)は optional。
