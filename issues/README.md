# issues/

軽量な issue トラッカー。1 ファイル 1 issue、`issue-YYYYMMDD-<slug>.md`。

- 先頭に `# タイトル` と `- status: open|closed` / `- created` / `- area` / `- origin`
- 着手したら status を更新し、解決コミット/変更を本文に追記して `closed` にする
- 大きな設計を要するものは dev-docs (docs/changes/) の change package へ昇格する
