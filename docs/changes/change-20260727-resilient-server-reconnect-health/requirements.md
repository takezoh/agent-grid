---
change: change-20260727-resilient-server-reconnect-health
role: requirements
functional_requirements:
- id: FR-REC-001
  statement: When an initiating connection fails or a hello-confirmed connection closes
    unexpectedly, the shared UI shall run no more than sixteen automatic retries and
    then expose closed with Retry.
  priority: must
- id: FR-REC-002
  statement: The active connection shall never publish nominal or recovered state
    before a daemon-derived hello from the current lifecycle generation.
  priority: must
- id: FR-REC-003
  statement: Retry shall start exactly one fresh reload-free lifecycle with fresh
    accounting and shall become reusable after another exhaustion.
  priority: must
- id: FR-REC-004
  statement: The persistent StatusBar shall be the only active-recovery and Retry
    owner and shall preserve no-session, keyboard, pointer, assistive-technology,
    and focus reachability.
  priority: must
- id: FR-FLEET-001
  statement: Exactly one host FleetRuntime shall own enabled-member health generation
    and runtime-only publication; views shall not probe or persist health.
  priority: must
- id: FR-FLEET-002
  statement: Every configured server shall have exactly one exclusive qualified observation;
    exact protocol mismatch is terminal for that member generation and Health unavailable
    may retain only a nested stale prior observation.
  priority: must
- id: FR-FLEET-003
  statement: Degraded enabled-fleet health shall produce correct partial or total
    wording and Connections diagnosis in both hosts, including an operable browser
    configuration-path copy action and restart instruction.
  priority: must
- id: FR-FLEET-004
  statement: Active connection status and fleet aggregate status shall remain independent
    so recovery of one axis cannot erase degradation on the other.
  priority: must
- id: NFR-LIFE-001
  statement: Retired active and host-member generations shall release or fence fetches,
    timers, sockets, channels, callbacks, pending requests, and publication.
  priority: must
- id: NFR-HEALTH-001
  statement: Capabilities, ticket, and hello phases shall each terminate within three
    seconds; Connected evidence expires after five seconds, and concurrent/render
    consumers reuse one host snapshot without multiplying work.
  priority: must
- id: NFR-HOST-001
  statement: Web and Workspace shall normalize equal evidence to equal credential-free
    health meaning while differing only in delivery transport.
  priority: must
- id: NFR-TEST-001
  statement: Lifecycle races, host parity, and external gateway evidence shall have
    deterministic fake, invariant-contract, and FakeVsReal verification seams.
  priority: must
---

<!-- lifecycle is owned by change.md -->

# Requirements

## Functional requirements

- `FR-REC-001` (event/state-driven): initiating connectionがhello前に失敗した場合、またはhello確認済みconnectionが予期せず閉じた場合、shared UIはinitiating attemptとは別に最大16回だけ自動再試行し、16回目がhelloなしで失敗した直後に`closed`とRetryを表示しなければならない。
- `FR-REC-002` (invariant): active connectionは、現在のlifecycle generationかつattempt sequenceに属するdaemon由来helloを受信するまでnominal/open/recoveredを公開してはならない。WebSocket open、ticket成功、retired attemptのhelloは復旧証拠ではない。
- `FR-REC-003` (state-driven): 自動復旧枯渇中にRetryを実行した場合、UIは同期的にprogressへ入り、新しいlifecycle generationと再試行会計で1つだけ接続を開始しなければならない。そのlifecycleも枯渇した場合、ReloadなしでRetryを再び利用可能にしなければならない。
- `FR-REC-004` (invariant): app shellが存在する間、active recovery状態とRetryはsession未選択時を含めpersistent StatusBarだけが所有しなければならない。状態文は専用のpolite status region、Retryはその外のsibling native buttonとし、focused Retry消滅時だけpersistent StatusBar landmarkへfocusを移す。Header、Sidebar、toast、modal、ConnectionsはRetryを重複所有してはならない。
- `FR-FLEET-001` (invariant): host processごとに既存application-scoped `FleetRuntime` 1つだけがenabled memberのhealth generation、freshness、revision、publicationを所有しなければならない。SPAとsettingsはsnapshotを参照するだけで、独立probeやhealth永続化をしてはならない。
- `FR-FLEET-002` (event/state-driven): health evidence変更時、hostは全configured serverをstable `serverId`で、current Checking/Connected/Unavailable/Incompatible protocol、freshnessを持たないDisabled、またはoptionalなnested stale priorを持つHealth unavailableの排他的unionとして公開しなければならない。exact protocol mismatchは当該member generationでterminalとし、config/process generation変更までoperational/reconnect workを開始してはならない。
- `FR-FLEET-003` (state-driven): enabled fleetがdegradedな場合、StatusBarはpartial unavailable/incompatibleとall-enabled-unavailableを区別し、pointer/keyboardで利用できるConnectionsを表示しなければならない。両hostのdiagnosisは全serverのqualified state/reasonを表示し、Workspace設定操作とbrowserの`Copy configuration path`＋restart-required instructionをhealth failure中も利用可能にしなければならない。
- `FR-FLEET-004` (invariant): active connectionとenabled-fleet aggregateは独立にreduceしなければならない。active helloは他member degradationを隠さず、fleet recoveryは独立してreconnecting/closedなactive connectionを変更してはならない。

## Non-functional requirements

- `NFR-LIFE-001`: active lifecycle/attemptまたはhost member generationのretire/shutdownはfetch、phase/retry timer、socket、channel、callback、pending requestをcancel/closeまたは二重ID fenceし、retired workによるpublicationを禁止しなければならない。
- `NFR-HEALTH-001`: capabilities、ticket、hello waitは各3秒以内に終了しなければならない。Connected evidenceは5秒で失効し、同じworkerがbounded refreshする。consumer/render数はworker、probe、timer、socket数を増やしてはならない。
- `NFR-HOST-001`: 同一config/member evidenceはWebとWorkspaceでdeep-equalなcredential-free health meaningへnormalizeしなければならない。差異はsame-origin HTTP/WSとpreload/IPC transportだけとし、bearer、URL userinfo、ticket、raw transport errorをwire、DOM、health logへ出してはならない。
- `NFR-TEST-001`: retry会計、二重generation race、health union/freshness、codec parity、resource closure、redactionをinjected clock/scheduler/fetch/socketとcanonical fixtureで決定的に検証可能にしなければならない。変更する外部gateway fakeにはfake、invariant-naming contract test、`FakeVsRealFleetHealth*` backstopを揃えなければならない。

## Acceptance trace

- `UAC-001`〜`UAC-006`: established/initial failure、open-without-hello、exact 16 retries、repeatable Retry、no-session accessibility、StatusBar sole ownershipを検証する。
- `UAC-007`〜`UAC-014`: enabled denominator、partial/total outage、両host Connections、browser edit path、independent axes、Health unavailable＋stale prior、current all-healthy recoveryを検証する。
