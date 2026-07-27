---
change: change-20260727-resilient-server-reconnect-health
role: verification
---

<!-- lifecycle is owned by change.md -->

# Verification

## T0/T1 verification

- T0/T1: `Connection daemon recovery regressions` — 17回のcontrol-closeで自動停止し、manual Retry後のhelloで復旧する。
- T0: `StatusBar` — closed時のRetryとdisabledを除外したpartial fleet summaryを検証する。
- T1: `probeServerFleet` — enabledだけをprobeし、接続状態と理由をserver idへ対応付ける。
- T1: `FleetHealthMonitor` — 複数renderer要求を統合し、invalidate後だけ再probeする。
- Static: UIとWorkspaceのTypeScript typecheck。

## Commands

```sh
cd clients/ui && npm run test:unit -- src/socket/connection.recovery-regression.test.ts src/components/StatusBar.test.tsx
cd clients/workspace && npm run test:unit -- tests/server-health.test.ts tests/settings-page.test.ts tests/daemon-config.test.ts
cd clients/ui && npm run typecheck
cd clients/workspace && npm run typecheck
```
