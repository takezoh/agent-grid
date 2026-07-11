package harnesspolicy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type ProtectedManifest struct {
	Version int             `json:"version"`
	Paths   []ProtectedPath `json:"paths"`
}

type ProtectedPath struct {
	Path            string   `json:"path"`
	Category        string   `json:"category"`
	RequiredMarkers []string `json:"required_markers,omitempty"`
}

type EscalationRequest struct {
	Reason   string   `json:"reason"`
	Owner    string   `json:"owner"`
	Expiry   string   `json:"expiry"`
	Evidence []string `json:"evidence"`
}

type ProtectedFinding struct {
	Path     string `json:"path"`
	Category string `json:"category"`
	Change   string `json:"change"`
	Detail   string `json:"detail"`
}

type TamperingResult struct {
	Status        string             `json:"status"`
	Findings      []ProtectedFinding `json:"findings"`
	RequestErrors []string           `json:"request_errors,omitempty"`
}

type ProtectedTree map[string][]byte

func ParseProtectedManifest(data []byte) (ProtectedManifest, error) {
	var manifest ProtectedManifest
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return manifest, fmt.Errorf("parse protected manifest: %w", err)
	}
	if manifest.Version != 1 || len(manifest.Paths) == 0 {
		return manifest, fmt.Errorf("protected manifest requires version 1 and at least one path")
	}
	seen := make(map[string]bool)
	for _, entry := range manifest.Paths {
		if entry.Path == "" || entry.Category == "" || filepath.IsAbs(entry.Path) || strings.Contains(filepath.ToSlash(entry.Path), "../") {
			return manifest, fmt.Errorf("invalid protected path %q", entry.Path)
		}
		if seen[entry.Path] {
			return manifest, fmt.Errorf("duplicate protected path %q", entry.Path)
		}
		seen[entry.Path] = true
	}
	return manifest, nil
}

func ParseEscalationRequest(data []byte) (EscalationRequest, error) {
	var request EscalationRequest
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return request, fmt.Errorf("parse escalation request: %w", err)
	}
	return request, nil
}

func ReadProtectedTree(root string, manifest ProtectedManifest) (ProtectedTree, error) {
	tree := make(ProtectedTree, len(manifest.Paths))
	for _, entry := range manifest.Paths {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(entry.Path)))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read protected path %s: %w", entry.Path, err)
		}
		tree[entry.Path] = data
	}
	return tree, nil
}

func ClassifyProtectedChanges(manifest ProtectedManifest, base, head ProtectedTree, request *EscalationRequest) TamperingResult {
	result := TamperingResult{Status: "pass", Findings: []ProtectedFinding{}}
	for _, entry := range manifest.Paths {
		baseData, baseOK := base[entry.Path]
		headData, headOK := head[entry.Path]
		if !baseOK {
			result.Findings = append(result.Findings, finding(entry, "invalid-baseline", "path is absent from trusted merge-base"))
			continue
		}
		if !headOK {
			result.Findings = append(result.Findings, finding(entry, "deleted", "restore the merge-base protected path"))
			continue
		}
		for _, marker := range entry.RequiredMarkers {
			if !bytes.Contains(baseData, []byte(marker)) {
				result.Findings = append(result.Findings, finding(entry, "invalid-baseline", fmt.Sprintf("required marker %q is absent from merge-base", marker)))
			} else if !bytes.Contains(headData, []byte(marker)) {
				result.Findings = append(result.Findings, finding(entry, "weakened", fmt.Sprintf("required marker %q was removed", marker)))
			}
		}
		if !bytes.Equal(baseData, headData) {
			result.Findings = append(result.Findings, finding(entry, "modified", "protected content differs from merge-base"))
		}
	}
	result.Findings = uniqueFindings(result.Findings)
	if len(result.Findings) == 0 {
		return result
	}
	result.Status = "review-required"
	result.RequestErrors = validateEscalationRequest(request)
	return result
}

func validateEscalationRequest(request *EscalationRequest) []string {
	if request == nil {
		return []string{"missing escalation request: reason, owner, expiry, and evidence are required"}
	}
	var errs []string
	for field, value := range map[string]string{"reason": request.Reason, "owner": request.Owner, "expiry": request.Expiry} {
		if strings.TrimSpace(value) == "" {
			errs = append(errs, "missing escalation "+field)
		}
	}
	if len(request.Evidence) == 0 {
		errs = append(errs, "missing escalation evidence")
	} else {
		for _, evidence := range request.Evidence {
			if strings.TrimSpace(evidence) == "" {
				errs = append(errs, "empty escalation evidence")
				break
			}
		}
	}
	if request.Expiry != "" {
		if _, err := time.Parse("2006-01-02", request.Expiry); err != nil {
			errs = append(errs, "invalid escalation expiry; use YYYY-MM-DD")
		}
	}
	sort.Strings(errs)
	return errs
}

func finding(entry ProtectedPath, change, detail string) ProtectedFinding {
	return ProtectedFinding{Path: entry.Path, Category: entry.Category, Change: change, Detail: detail}
}

func uniqueFindings(findings []ProtectedFinding) []ProtectedFinding {
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Path != findings[j].Path {
			return findings[i].Path < findings[j].Path
		}
		return findings[i].Change < findings[j].Change
	})
	result := findings[:0]
	for _, item := range findings {
		if len(result) == 0 || result[len(result)-1] != item {
			result = append(result, item)
		}
	}
	return result
}
