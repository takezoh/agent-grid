package state

// Repository preparation domain: distinct identities, pure lifecycle events/
// effects, sole Session availability commit (before external I/O).

type (
	ExecutionServerID       string
	GitHubRepositoryID      string
	RepositoryOwner         string
	RepositoryName          string
	CreationIntentID        string
	PreparationID           string
	ClonePathID             string
	RepositoryEnvironmentID string
)

type TargetSource string

const (
	TargetSourceProject    TargetSource = "project"
	TargetSourceRepository TargetSource = "repository"
)

type GitHubRepositoryIdentity struct {
	ID    GitHubRepositoryID
	Owner RepositoryOwner
	Name  RepositoryName
}

func (r GitHubRepositoryIdentity) Valid() bool {
	return r.ID != "" && r.Owner != "" && r.Name != ""
}

type ProjectTarget struct {
	ServerID ExecutionServerID
	Project  string
}

func (p ProjectTarget) Valid() bool { return p.ServerID != "" && p.Project != "" }

type RepositoryTarget struct{ Repository GitHubRepositoryIdentity }

func (r RepositoryTarget) Valid() bool { return r.Repository.Valid() }

type SessionTarget struct {
	Source     TargetSource
	Project    ProjectTarget
	Repository RepositoryTarget
}

func (t SessionTarget) Valid() bool {
	switch t.Source {
	case TargetSourceProject:
		return t.Project.Valid() && t.Repository.Repository == (GitHubRepositoryIdentity{})
	case TargetSourceRepository:
		return t.Repository.Valid() && t.Project == (ProjectTarget{})
	default:
		return false
	}
}

type PreparationStage string

const (
	PrepStageConfirmed              PreparationStage = "confirmed"
	PrepStageCloneInProgress        PreparationStage = "clone-in-progress"
	PrepStageCloneReady             PreparationStage = "clone-ready"
	PrepStageDevcontainerInProgress PreparationStage = "devcontainer-in-progress"
	PrepStageFailed                 PreparationStage = "failed"
	PrepStageCancelled              PreparationStage = "cancelled"
	PrepStageInterrupted            PreparationStage = "interrupted"
	PrepStageInternalCorruption     PreparationStage = "internal_corruption"
	PrepStageAvailable              PreparationStage = "available"
	PrepStageAbandoned              PreparationStage = "abandoned"
)

func (st PreparationStage) IsSessionAvailable() bool { return st == PrepStageAvailable }
func (st PreparationStage) IsTerminal() bool {
	return st == PrepStageAvailable || st == PrepStageAbandoned || st == PrepStageInternalCorruption
}

type PreparationFailureCode string

const (
	PrepFailureNone              PreparationFailureCode = ""
	PrepFailureInvalidIdentity   PreparationFailureCode = "invalid_identity"
	PrepFailureCloneFailed       PreparationFailureCode = "clone_failed"
	PrepFailureDevcontainerFail  PreparationFailureCode = "devcontainer_failed"
	PrepFailureCancelled         PreparationFailureCode = "cancelled"
	PrepFailureInterrupted       PreparationFailureCode = "interrupted"
	PrepFailureInternalCorrupt   PreparationFailureCode = "internal_corruption"
	PrepFailureIncompleteCommit  PreparationFailureCode = "incomplete_commit_evidence"
	PrepFailureIllegalTransition PreparationFailureCode = "illegal_transition"
)

type RetryOrigin string

const (
	RetryOriginNone         RetryOrigin = ""
	RetryOriginClone        RetryOrigin = "clone"
	RetryOriginDevcontainer RetryOrigin = "devcontainer"
)

type EnvironmentDisposition string

const (
	EnvDispositionNone     EnvironmentDisposition = "none"
	EnvDispositionEnsuring EnvironmentDisposition = "ensuring"
	EnvDispositionReady    EnvironmentDisposition = "ready"
	EnvDispositionReused   EnvironmentDisposition = "reused"
	EnvDispositionFailed   EnvironmentDisposition = "failed"
)

type CreationIntent struct {
	ID          CreationIntentID
	ServerID    ExecutionServerID
	Repository  GitHubRepositoryIdentity
	Fingerprint string
}

type RepositoryPreparation struct {
	ID                        PreparationID
	IntentID                  CreationIntentID
	ServerID                  ExecutionServerID
	Repository                GitHubRepositoryIdentity
	Stage                     PreparationStage
	FailureCode               PreparationFailureCode
	RetryOrigin               RetryOrigin
	EnvironmentDisposition    EnvironmentDisposition
	Revision                  uint64
	LastOwnedWorkerGeneration uint64
	ClonePath                 ClonePathID
	EnvironmentID             RepositoryEnvironmentID
	SessionID                 SessionID
	CloneDurable              bool
	EnvironmentReady          bool
	LaunchSucceeded           bool
}

func (p RepositoryPreparation) IsSessionAvailable() bool {
	return p.Stage.IsSessionAvailable() && p.SessionID != ""
}

func AvailablePreparations(s State) []RepositoryPreparation {
	out := make([]RepositoryPreparation, 0)
	for _, p := range s.RepositoryPreparations {
		if p.IsSessionAvailable() {
			out = append(out, p)
		}
	}
	return out
}

func FindPreparation(s State, id PreparationID) (RepositoryPreparation, bool) {
	if s.RepositoryPreparations == nil || id == "" {
		return RepositoryPreparation{}, false
	}
	p, ok := s.RepositoryPreparations[id]
	return p, ok
}

func putPreparation(s State, p RepositoryPreparation) State {
	s.RepositoryPreparations = cloneMap(s.RepositoryPreparations)
	if s.RepositoryPreparations == nil {
		s.RepositoryPreparations = map[PreparationID]RepositoryPreparation{}
	}
	s.RepositoryPreparations[p.ID] = p
	return s
}

func putCreationIntent(s State, intent CreationIntent) State {
	s.CreationIntents = cloneMap(s.CreationIntents)
	if s.CreationIntents == nil {
		s.CreationIntents = map[CreationIntentID]CreationIntent{}
	}
	s.CreationIntents[intent.ID] = intent
	return s
}

func SessionIsAvailableProjection(s State, id SessionID) bool {
	_, ok := s.Sessions[id]
	return id != "" && ok
}

// EvRepository is the closed lifecycle command set for preparation.
// Kind selects the transition; unused fields are ignored per kind.
type EvRepository struct {
	ConnID            ConnID
	ReqID             string
	Kind              string // create|clone_started|clone_ready|devcontainer_started|failed|cancelled|interrupted|quarantined|retry|commit_available|abandon
	IntentID          CreationIntentID
	PreparationID     PreparationID
	ServerID          ExecutionServerID
	Repository        GitHubRepositoryIdentity
	Fingerprint       string
	ObservedRevision  uint64
	ClonePath         ClonePathID
	Code              PreparationFailureCode
	SessionID         SessionID
	EnvironmentID     RepositoryEnvironmentID
	Command           string
	CloneDurable      bool
	EnvironmentReady  bool
	LaunchSucceeded   bool
	EnvironmentReused bool
}

func (EvRepository) isEvent() {}

type EffRepositoryPersist struct {
	PreparationID PreparationID
	Revision      uint64
}
type EffRepositoryStartClone struct {
	PreparationID PreparationID
	ServerID      ExecutionServerID
	Repository    GitHubRepositoryIdentity
	Generation    uint64
}
type EffRepositoryEnsureEnvironment struct {
	PreparationID PreparationID
	ServerID      ExecutionServerID
	Repository    GitHubRepositoryIdentity
	Generation    uint64
}
type EffRepositoryStopOwnedWork struct {
	PreparationID PreparationID
	Generation    uint64
}

func (EffRepositoryPersist) isEffect()           {}
func (EffRepositoryStartClone) isEffect()        {}
func (EffRepositoryEnsureEnvironment) isEffect() {}
func (EffRepositoryStopOwnedWork) isEffect()     {}
