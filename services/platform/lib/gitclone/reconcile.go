package gitclone

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// ReconcileDisposition classifies durable clone evidence after restart/crash.
type ReconcileDisposition string

const (
	ReconcileCloneReady   ReconcileDisposition = "clone_ready"
	ReconcileRetryClone   ReconcileDisposition = "retry_clone"
	ReconcileConflict     ReconcileDisposition = "clone_identity_conflict"
	ReconcileUnsupported  ReconcileDisposition = "atomic_publish_unsupported"
	ReconcileQuarantine   ReconcileDisposition = "internal_corruption"
)

// ReconcileRequest inspects staging/final evidence for one intent.
type ReconcileRequest struct {
	StagingDir string
	FinalDir   string
	// ExpectedMarker is the generation-bound marker for this intent; empty accepts any marker on final.
	ExpectedMarker string
	// Root is the canonical repository sessions root; final/staging must be contained.
	Root string
	FS   FilesystemProcess
}

// ReconcileResult is the deterministic recovery decision.
type ReconcileResult struct {
	Disposition ReconcileDisposition
	FinalDir    string
	Marker      string
	Diagnostic  string
}

// ReconcileCloneEvidence applies alg-clone-publication-reconciliation.
func ReconcileCloneEvidence(req ReconcileRequest) ReconcileResult {
	fs := req.FS
	if fs == nil {
		fs = OSFilesystem{}
	}
	if req.FinalDir == "" || req.StagingDir == "" {
		return ReconcileResult{Disposition: ReconcileQuarantine, Diagnostic: "paths required"}
	}
	if req.Root != "" {
		if !pathContained(req.Root, req.FinalDir) || !pathContained(req.Root, req.StagingDir) {
			return ReconcileResult{Disposition: ReconcileQuarantine, Diagnostic: "path escapes root"}
		}
	}
	finalMarker, finalOK, finalErr := readMarker(fs, req.FinalDir)
	if finalErr != nil && !errors.Is(finalErr, os.ErrNotExist) {
		return ReconcileResult{Disposition: ReconcileQuarantine, Diagnostic: finalErr.Error()}
	}
	if finalOK {
		if req.ExpectedMarker != "" && finalMarker != req.ExpectedMarker {
			return ReconcileResult{Disposition: ReconcileConflict, FinalDir: req.FinalDir, Marker: finalMarker, Diagnostic: "marker mismatch"}
		}
		return ReconcileResult{Disposition: ReconcileCloneReady, FinalDir: req.FinalDir, Marker: finalMarker}
	}
	// Staging present without durable final → retry clone (clean staging).
	if _, err := fs.Stat(req.StagingDir); err == nil {
		_ = fs.RemoveAll(req.StagingDir)
		return ReconcileResult{Disposition: ReconcileRetryClone, Diagnostic: "incomplete staging"}
	}
	// Nothing durable.
	return ReconcileResult{Disposition: ReconcileRetryClone, Diagnostic: "no durable final"}
}

func readMarker(fs FilesystemProcess, dir string) (string, bool, error) {
	if _, err := fs.Stat(dir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", false, err
		}
		return "", false, err
	}
	b, err := fs.ReadFile(filepath.Join(dir, MarkerFileName))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Final tree without marker is conflicting durable evidence.
			return "", false, nil
		}
		return "", false, err
	}
	m := strings.TrimSpace(string(b))
	if m == "" {
		return "", false, nil
	}
	return m, true, nil
}

func pathContained(root, target string) bool {
	r, t := filepath.Clean(root), filepath.Clean(target)
	if r == t {
		return true
	}
	prefix := r + string(os.PathSeparator)
	return strings.HasPrefix(t, prefix)
}
