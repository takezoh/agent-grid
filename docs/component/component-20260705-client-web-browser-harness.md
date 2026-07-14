---
id: component-20260705-client-web-browser-harness
kind: component
title: client/web browser harness
status: active
created: '2026-07-05'
updated: '2026-07-14'
tags:
- testing
- web
- client
owners: []
provides:
- client-web-browser-harness
source_paths:
- src/client/web/playwright.config.ts
- src/client/web/e2e/
- src/client/web/package.json
- .github/workflows/ci.yml
relations:
- {type: references, target: component-20260624-client-overview}
- {type: references, target: spec-20260705-test-harness}
- {type: referencedBy, target: note-20260624-agent-testing}
summary: Playwright browser smoke と fake backend で Web UI の session hydrate / command
  palette / new-session submit を常時検証する harness
---

## Overview

`client/web` の browser harness は、happy-dom では証明し切れないブラウザ配線を常時検証する
Playwright smoke 層である。責務は UI の見た目を比較することではなく、実ブラウザ上で
「アプリが起動し、session 一覧と command 導線が正しくつながっている」ことを短時間で pin する点にある。

現在の常時シナリオは次の 3 つで固定する。

- session hydrate: 初期 `hello` / `view-update` を受けて既存 session が一覧へ反映される
- command palette: keyboard shortcut から palette を開ける
- new-session submit: session 作成フォーム送信で API 呼び出しと新規 session 描画が成立する

この harness は `src/client/web/e2e/support/fake-backend.ts` の deterministic fake backend に依存する。
REST は `page.route()` で `/api/ws-ticket` / `/api/session-config` / `/api/sessions` を fake 化し、
WebSocket は `page.addInitScript()` で差し替えて初期 event 列を制御する。これにより flaky な外部依存を
持ち込まず、`npm run test:web` を PR CI の必須 gate にできる。

ブラウザ smoke が保証するのは wiring までであり、次は意図的に対象外とする。

- 実 soft keyboard / 実 VoiceOver / 実 long-press のような OS 実機依存挙動
- visual regression や screenshot diff
- 本物の backend / websocket daemon を使う fidelity 検証

実機依存の観察は `docs/specs/web-terminal-mobile-ux/` の手動検証チェックリストが正本で、backend 側の
server→view 貫通は `src/server/web` の gateway scenario e2e が担当する。

## Parts

主要な構成要素:

- `playwright.config.ts`: dev server 起動と Chromium smoke project の定義
- `e2e/support/fake-backend.ts`: deterministic fake backend と fake WebSocket
- `e2e/app.smoke.spec.ts`: hydrate / palette / new-session の常時シナリオ
- `package.json` の `test:e2e` / `test:web`: unit・build・browser smoke を分離した入口
- `.github/workflows/ci.yml`: Chromium install を含む CI gate

## Running locally

Run commands: [AGENTS.md](../../AGENTS.md) (Build & Test → E2E → Web Playwright smoke). Below is harness-specific detail for troubleshooting.

PR CI (`.github/workflows/ci.yml`) と同じ browser gate を手元で再現する。

### 1. 依存関係のインストール

```sh
cd src/client/web
npm ci
```

`~/.npm` への書き込み権限がない環境では cache を `/tmp` に逃がす。

```sh
NPM_CONFIG_CACHE=/tmp/npm-cache-agent-grid npm ci
```

### 2. Playwright ブラウザのインストール (初回 or `@playwright/test` 更新後)

```sh
npx playwright install chromium
```

CI は `npx playwright install --with-deps chromium` を使う。ローカルで system deps が足りないときは同じフラグを付ける。

`~/.cache/ms-playwright` へ書けない・ダウンロードできない場合は、ブラウザ保存先を明示する。

```sh
PLAYWRIGHT_BROWSERS_PATH=/tmp/ms-playwright npx playwright install chromium
```

その後の実行でも同じ `PLAYWRIGHT_BROWSERS_PATH` を渡す。

ダウンロードが難しい場合の escape hatch: `playwright.config.ts` が読む `PW_USE_SYSTEM_CHROME=1` でインストール済み Chrome を使う。

```sh
PW_USE_SYSTEM_CHROME=1 npm run test:e2e
```

### 3. 実行コマンド

| 目的 | コマンド |
|------|----------|
| browser smoke のみ | `npm run test:e2e` |
| lint + unit + browser smoke (CI と同じ web gate) | `npm run test:web` |
| unit のみ | `npm run test:unit` |

`test:e2e` は内部で `npm run build` (tsc + vite build) を走らせてから `playwright test` する。`playwright.config.ts` が `vite preview` を `127.0.0.1:4174` で起動し、`e2e/app.smoke.spec.ts` が fake backend 経由で hydrate / palette / new-session を検証する。

ブラウザ保存先を `/tmp` にした例:

```sh
PLAYWRIGHT_BROWSERS_PATH=/tmp/ms-playwright npm run test:e2e
```

### 4. よくある失敗

| 症状 | 対処 |
|------|------|
| `Executable doesn't exist at .../ms-playwright/...` | 手順 2 を実行。`PLAYWRIGHT_BROWSERS_PATH` を install と test で揃える |
| `biome: command not found` | `npm ci` 未実行。手順 1 からやり直す |
| TypeScript build が落ちる | `npm run build` を単体で実行し、`tsc` エラーを先に直す |
