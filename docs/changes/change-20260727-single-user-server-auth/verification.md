---
change: change-20260727-single-user-server-auth
role: verification
---

<!-- lifecycle is owned by change.md -->

# Verification

## Success conditions

| Requirement | Tier | Verification |
|---|---|---|
| FR-AUTH-001..004, NFR-AUTH-001 | T0/T2 | Shared v2 valid/invalid fixtures pass JSON Schema, C#, and TypeScript; v1/token_path fails without file access or rewrite. |
| FR-AUTH-002..003, NFR-AUTH-002..003 | T1 | Both clients prove bearer header/ticket mint, none omission, and 401/unavailable without fallback. |
| FR-AUTH-005..007 | T0/T1 | Server parsing rejects the removed flag; listener-address tables cover loopback/non-loopback/unspecified; restart reuses token-file and missing/empty/unreadable fails. |
| FR-AUTH-008 | T2 | Normal and injected partial-failure witnesses prove success is withheld and rollback restores authenticated health. |
| FR-AUTH-009 | T0/T1 | Sentinel token capture across the enumerated consumers proves masking and absence from URL/log/status/error output. |

## Commands

```sh
cd src && GOCACHE=/tmp/gocache-agent-grid go test ./cmd/server/... ./server/api/...
cd clients/workspace && npm run typecheck && npm run test:unit
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "$(wslpath -w clients/windows-shell/scripts/win-test.ps1)"
python3 /home/take/.codex/plugins/cache/agent-module-dev/dev/0.2.0/skills/dev-docs/scripts/docs_cli.py lint --conformance
```

## Adversarial witnesses

- Requested hostname or `:8443` resolves/binds to unspecified/non-loopback: WARN is still emitted because the listener address is authoritative.
- Bearer token file disappears at restart: startup fails; no new identity is generated.
- One client remains on the old token during explicit change: the operation fails verification and rolls back.
- A sentinel token reaches any settings error, health status, log, URL, or config preview: verification fails.
