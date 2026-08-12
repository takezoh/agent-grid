package state

// Repository preparation domain: distinct identities, pure lifecycle events/
// effects, sole Session availability commit (before external I/O).

type (
	ExecutionServerID  string
	GitHubRepositoryID string
	RepositoryOwner    string
	RepositoryName     string
	CreationIntentID   string
	PreparationID      string
	ClonePathID        string
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
	PrepStageConfirmed          PreparationStage = "confirmed"
	PrepStageCloneInProgress    PreparationStage = "clone-in-progress"
	PrepStageCloneReady         PreparationStage = "clone-ready"
	PrepStageFailed             PreparationStage = "failed"
	PrepStageCancelled          PreparationStage = "cancelled"
	PrepStageInterrupted        PreparationStage = "interrupted"
	PrepStageInternalCorruption PreparationStage = "internal_corruption"
	PrepStageAvailable          PreparationStage = "available"
	PrepStageAbandoned          PreparationStage = "abandoned"
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
	PrepFailureLaunchFailed      PreparationFailureCode = "launch_failed"
	PrepFailureCancelled         PreparationFailureCode = "cancelled"
	PrepFailureInterrupted       PreparationFailureCode = "interrupted"
	PrepFailureInternalCorrupt   PreparationFailureCode = "internal_corruption"
	PrepFailureIncompleteCommit  PreparationFailureCode = "incomplete_commit_evidence"
	PrepFailureIllegalTransition PreparationFailureCode = "illegal_transition"
)

type RetryOrigin string

const (
	RetryOriginNone   RetryOrigin = ""
	RetryOriginClone  RetryOrigin = "clone"
	RetryOriginLaunch RetryOrigin = "launch"
)

type CreationIntent struct {
	ID          CreationIntentID
	ServerID    ExecutionServerID
	Repository  GitHubRepositoryIdentity
	Fingerprint string
	Command     string
}

type RepositoryPreparation struct {
	ID                        PreparationID
	IntentID                  CreationIntentID
	ServerID                  ExecutionServerID
	Repository                GitHubRepositoryIdentity
	Stage                     PreparationStage
	FailureCode               PreparationFailureCode
	RetryOrigin               RetryOrigin
	Revision                  uint64
	LastOwnedWorkerGeneration uint64
	ClonePath                 ClonePathID
	SessionID                 SessionID
	CloneDurable              bool
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
	ConnID           ConnID
	ReqID            string
	Kind             string // create|clone_started|clone_ready|failed|cancelled|interrupted|quarantined|retry|commit_available|abandon
	IntentID         CreationIntentID
	PreparationID    PreparationID
	ServerID         ExecutionServerID
	Repository       GitHubRepositoryIdentity
	Fingerprint      string
	ObservedRevision uint64
	ClonePath        ClonePathID
	Code             PreparationFailureCode
	SessionID        SessionID
	Command          string
	CloneDurable     bool
	LaunchSucceeded  bool
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

// EffRepositoryLaunchSession asks the runtime to spawn the session terminal in
// the prepared clone once the clone is durable on disk.
type EffRepositoryLaunchSession struct {
	PreparationID PreparationID
	ServerID      ExecutionServerID
	Repository    GitHubRepositoryIdentity
	Generation    uint64
}
type EffRepositoryStopOwnedWork struct {
	PreparationID PreparationID
	Generation    uint64
}

func (EffRepositoryPersist) isEffect()       {}
func (EffRepositoryStartClone) isEffect()    {}
func (EffRepositoryLaunchSession) isEffect() {}
func (EffRepositoryStopOwnedWork) isEffect() {}
