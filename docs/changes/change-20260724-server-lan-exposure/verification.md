---
change: change-20260724-server-lan-exposure
role: verification
---

<!-- lifecycle is owned by change.md -->

# Verification

## Content

### 自動 test (T0-T1)

- `go test ./cmd/server/...` — resolve_auth_test の 3 分岐 (opt-in なし拒否 / opt-in 許容 / opt-in 単独 no-op) が全て PASS
- `make lint` — 0 issues

### 手動 fidelity 検証 (T3, ローカル systemd)

- `make update-server` で unit を restart 後、`ss -tln` で `*:8443` (wildcard bind) を確認
- `curl http://127.0.0.1:8443/healthz` と `curl http://<LAN IP>:8443/healthz` の両方が 200 を返す
- `server.log` (`/home/dev/.local/state/agent-grid/server.log`) に以下を確認:
  - `INFO msg="gateway listening" url=http://0.0.0.0:8443 auth="auth=disabled"`
  - `WARN msg="gateway: -no-auth on NON-LOOPBACK bind …" addr=0.0.0.0:8443`
