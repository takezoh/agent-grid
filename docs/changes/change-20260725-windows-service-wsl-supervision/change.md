---
id: change-20260725-windows-service-wsl-supervision
kind: change
title: Windows Service supervision for the WSL host server
status: done
created: '2026-07-25'
profile: sdd@1
intent: Windows でサインインしていない状態でも host server が利用可能で、落ちても復帰し、止めれば本当に止まり、何が起きたかを後から読める状態にする。plans/windows-service-wsl-host-supervision.md
  の §2-§15 を観察可能な受け入れ条件へ変換し、plan §12 の PoC 11 検証項目に対応させる。対象は host server 自身の Windows
  上での起動・監視・停止であり、クライアントは対象外。
outcomes:
- Automatic Session 0 startup reaches status ok without an interactive sign-in.
- Transient failure recovers within fixed retry bounds or becomes a typed terminal
  failure.
- SCM stop reaches the existing SIGTERM drain path and never broadens beyond the registered
  distribution.
- Machine-wide structured records distinguish Ready, Backoff, Failed, already-running,
  stop, and escalation without secrets.
scope:
- src/cmd/agent-grid-wsl-supervisor and src/platform/wslhost Windows Service supervision
- additive src/cmd/server supervisor-probe and supervisor-stop contracts
- Windows SCM, WSL, process, health, filesystem/ACL, clock/resume, and record dependency
  seams
- fake, invariant contract, and FakeVsReal Windows/WSL verification triple
non_goals:
- WSL 内 systemd unit の作成
- systemd と Windows SCM の二重監督
- Linux host server の Windows native port
- Windows Container sandbox
- Windows と Linux を跨ぐ bridge 接続
- WSL distribution の汎用 daemon manager 化
change_classes:
- behavior
- responsibility
- boundary
- invariant
- capability
governance:
  gate: soft
  reasons:
  - seven-proposed-adr-consultation-required-before-ready
  - windows-scm-wsl-fidelity-required-before-closure
evidence_refs:
- type: source
  ref: git:delivery/windows-service-wsl-supervision
- type: test
  ref: go:test ./platform/wslhost/... ./cmd/server/... ./cmd/agent-grid-wsl-supervisor/...
- type: test
  ref: go:test -tags=e2e ./platform/wslhost/...
- type: test
  ref: GOOS=windows GOARCH=amd64 go:test ./platform/wslhost ./cmd/agent-grid-wsl-supervisor
    -run ^$
- type: test
  ref: go:vet ./...
members:
- role: requirements
  path: changes/change-20260725-windows-service-wsl-supervision/requirements.md
  required: true
- role: implementation
  path: changes/change-20260725-windows-service-wsl-supervision/implementation.md
  required: true
- role: verification
  path: changes/change-20260725-windows-service-wsl-supervision/verification.md
  required: true
- role: ux
  path: changes/change-20260725-windows-service-wsl-supervision/ux.md
  required: false
promotion:
- target: design-platform
  section: capabilities
  action: upsert
  item:
    id: cap:windows-wsl-supervision
    uniqueness: per-platform
  reason: The capability and its injection boundary govern future Windows/WSL supervision
    changes.
- target: design-platform
  section: invariants
  action: upsert
  item:
    id: INV-009
    statement: Every SCM, WSL, Windows process, ACL, resume, and OS-record dependency
      ships a fake, invariant-naming contract test, and FakeVsReal fidelity backstop.
    enforcement: contract
  reason: This external-dependency discipline must survive after the change package
    closes.
- target: design-client
  section: failure_responsibilities
  action: upsert
  item:
    id: FAILURE-005
    statement: Windows supervision may terminate only the registration-bound WSL distribution
      and must preserve the host server SIGTERM drain and health semantics.
  reason: Bound termination and preservation of host-owned shutdown semantics constrain
    future host lifecycle work.
unresolved_decisions: []
tags:
- windows-shell
- supervision
owners: []
relations:
- {type: references, target: design-windows-shell}
- {type: references, target: adr-20260724-boundary-3-adopt-before-spawn}
- {type: references, target: adr-20260724-boundary-3-wsl-detach-spike}
- {type: references, target: adr-20260724-daemon-health-toast-structural-separation}
- {type: references, target: design-client}
- {type: modifies, target: design-platform}
- {type: modifies, target: design-client}
source_paths: []
summary: Windows Service (SCM) を supervisor として、その配下で wsl.exe 経由に host server を保持する構成の
  UX 要件。サインインしていない状態での自動起動、readiness、backoff 再起動、graceful stop、対象 distribution 限定の
  terminate、Windows 側の記録を観察可能な受け入れ条件へ変換する。
updated: '2026-07-26'
promotion_applied_at: '2026-07-25T19:24:06.312878+00:00'
closure:
  closed_at: '2026-07-25T19:50:44.586326+00:00'
  content_hash: sha256:9e9625d48bdae455c077a94537fdac7111d0933b69ebd0f695e54d2c65fb2705
---

## Summary

## Closure Notes


{% transition from="ready" to="active" date="2026-07-26" %}
implementation, verification, and independent review completed
{% /transition %}


{% transition from="active" to="closing" date="2026-07-26" %}
implementation, verification, and independent review completed
{% /transition %}
