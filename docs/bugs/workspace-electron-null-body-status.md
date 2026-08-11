# Repro: Workspace (Electron) routed API rejects every null-body status (2026-08-11 / debug-run-workspace-electron-terminal-dead)

## 症状

Workspace (Electron) で Created 状態のセッションを stop すると、実際にはデーモン側で停止しているにもかかわらず
`Failed to stop session (network error)` のトーストが出て確認ダイアログが閉じない。

## 環境 (必須)

- OS: Windows (報告者環境。未確認 — Chromium ベースであれば OS 非依存)
- Runtime: Electron renderer (Chromium)。検証は Playwright Chromium 149.0.7827.55 (HeadlessChrome) で実施
- Version / branch / commit: agent-grid main @ e5001d80 (2026-08-11)
- 依存: `clients/workspace` preload/IPC bridge (`agent-grid:session-fleet-request`)
- その他: `clients/ui` webProvider (ブラウザ経路) では発生しない

## 再現手順 (必須)

1. Workspace (Electron) クライアントを起動し、設定画面で対象サーバが Connected になっていることを確認する。
2. セッションを 1 つ作成する。
3. そのセッションに対して stop / terminate を実行する。
4. `Failed to stop session (network error)` トーストが出る。ダイアログは開いたままになる。
5. サーバ側ではセッションが実際に停止している (`handleDeleteSession` は daemon の `EventStopSession` 成功後に 204 を書く)。

## 発生頻度

毎回 (null-body status を返すルートを Workspace 経路で叩いた場合は決定的)。

## 期待 / 実際

- 期待: 204 No Content が成功として扱われ、ダイアログが閉じ、次のセッションが選択される。
- 実際: renderer 内で `TypeError: Failed to construct 'Response': Response with null body status cannot have body` が発生し、
  `api/sessions.ts` の `doFetch` がこれを `Error("network")` に写像し、`useTerminateSession.ts:66` が
  `Failed to stop session (network error)` を表示する。

## 特定した原因 (RCA)

`clients/ui/src/fleet/provider.ts` の `workspaceProvider.request()` が IPC 結果を DOM Response へ再構築する際、

```ts
return new Response(result.body, {
  status: result.status,
  headers: result.headers,
});
```

を無条件に実行する。main 側 (`clients/workspace/src/main/electron-entry.ts` の
`agent-grid:session-fleet-request`) は常に `body: await response.text()` を返すため、204 は空文字列ボディとして
renderer へ渡る。Fetch 仕様上、null body status (101 / 204 / 205 / 304) は非 null ボディを持てないため、
Chromium の `Response` コンストラクタは TypeError を投げる。

責務破綻: IPC 境界が HTTP レスポンスを再物質化しているのに、「null body status はボディを持たない」という
不変条件の持ち主がどちら側にもいない (境界の空白 + 契約の暗黙化)。`{status, headers, body}` という
IPC envelope 型は、この不変条件を型で表現していない。

同じ根本を持つ他経路 (すべて `fleet.request` 経由):

- `clients/ui/src/api/sessions.ts` `deleteSession` → `DELETE /api/sessions/{id}` → 204 (`services/server/api/mux.go:454`)
- `clients/ui/src/api/sessions.ts` `markSessionMessagesRead` → 204 (`services/server/api/messages.go:84`)
- `clients/ui/src/api/transcripts.ts` → `GET /api/sessions/{id}/transcript` の 304 (`services/server/api/transcript.go:104`)
  と 204 (`services/server/api/transcript.go:111`)。クライアントは 304 を明示的に分岐しており
  (`api/transcripts.ts:42`)、Workspace 経路ではその分岐に到達できない。
- `clients/ui/src/api/workspace.ts` `sessionRequest`、`clients/ui/src/api/capsules.ts` — 同じ再構築を通るが、
  終点ハンドラを突合した結果 **除外** (2026-08-11 再調査)。`services/server/api/workspace.go` は全経路
  `writeJSON(w, http.StatusOK, …)` / 4xx / 5xx、`services/server/api/mux_resumecapsule.go` は 200 / 4xx / 5xx のみで、
  null body status を返さない。`services/server/api/mux_repository.go:182,204` の 204 (lease revoke) は
  `clients/ui` からの呼び出し元が存在しない。

## 修正 (fixer 段で追記)

<未適用>

## テスト (2026-08-11 再調査で撤回・置換)

**撤回**: 初版はこの欠陥を「unit test 不可能」と記録していた。根拠は「vitest 環境 (happy-dom 20.10.6) の
`Response` は `new Response("", { status: 204 })` を throw せず受理しボディを null へ丸めるため、
どんなアサーションも正誤を区別できない」というもの。happy-dom の寛容さ自体は事実だが、
**environment は file 単位で選べる**ため「不可能」は導けない。

Node 24.15.0 の global `Response` は Chromium と同じ判別を行う (`new Response("", {status:204})` →
`TypeError: Response constructor: Invalid response status code 204`、`new Response(null,{status:204})` と
`new Response("",{status:200})` は成功)。したがって file 先頭に `// @vitest-environment node` を置けば
この seam は T1 で機械検出できる。

RED test: `clients/ui/src/fleet/provider.null-body-status.test.ts`

```sh
cd clients/ui && npx vitest run src/fleet/provider.null-body-status.test.ts
```

観測 (修正前 / 2026-08-11): 4 件中 2 件失敗。

```
TypeError: Response constructor: Invalid response status code 204
 ❯ Object.request src/fleet/provider.ts:644:14
TypeError: Response constructor: Invalid response status code 304
 ❯ Object.request src/fleet/provider.ts:644:14
```

残る 2 件 (環境ガードと 200 系の素通し) は修正前から緑で、修正が既存契約を壊さないことを固定する。
環境ガードのケース (`new Response("", {status:204})` が throw することを assert) は、環境が寛容な
Response へ戻った瞬間に残りのアサーションが偽の緑になるのを防ぐためのもの。

付随変更: `clients/ui/src/test-setup.ts` は共有 setupFile として全 file に適用されるため、
DOM 前提の 3 フック (`window.history.replaceState` / `@testing-library` の `cleanup` /
xterm token の documentElement 操作) を `typeof document !== "undefined"` でガードした。
DOM 環境では従来どおり実行されるので既存アサーションは一切弱まらない
(全 unit suite: 1872 passed / 変更前後で差分なし)。

## Validation 方法 (実 Chromium 側の裏取り)

実 Chromium での probe (このリポジトリの `clients/ui` から実行可能、Playwright 済みインストールを使用):

```sh
cd clients/ui
cat > chk204.mjs <<'EOF'
import { chromium } from "@playwright/test";
const browser = await chromium.launch();
const page = await browser.newPage();
console.log(await page.evaluate(() => {
  try { return "ok:" + new Response("", { status: 204 }).status; }
  catch (e) { return "THREW: " + e.name + ": " + e.message; }
}));
await browser.close();
EOF
PLAYWRIGHT_BROWSERS_PATH=/tmp/ms-playwright node ./chk204.mjs && rm chk204.mjs
```

観測結果 (2026-08-11):

```
THREW: TypeError: Failed to construct 'Response': Response with null body status cannot have body
```

参考: `new Response(null, { status: 204 })` は成功し、`new Response("", { status: 200 })` も成功する
— すなわち失敗条件は「null body status + 非 null ボディ」に限定される。

恒久的な回帰検出の主戦力は上記の node-environment T1。加えて happy-dom が real (Chromium / Node) より
甘いこと自体が fake-vs-real 忠実性の破れなので、happy-dom の `Response` を real 準拠へ寄せるか、
DOM API のエッジ仕様に依存する seam を node environment へ隔離する規約が要る (harness gap として起票対象)。
