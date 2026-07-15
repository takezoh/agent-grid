# Remote Control セッションをモバイルで直接開くための調査メモ

## 目的

Ubuntu Server などで起動した Claude Code / Codex の Remote Control セッションについて、通知や外部リンクからモバイルアプリ上の対象セッション画面を直接開けるかを整理する。

## 想定ユースケース

```text
Ubuntu Server
  └─ Remote Control セッションを開始
       └─ session ID / thread ID を取得
            └─ 通知へリンクを埋め込む
                 └─ モバイルで通知をタップ
                      └─ 対象セッション画面を直接開く
```

デスクトップアプリは使用しない。

## 結論

### Claude

Claude の Remote Control は、セッション ID を含むリンクからモバイルアプリ上の対象セッションを直接開ける。

したがって、外部通知から特定の Remote Control セッションへ直接遷移する構成を実現できる。

### Codex

Codex の Remote Control は、ChatGPT モバイルアプリ上でリモートセッション一覧を表示し、一覧から対象セッションを選択して開くことはできる。

一方、現時点で確認できた公開インターフェースには、thread ID や remote session ID を指定して ChatGPT モバイルアプリ上の対象画面を直接開く Deep Link / Universal Link はない。

したがって、Codex では次の導線になる。

```text
ChatGPT モバイルアプリを開く
  └─ Remote Control の接続先を開く
       └─ セッション一覧を表示
            └─ 対象セッションを手動で選択
```

## Codex の `codex://threads/...` について

`codex://threads/new` や `codex://threads/<thread-id>` は Codex デスクトップアプリ向けの Deep Link であり、Ubuntu Server 上の Remote Control セッションを ChatGPT モバイルアプリで直接開くためのものではない。

そのため、デスクトップアプリを使用しない今回の要件には適用できない。

## App Server の thread ID との違い

Codex App Server は内部プロトコルとして thread ID を扱い、`thread/start`、`thread/list`、`thread/read`、`thread/resume` などでスレッドを識別できる。

ただし、App Server が thread ID を扱えることと、ChatGPT モバイルアプリが外部リンクからその thread ID を受け取って対象画面へ遷移できることは別問題である。

現状不足しているのは、概念的には次のようなモバイル向け公開エントリーポイントである。

```text
chatgpt://codex/remote/thread/<thread-id>
```

上記は説明用の仮想例であり、実在する公開スキーマではない。

## 機能比較

| 機能 | Claude Remote Control | Codex Remote Control |
|---|---:|---:|
| モバイルでリモートセッション一覧を表示 | 可能 | 可能 |
| 一覧から対象セッションを開く | 可能 | 可能 |
| セッション ID を指定して直接開く | 可能 | 公開手段なし |
| デスクトップアプリなしで利用 | 可能 | 可能 |
| 外部通知から対象セッションへワンタップ遷移 | 可能 | 現状不可 |

## agent-grid への示唆

agent-grid から Remote Control セッションへの導線を提供する場合、サービスごとに UX を分ける必要がある。

### Claude

- session ID を保持する
- セッション固有リンクを生成する
- 通知や UI の `Open session` から直接開く

### Codex

- thread ID を保持しても、モバイル画面への直接遷移には使用できない
- `Open ChatGPT` または `Open Remote Control` としてアプリの入口まで案内する
- 対象セッション名、作業ディレクトリ、開始時刻などを通知に表示し、一覧上で識別しやすくする
- 将来モバイル向け Deep Link が公開された場合に差し替えられるよう、リンク生成を provider abstraction の背後に置く

## 推奨インターフェース

```ts
interface RemoteSessionLinkProvider {
  getDirectSessionUrl(session: RemoteSession): string | null;
  getRemoteControlEntryUrl(): string | null;
}
```

- Claude は `getDirectSessionUrl()` を返す
- Codex は現状 `getDirectSessionUrl()` を `null` とし、利用可能なら `getRemoteControlEntryUrl()` のみを返す
- UI は direct link の有無に応じて表示文言を変える

## 未確認事項

- Codex / ChatGPT モバイルアプリ内部に未公開 Deep Link が存在する可能性
- Codex Remote Control の正式公開時にモバイル向け URL スキーマが追加される可能性
- iOS と Android で対応状況が異なる可能性

未公開挙動には依存せず、公開仕様として確認できる範囲だけを製品要件に採用する。
