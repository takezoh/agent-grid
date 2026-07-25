# windows-shell: cross-client wire parity テストの敷設 (hello 黙殺の再発防止)

- status: open
- created: 2026-07-25
- area: clients/windows-shell (WsFrameParser) / src/server/api (wire fixtures)
- origin: debug-run-shell-zero-sessions (`~/.dev-skills/debug/shell-zero-sessions/`)

## 背景

「Shell が常に 0 sessions」の第一機構は、gateway の `k:"h"` hello フレームを
`WsFrameParser` が未知フレームとして黙って捨てていたこと。原因の構造面は:

1. **契約の暗黙化** — hello/delta の分節が Go コメントとテストにしか存在しない
2. **境界の空白** — cross-client wire 契約テスト (`wire_fixtures_test.go`) の対象が
   `clients/ui` (TS) だけで、windows-shell (C#) はどのハーネスにも守られていない

修正で doc コメントには明文化済みだが、**機械的強制がない** — 新しい `k` 値が
増えたとき、C# パーサだけ黙って取りこぼす事故は今も再現し得る。

## 提案

- server 側の golden wire fixtures (`wire_fixtures_test.go` が使う JSON) を
  windows-shell の Core.Tests からも読み、`WsFrameParser.TryParse` が
  「既知フレーム全種を drop しない」ことを固定するパリティテストを追加する
- fixture 置き場は repo 内共有パス (テスト時に相対パスで読む。wire 型は
  stdlib-only 制約に抵触しない — テストのみ)
- 併せて NamedPipeClientStream の実例外型 (OperationCanceledException) を pin する
  Windows T3 fidelity テスト (debug-run-workspace-not-launched の手動 probe の常設化)
  もこのハーネス整備に含める

## 参考

- 検討には dev:test-harness スキル (外部依存 3 点セット / 二重 enforcement) を使う
