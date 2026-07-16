---
id: adr-20260715-test-harness-dependency-strategies
kind: adr
title: External dependency admission validates executable strategies
status: proposed
created: '2026-07-15'
decision_makers:
- unknown
tags:
- testing
- harness
owners: []
relations: []
source_paths:
- test-harness/dependencies.json
- src/internal/harnesspolicy/dependency.go
summary: Triple evidence is AST-validated; hermetic real and trusted runtime exceptions
  are explicit, owned, expiring ADR-backed strategies
---

## Context

2026-07-11 の dependency registry は fake / contract / fidelity の path と marker を持つが、marker の文字列存在しか検証しない。その結果、test function を public fake、production function を fidelity、fake clock advance を real fidelity として登録しても green になった。一方、real git を一時repositoryへ向けるtest、kernel pty、standard-library clockのように、fake再実装より安価でhermeticなreal contractの方が正確な依存もある。

## Decision

dependency admission は次の2 strategyだけを許す。

1. `triple`: production seamとpublic fakeはproduction sourceのexported declaration、T2 contractはalways-on `_test.go` の実test functionと失敗assertion、T3 fidelityは`//go:build e2e`付き`*_e2e_test.go`の実test functionとしてAST検証する。T3 fileは`e2e-suites.json`のdependency_idへ結線する。
2. `exception`: fakeを作らない理由、owner、期限、evidence ADRを必須化する。`hermetic-real`または`trusted-runtime`だけを許し、期限切れ・存在しないADR・空metadataはfailする。

新しいproduction external boundaryはcallsite inventoryへ明示加入し、未登録callsiteをCIで拒否する。exceptionはtripleと同じくprotected changeであり、自己承認できない。

## Consequences

{% consequence kind="positive" %} markerを置くだけの偽tripleが拒否され、fake driftを実際に検出できるdependencyだけがtripleを名乗れる。{% /consequence %}

{% consequence kind="negative" %} Go ASTとbuild constraint検証、callsite inventoryの保守コストが増える。新しい外部境界の追加にはregistry更新が必要になる。{% /consequence %}

{% consequence kind="neutral" %} hermetic real contractはfakeへ置換しない。例外は品質低下ではなく、依存特性に応じた明示strategyとして期限付きで再評価する。{% /consequence %}

## Alternatives

- 全依存へ一律にpublic fakeを強制: git/pty/clockで本物より不正確な再実装を増やすため棄却。
- marker検査を維持しreviewへ委ねる: 現にfalse greenを生成したため棄却。
- registryを廃止してlintだけにする: fake/contract/fidelity間の意味結線と例外理由を保持できないため棄却。

## Confirmation

`go test ./internal/harnesspolicy -run Dependency`、`scripts/check-harness-dependencies.sh`、trusted harness gateがstrategy・AST evidence・callsite inventoryを検証する。
