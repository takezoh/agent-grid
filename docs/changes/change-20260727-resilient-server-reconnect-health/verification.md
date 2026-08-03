---
change: change-20260727-resilient-server-reconnect-health
role: verification
---

<!-- lifecycle is owned by change.md -->

# Verification

## Success conditions

- T0/T1 active lifecycle: initiating＋自動16回で停止し17回目を作らない。socket openだけではopenにならず、current lifecycle/attempt helloだけで復旧する。attempt nのlate hello/close、server switch、repeated Retry、shutdownはstate/resourceを変更しない。
- T1 StatusBar: no-sessionでもRetryへpointer/keyboard/ATで到達でき、live textとbuttonはsibling、focused Retry消滅時だけ固定landmarkへfocusする。他surfaceにRetryがない。
- T0/T1 fleet: exclusive unionがinvalid combinationをrejectし、Connectedは5秒でrefresh、capability/ticket/helloは各3秒で終了する。exact mismatchはIncompatible protocolでterminal、disabled workerは0、shutdown後handleは0になる。
- T1 host parity/wire: canonical fixtureをTS/Workspace/Go codecが同じ結果へdecode/encodeし、unknown version/stateをrejectする。delivery lossはhost revisionを進めずHealth unavailable overlay＋nested stale priorを生成し、reconnect snapshotで解除する。
- T1 security/diagnosis: bearer、userinfo、query token sentinelがprojection、serialized wire、DOM、config、captured health logsにない。両host Connections rowsが一致し、Workspace edit controlsとbrowser Copy configuration path/restart instructionはhealth failure中も利用できる。
- T3 fidelity: modified gateway-facing fakeにinvariant-naming contract testと`FakeVsRealFleetHealth*`があり、real binary未設定時だけexpected skipする。

## Commands

```sh
cd clients/ui && npm run test:unit -- src/socket/connection.recovery-regression.test.ts src/socket/connection.test.ts src/components/StatusBar.test.tsx src/fleet/session-protocol.test.ts src/fleet/connections-presentation.test.ts src/components/ConnectionsPanel.test.tsx
cd clients/workspace && npm run test:unit -- tests/fleet-session-runtime.test.ts tests/server-health.test.ts tests/settings-page.test.ts tests/fleet-health-wire.test.ts tests/desktop-config.test.ts
cd clients/web-host && go test ./internal/webhost -run 'TestFleetHealth|TestFleetHealthWire|TestFleetHealthRedaction|TestFleetHealthTimeout|TestFleetHealthShutdown'
cd services && GOCACHE=/tmp/gocache-agent-grid go test ./server/api/...
cd clients/ui && npm run typecheck
cd clients/workspace && npm run typecheck
cd clients/ui && npm run test:e2e
cd clients/ui && npm run test:e2e:real
GOCACHE=/tmp/gocache-agent-grid make test-e2e
```

実装後のcross-cutting gateとしてrepository `AGENTS.md`のGo lint/test、UI lint/unit/e2eを追加実行する。未実行のT3/manual evidenceを完了扱いにしない。
