---
change: change-20260803-execution-server-selector-health
role: verification
---

<!-- lifecycle is owned by change.md -->

# Verification

## Content

- RED/GREEN: `ExecutionServerSelector.test.tsx` の正常 option 選択と health 色/disabled 契約。
- Integration: `ParamSelectPhase.test.tsx` の server selection store 更新。
- Browser: `app.smoke.spec.ts` の Change → connected server click → current label 更新。
- Gateway: hello 後 4.9 秒以内に application heartbeat が届くこと。
- Host parity: Web Host / Workspace Host の双方で 2 freshness windows を越えて heartbeat が接続を維持すること。
- Recovery: 初回 config failure 後の unavailable → connected で一度だけ再取得し、Command List を hydrate すること。
- Gate: UI unit tests, lint/typecheck, Playwright smoke（実行可能な環境で実施）。
