---
id: change-20260727-web-host-client-boundary
kind: change
title: Treat the Web host as a configured client
status: active
created: '2026-07-27'
profile: sdd@1
intent: Web host と Workspace Electron を対等なclient hostにし、非client deployable servicesと専用共有基盤を独立services
  moduleへ、client config schema/fixturesをclients/configへ明確化する。
outcomes:
- Web host と Workspace は同じ複数-server projection、qualified identity、partial failure、retry
  semantics を提供する。
- clients/web-host はservices moduleから独立してbuild/testできる。
- services/はgithub.com/takezoh/agent-grid/services module、clients/configは全client host共通のlanguage-neutral
  config ownerになる。
- top-level ownershipはservices/, clients/, protocol/, contracts/, docs/, deploy/,
  scripts/, test-harness/に閉じる。
- browser/Electronのcardinality/policy特別扱いを禁止する永続設計とfitness functionが残る。
scope:
- clients/web-host nested Go module、Web fleet proxy、Workspace/UI provider generalization
- src Go module/source rootsのservices/ flag-day、clients/config rename、active build/CI/scripts/current
  docs更新
- active multi-server changeのbrowser one-server契約をhost-neutral契約へreconcile
- Web static/CSP/header/same-origin WS/retryable upstream behaviorのqualified route上の保存
non_goals:
- completed/abandoned change historyの現在pathへの同期
- clients/web-hostからservices moduleへのrequire/replace/internal/source dependency
- repository-root Go moduleまたはflat Go package tree
- runtime config reload、UIへのcredential投影、legacy unqualified /api・/ws compatibility
  shim
change_classes:
- behavior
- responsibility
- boundary
- dependency
- invariant
- capability
governance:
  gate: hard
  reasons:
  - accepted ADRのbrowser one-server clauseをsupersedeし、public Web routeとrepository
    layoutをbreaking変更するため。
members:
- role: requirements
  path: changes/change-20260727-web-host-client-boundary/requirements.md
  required: true
- role: implementation
  path: changes/change-20260727-web-host-client-boundary/implementation.md
  required: true
- role: verification
  path: changes/change-20260727-web-host-client-boundary/verification.md
  required: true
promotion:
- target: design-clients-ui
  section: invariants
  action: upsert
  item:
    id: invariant-host-semantics-transport-only
    statement: Web host と Workspace Electron の差はdelivery transportだけであり、fleet cardinality、identity、auth
      capability、failure、retry semanticsをhost kindで分岐しない。
  reason: browser one-server/Electron multi-serverの再導入を将来変更で禁止する。
- target: design-clients-ui
  section: boundaries.consumes
  action: upsert
  item:
    id: boundary-host-neutral-fleet-provider
    statement: clients/uiはcredential-free server-qualified FleetProviderだけをconsumesし、host
      credential/config sourceを認識しない。
  reason: shared SPA とhost trust boundaryを分離する。
- target: design-workspace-desktop
  section: responsibilities
  action: upsert
  item:
    id: responsibility-workspace-host-adapter
    statement: Workspace mainは共通FleetProviderのElectron transport adapterとcredentialed
      routingを所有する。
  reason: Electron固有の意味論ownerから共通契約のadapter ownerへ更新する。
- target: design-clients-ui
  section: capabilities
  action: upsert
  item:
    id: capability-configured-web-host
    statement: clients/web-hostはversioned configをloadする独立hostとしてSPA、credential-free
      config projection、qualified same-origin fleet routingを提供する。
  reason: 新しい恒久client-host capabilityを記録する。
- target: design-clients-ui
  section: boundaries.provides
  action: upsert
  item:
    id: boundary-client-config-owner
    statement: clients/configは全client hostが共有するlanguage-neutral versioned schema/fixturesを所有する。
  reason: clients/desktop-configのdesktop限定名を廃止し共通ownershipを固定する。
unresolved_decisions: []
tags:
- web-host
- client-boundary
- fleet-provider
- go-module
owners: []
relations:
- {type: dependsOn, target: change-20260727-multi-server-session-palette}
- {type: modifies, target: design-clients-ui}
- {type: modifies, target: design-workspace-desktop}
- {type: references, target: adr-20260727-single-user-server-auth}
- {type: references, target: adr-20260727-server-qualified-identity-partial-failure}
source_paths:
- clients/ui
- clients/workspace
- clients/config
- clients/web-host
- services
- protocol
- contracts
- deploy
- test-harness
- Makefile
- .github
- scripts
- AGENTS.md
- ARCHITECTURE.md
summary: Make Web host and Workspace equal configured clients while separating services
  and client-host module ownership.
updated: '2026-07-27'
---

## Summary

Web host を単一server browser adapterとして扱う設計を廃止し、Workspace と同じ client config / fleet/provider 意味論を別transportで実装する。Web host はclients/web-host、非client service群はservices、共通config契約はclients/configへflag-day移行する。

## Migration Classification

Active consumer は source/tests、Makefile・CI・scripts、AGENTS.md・ARCHITECTURE.md、`docs/design/`、現行guide/note、statusが draft/ready/active/closing のchange packageである。statusが done/abandoned のchange packageはimmutable historyとしてpath一括置換対象外とする。tracked zero-byte `src/.claude/*` はroot `.claude`へ移さず削除し、ユーザー/host設定を上書きしない。

## Closure Notes

HARD approval、全proposed ADR採否、promotion適用、services/Web-host module gates、top-level ownership、host parity、active-reference audit、docs conformanceのevidenceが揃ってからclosingへ進める。

## Downstream sharpening (2026-08-14)

`change-20260814-cli-prerequisite-contracts` が `boundary-client-config-owner` を宣言から**機械的に強制される契約**へ進めた。この change の outcome を変えるものではなく、その到達点を具体化するもの。

- `clients/config` の Go projection は `clients/web-host/internal/clientconfig` から新 module `clients/goclient/clientconfig` へ移動した。web-host はそれを `replace` 経由で import する **consumer** であり、owner ではない。CLI も同じ loader を読む。non_goal「clients/web-host から services module への dependency」は変わらず守られている（`clients/goclient` は services module ではない）。
- 言語ごとに loader は 1 つだけで、2 つ目が現れると `cd services && go test ./gorules/ -run TestClientConfigHasExactlyOneLoaderPerLanguage` が落ちる。
- 3 言語の一致は README の主張ではなく `clients/config/fixtures/servers-corpus.json` の期待結果 corpus で担保される（`scripts/check-client-config-parity.sh`）。
- 契約自体も変わった: admission は per-entry になり、不正な 1 entry が fleet 全体を落とさなくなった。詳細は `adr-20260814-shared-go-client-config-module` と `adr-20260814-server-display-name-naming-contract`。

`change-20260812-cli-terminal-client` でその「CLI も同じ loader を読む」が実際に着地した。`clients/cli` は 5 番目の Go module で、`clients/goclient` を `replace` 経由で import する 3 番目の consumer である。この change の non_goal はそのまま強くなった側にある: CLI は services module への edge を持たず、depguard・`go.mod` assertion・`go list -deps` を歩く `cli-module-boundary` の 3 つで機械的に禁じられている。web-host と同じく、client host は公開 wire contract の consumer であって内部型の consumer ではない。
