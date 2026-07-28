package gitclone

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Publication outcome codes (clone-publication-transaction contract).
const (
	CodeAtomicPublishOK          = "ok"
	CodeAtomicPublishUnsupported = "atomic_publish_unsupported"
	CodeCloneIdentityConflict    = "clone_identity_conflict"
	CodePublishFailed            = "publish_failed"
	CodePublishCancelled         = "publish_cancelled"
)

// MarkerFileName is written beside the clone before rename and must match on reconcile.
const MarkerFileName = ".agent-grid-clone-marker"

// PublishRequest describes a same-filesystem staging → final clone publication.
type PublishRequest struct {
	// StagingDir is the fully-cloned temporary tree (must not equal FinalDir).
	StagingDir string
	// FinalDir is the durable session clone path.
	FinalDir string
	// Marker is an immutable identity binding intent/generation (non-empty).
	Marker string
	// FS is optional; nil uses OSFilesystem.
	FS FilesystemProcess
}

// PublishResult is the typed outcome of atomic clone publication.
type PublishResult struct {
	Code       string
	Diagnostic string
	FinalDir   string
	Marker     string
}

// PublishClone atomically publishes a staged clone:
// staging content durability → marker durability → rename → parent fsync.
// Conflicting finals are never overwritten.
func PublishClone(req PublishRequest) PublishResult {
	fs := req.FS
	if fs == nil {
		fs = OSFilesystem{}
	}
	if req.StagingDir == "" || req.FinalDir == "" || req.Marker == "" {
		return PublishResult{Code: CodePublishFailed, Diagnostic: "staging, final, and marker required"}
	}
	if filepath.Clean(req.StagingDir) == filepath.Clean(req.FinalDir) {
		return PublishResult{Code: CodePublishFailed, Diagnostic: "staging and final must differ"}
	}
	if err := ensureContainedSibling(req.StagingDir, req.FinalDir); err != nil {
		return PublishResult{Code: CodePublishFailed, Diagnostic: err.Error()}
	}
	if st, err := fs.Stat(req.FinalDir); err == nil && st != nil {
		return PublishResult{Code: CodeCloneIdentityConflict, Diagnostic: "final path exists", FinalDir: req.FinalDir}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return PublishResult{Code: CodePublishFailed, Diagnostic: err.Error()}
	}
	if err := fs.SyncTree(req.StagingDir); err != nil {
		return PublishResult{Code: CodePublishFailed, Diagnostic: "staging_sync: " + err.Error()}
	}
	markerPath := filepath.Join(req.StagingDir, MarkerFileName)
	if err := fs.WriteFileAtomic(markerPath, []byte(req.Marker), 0o644); err != nil {
		return PublishResult{Code: CodePublishFailed, Diagnostic: "marker_write: " + err.Error()}
	}
	if err := fs.SyncFile(markerPath); err != nil {
		return PublishResult{Code: CodePublishFailed, Diagnostic: "marker_sync: " + err.Error()}
	}
	parent := filepath.Dir(req.FinalDir)
	if err := fs.MkdirAll(parent, 0o755); err != nil {
		return PublishResult{Code: CodePublishFailed, Diagnostic: "mkdir_parent: " + err.Error()}
	}
	if err := fs.Rename(req.StagingDir, req.FinalDir); err != nil {
		if isCrossDevice(err) {
			return PublishResult{Code: CodeAtomicPublishUnsupported, Diagnostic: err.Error()}
		}
		return PublishResult{Code: CodePublishFailed, Diagnostic: "rename: " + err.Error()}
	}
	if err := fs.SyncDir(parent); err != nil {
		if errors.Is(err, ErrDirSyncUnsupported) {
			return PublishResult{Code: CodeAtomicPublishUnsupported, Diagnostic: err.Error(), FinalDir: req.FinalDir, Marker: req.Marker}
		}
		return PublishResult{Code: CodePublishFailed, Diagnostic: "parent_sync: " + err.Error(), FinalDir: req.FinalDir, Marker: req.Marker}
	}
	return PublishResult{Code: CodeAtomicPublishOK, FinalDir: req.FinalDir, Marker: req.Marker}
}

func ensureContainedSibling(staging, final string) error {
	sp, fp := filepath.Clean(staging), filepath.Clean(final)
	if strings.HasPrefix(sp+string(os.PathSeparator), fp+string(os.PathSeparator)) ||
		strings.HasPrefix(fp+string(os.PathSeparator), sp+string(os.PathSeparator)) {
		return fmt.Errorf("staging and final must not nest")
	}
	return nil
}

func isCrossDevice(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "cross-device") || strings.Contains(msg, "invalid cross-device") || strings.Contains(msg, "exdev")
}
