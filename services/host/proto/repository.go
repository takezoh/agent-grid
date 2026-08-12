package proto

import (
	"encoding/json"
	"fmt"
)

// Source-discriminated targets and preparation wire types.

type TargetSource string

const (
	TargetSourceProject    TargetSource = "project"
	TargetSourceRepository TargetSource = "repository"
)

type GitHubRepositoryWire struct {
	ID    string `json:"repository_id"`
	Owner string `json:"owner"`
	Name  string `json:"name"`
}

type ProjectTargetWire struct {
	ServerID string `json:"server_id"`
	Project  string `json:"project"`
}

type SessionTargetWire struct {
	Source     TargetSource          `json:"source"`
	Project    *ProjectTargetWire    `json:"project,omitempty"`
	Repository *GitHubRepositoryWire `json:"repository,omitempty"`
}

func EncodeSessionTarget(t SessionTargetWire) ([]byte, error) {
	if err := ValidateSessionTarget(t); err != nil {
		return nil, err
	}
	return json.Marshal(t)
}

func DecodeSessionTarget(raw []byte) (SessionTargetWire, error) {
	var t SessionTargetWire
	if err := json.Unmarshal(raw, &t); err != nil {
		return SessionTargetWire{}, fmt.Errorf("proto: session target: %w", err)
	}
	if err := ValidateSessionTarget(t); err != nil {
		return SessionTargetWire{}, err
	}
	return t, nil
}

func ValidateSessionTarget(t SessionTargetWire) error {
	switch t.Source {
	case TargetSourceProject:
		if t.Project == nil || t.Project.ServerID == "" || t.Project.Project == "" {
			return fmt.Errorf("proto: project target requires server_id and project")
		}
		if t.Repository != nil {
			return fmt.Errorf("proto: project target must not carry repository identity")
		}
	case TargetSourceRepository:
		if t.Repository == nil || t.Repository.ID == "" || t.Repository.Owner == "" || t.Repository.Name == "" {
			return fmt.Errorf("proto: repository target requires repository_id, owner, and name")
		}
		if t.Project != nil {
			return fmt.Errorf("proto: repository target must not carry project identity")
		}
	default:
		return fmt.Errorf("proto: unknown target source %q", t.Source)
	}
	return nil
}

type PreparationStageWire string

const (
	PrepWireConfirmed          PreparationStageWire = "confirmed"
	PrepWireCloneInProgress    PreparationStageWire = "clone-in-progress"
	PrepWireCloneReady         PreparationStageWire = "clone-ready"
	PrepWireFailed             PreparationStageWire = "failed"
	PrepWireCancelled          PreparationStageWire = "cancelled"
	PrepWireInterrupted        PreparationStageWire = "interrupted"
	PrepWireInternalCorruption PreparationStageWire = "internal_corruption"
	PrepWireAvailable          PreparationStageWire = "available"
	PrepWireAbandoned          PreparationStageWire = "abandoned"
)

type RepositoryPreparationWire struct {
	PreparationID string               `json:"preparation_id"`
	IntentID      string               `json:"intent_id"`
	ServerID      string               `json:"server_id"`
	Repository    GitHubRepositoryWire `json:"repository"`
	Stage         PreparationStageWire `json:"stage"`
	FailureCode   string               `json:"failure_code,omitempty"`
	RetryOrigin   string               `json:"retry_origin,omitempty"`
	Revision      uint64               `json:"revision"`
	SessionID     string               `json:"session_id,omitempty"`
	ClonePath     string               `json:"clone_path,omitempty"`
}

func EncodeRepositoryPreparation(p RepositoryPreparationWire) ([]byte, error) {
	if err := ValidateRepositoryPreparation(p); err != nil {
		return nil, err
	}
	return json.Marshal(p)
}

func DecodeRepositoryPreparation(raw []byte) (RepositoryPreparationWire, error) {
	var p RepositoryPreparationWire
	if err := json.Unmarshal(raw, &p); err != nil {
		return RepositoryPreparationWire{}, fmt.Errorf("proto: repository preparation: %w", err)
	}
	if err := ValidateRepositoryPreparation(p); err != nil {
		return RepositoryPreparationWire{}, err
	}
	return p, nil
}

func ValidateRepositoryPreparation(p RepositoryPreparationWire) error {
	if p.PreparationID == "" || p.IntentID == "" || p.ServerID == "" {
		return fmt.Errorf("proto: preparation requires preparation_id, intent_id, server_id")
	}
	if p.Repository.ID == "" || p.Repository.Owner == "" || p.Repository.Name == "" {
		return fmt.Errorf("proto: preparation requires fully-qualified repository identity")
	}
	switch p.Stage {
	case PrepWireConfirmed, PrepWireCloneInProgress, PrepWireCloneReady,
		PrepWireFailed, PrepWireCancelled,
		PrepWireInterrupted, PrepWireInternalCorruption, PrepWireAvailable, PrepWireAbandoned:
	default:
		return fmt.Errorf("proto: unknown preparation stage %q", p.Stage)
	}
	if p.Stage == PrepWireAvailable && p.SessionID == "" {
		return fmt.Errorf("proto: available preparation requires session_id")
	}
	if p.Stage != PrepWireAvailable && p.SessionID != "" {
		return fmt.Errorf("proto: non-available preparation must not publish session_id")
	}
	return nil
}
