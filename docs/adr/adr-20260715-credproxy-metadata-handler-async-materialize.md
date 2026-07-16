---
id: adr-20260715-credproxy-metadata-handler-async-materialize
kind: adr
title: 'ADR — credproxy: metadata handler per-request /token write path becomes an
  asynchronous Materialize invocation; HTTP response reflects token-fetch success
  only'
status: accepted
created: '2026-07-15'
tags:
- adr
- credproxy
- gcloudcli
- metadata-handler
- observability
- boundary
owners:
- take.gn
relations:
- {type: partOf, target: change-20260715-credproxy-materialization-contract}
- {type: references, target: adr-20260715-credproxy-materialize-method}
- {type: references, target: adr-20260715-credproxy-retry-owner-caller-side}
- {type: references, target: change-20260715-credproxy-materialization-contract}
source_paths: []
decision_makers:
- take.gn
summary: metadata.go:78 の per-request /token handler で行われていた silent-swallow os.WriteFile
  を廃止し、token fetch 成功直後に go b.Materialize(bgCtx, project) を非同期に発火する。HTTP response
  semantics は従来通り (200 + JSON on token fetch success)、既存 curl-based backstop を保存する。
---

## Context

`metadata.go:78` 近辺の per-request `/token` handler は現状:

```go
// (pre-fix) — silent-swallow of file write error
tokenBytes, err := b.gcpToken(ctx, project)
if err != nil {
    http.Error(w, ...)
    return
}
_ = os.WriteFile(tokenHostPath, tokenBytes, 0o600)   // <— silent swallow
w.WriteHeader(http.StatusOK)
w.Write(tokenBytes)
```

投げつけられている問題は 2 つ:

- **(P1) silent-swallow が RCA の一因**: token 自体は取得成功、write が失敗、しかし caller (container 内 metadata endpoint hit) には 200 が返る → file が host 側に無いことに誰も気付かない。
- **(P2) 既存 backstop が有効に機能している**: container 側が起動時に metadata endpoint を hit すると host 側 token file が生成される (self-heal) — file が消えていても次の hit で復活する。**この endpoint に対する curl-based diagnostics も存在する**。

修正候補は 3 つ:

- **(A) HTTP 500 に昇格** — write error を HTTP response error に昇格。silent-swallow は消える。
- **(B) 同期的に Materialize 呼び出しに置換** — handler 内で `b.Materialize(ctx, project)` を synchronous に呼び、error なら 500。
- **(C) 非同期 Materialize + response は token fetch 成否のみ** — handler は token fetch 成功時に `go b.Materialize(ctx, project)` を non-blocking で発火し、HTTP は従来通り 200 で token bytes を返す。file-write の成否は Materialize の返り値経路 (SpecBuilder 内部で slog + Runner が observe できる signal に接続) で propagate する。

## Decision

**(C) を採用する。** metadata handler は次の形になる:

```go
tokenBytes, err := b.gcpToken(ctx, project)
if err != nil {
    http.Error(w, ...)
    return
}

// Fire-and-forget Materialize: the HTTP response semantics remain
// unchanged (200 + token bytes when token fetch succeeded), but the
// write-side effect is now attributed to a single SSOT method whose
// outcome flows to the caller-side observability channel
// (agent-grid Runner slog + ReadinessSnapshot).
go func() {
    // Use a background ctx tied to the SpecBuilder's lifetime, not the
    // per-request reqCtx (which is short-lived and would race the
    // HTTP response). The Materialize implementation SHALL respect
    // this ctx's cancellation for cleanup.
    if err := b.Materialize(b.bgCtx, project); err != nil {
        // Materialize itself is responsible for structured logging;
        // this branch is a no-op from the handler's perspective.
    }
}()

w.WriteHeader(http.StatusOK)
w.Write(tokenBytes)
```

- `b.Materialize(bgCtx, project)` は `adr-20260715-credproxy-materialize-method` で定義される single SSOT method を呼ぶ。file write は `Materialize` 実装が所有。
- `bgCtx` は SpecBuilder の lifetime に紐付いた long-lived context (既存の `b.rootCtx` を使うか、あるいは Materialize 呼び出し用に短命 `context.WithTimeout` を派生)。**per-request `reqCtx` は使わない** — HTTP response が返った瞬間に cancel される可能性があり、Materialize が中断されるため。
- Materialize は `adr-20260715-credproxy-retry-owner-caller-side` により内部 retry を持たない (fail fast)。file write が失敗すれば error を返し、その error は上記 `if err != nil` branch で observe されるだけ (goroutine 内)。caller-side retry は行わない (retry は agent-grid の `resolveOverlaySpecs` が envelope 内で組む方針で、この async 経路は best-effort backstop の位置付け)。
- **HTTP response は従来通り**: token fetch 成功 → 200 + JSON、token fetch 失敗 → 4xx/5xx。file write の成否は response に影響しない。既存 curl-based backstop / container 側 self-heal 挙動が壊れない。
- silent-swallow は消える: file-write error は `Materialize` の返り値として明示され、`Materialize` 実装内部の structured slog に流れ、agent-grid Runner の distinguishable warn line + `ReadinessSnapshot` にも次回の Materialize 呼び出し (次の frame launch) を通じて surface する。

## Consequences

- **silent-swallow の根絶**: `_ = os.WriteFile(...)` パターンは code base から消える。write error は必ず error 経路 (Materialize 返り値 → structured slog → 次回 Materialize 呼び出し時の caller-side observability) に接続される。
- **既存 backstop 保存**: container 側の metadata endpoint hit → response 200 + JSON という expected shape は不変。curl-based diagnostic scripts、container-side self-heal 挙動、既存の integration test はいずれも regressed しない。
- **endpoint latency 不変**: `go func() { ... }` は返り値を待たないので、`Materialize` の実行時間 (`os.WriteFile` + gcpToken の potential retry 等) が HTTP latency に加算されない。metadata endpoint は latency-sensitive path (container 起動時に呼ばれる) なので重要。
- **backpressure 未対応**: 大量の per-request `/token` request が同時に来て `go func` が大量に spawn されるケースは想定外だが、単一 devcontainer 環境では endpoint hit 頻度が低い (起動時数回) ため実用上 problem-free。future work として errgroup / sync.Semaphore で bound することは可能 (Open Question に保留)。
- **`bgCtx` の選択は実装詳細**: `b.rootCtx` を使うか、SpecBuilder 内で `context.Background()` から派生させるか、あるいは短い timeout を hardcoded で持たせるかは Materialize 実装との協調で決まる (Open Question に保留)。
- **既存 test 影響 minimal**: `metadata_test.go` の per-request 経路のテストは HTTP response shape を確認しているので regressed しない。新たに Materialize が呼ばれることを assert する test は Materialize の T1 test 側で追加する。

## Alternatives

### (A) HTTP 500 に昇格

**却下**: token 自体は fetch 成功したのに response が 5xx になる新規故障モードを生む。既存 curl-based backstop / container 側 self-heal が壊れる。RCA が指摘した silent-swallow を消す goal は達成できるが、他の regression コストが大きい。

### (B) 同期的に Materialize 呼び出し + error なら 500

**却下**: (A) と同型の response semantics 変化に加えて、endpoint latency が Materialize の実行時間ぶん増える。metadata endpoint は container 起動 critical path で latency-sensitive。加えて Materialize は idempotent かつ fail-fast なので、この handler で待つ意味が薄い (次回の Materialize 呼び出しで retry される)。

### 完全に removal (write は一切しない)

**却下**: container 側 self-heal の trigger 経路が消える。file 消失時に container 側から復旧できなくなる。既存 backstop の完全喪失。

### 別 goroutine ではなく worker queue で serialize

**却下**: 現状の endpoint hit 頻度では overengineering。将来 backpressure が問題になれば追加できる (Open Question)。
