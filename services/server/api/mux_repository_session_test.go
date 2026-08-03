package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/takezoh/agent-grid/services/host/config"
	"github.com/takezoh/agent-grid/services/host/proto"
	"github.com/takezoh/agent-grid/services/host/state"
)

func repositoryProfile(t *testing.T) (string, string) {
	t.Helper()
	profiles := config.DefaultConfig().Session.ResolveBaseProfiles()
	if len(profiles) == 0 {
		t.Fatal("default repository command profile is missing")
	}
	return profiles[0].ID, profiles[0].Revision
}

type authorityCapture struct {
	serverID, repositoryID, owner, name, username, token string
}

func (a *authorityCapture) Offer(serverID, repositoryID, owner, name, username, token string) error {
	a.serverID, a.repositoryID, a.owner, a.name = serverID, repositoryID, owner, name
	a.username, a.token = username, token
	return nil
}

func TestCreateRepositorySessionRoutesToRepositoryDomain(t *testing.T) {
	daemon, fake := newDaemonPair(t)
	fake.pumpResponses(func(reqID string) { fake.sendResp(reqID, proto.RespOK{}) })
	handler := newMux(daemon, "secret", false, defaultMuxDependencies())
	profileID, revision := repositoryProfile(t)
	body := fmt.Sprintf(`{"intent_id":"intent-1","preparation_id":"prep-1","server_id":"srv-1","repository_id":"42","owner":"octo","name":"repo","fingerprint":"42:srv-1","version":2,"profile_id":%q,"profile_revision":%q}`, profileID, revision)
	req := httptest.NewRequest(http.MethodPost, "/api/repository-sessions", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set(ProtocolVersionHeader, ProtocolVersion)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response["preparation_id"] != "prep-1" || response["stage"] != "confirmed" {
		t.Fatalf("response=%v", response)
	}
	if !state.IsRegisteredEvent(state.EventCreateRepositorySession) {
		t.Fatal("repository create event is not registered")
	}
}

func TestSessionConfigPublishesParameterlessBaseProfileAsArray(t *testing.T) {
	cfg := config.DefaultConfig()
	req := httptest.NewRequest(http.MethodGet, "/api/session-config", nil)
	rec := httptest.NewRecorder()
	handleSessionConfig(func() (*config.Config, error) { return cfg, nil })(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		BaseProfiles []struct {
			ID         string          `json:"id"`
			Parameters json.RawMessage `json:"parameters"`
		} `json:"base_command_profiles"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.BaseProfiles) == 0 || payload.BaseProfiles[0].ID != "shell" {
		t.Fatalf("base profiles=%s", rec.Body.String())
	}
	if string(payload.BaseProfiles[0].Parameters) != "[]" {
		t.Fatalf("parameterless profile shape=%s", payload.BaseProfiles[0].Parameters)
	}
}

func TestCreateRepositorySessionMaterializesSelectedBaseProfile(t *testing.T) {
	daemon, fake := newDaemonPair(t)
	cfg := config.DefaultConfig()
	cfg.Session.CommandProfiles = []config.CommandProfileConfig{{
		ID: "agent", Label: "Agent", Executable: "agent-bin",
		Argv: []string{"--model", "{{model}}"},
		Parameters: []config.CommandParameterConfig{{
			Name: "model", Label: "Model", Type: config.ParameterEnum,
			Required: true, Default: "fast", Values: []string{"fast", "deep"},
		}},
	}}
	profile := cfg.Session.ResolveBaseProfiles()[0]
	commandCh := make(chan string, 1)
	go func() {
		env := fake.recv()
		defer fake.sendResp(env.ReqID, proto.RespOK{})
		decoded, err := proto.DecodeCommand(env)
		if err != nil {
			commandCh <- "decode error: " + err.Error()
			return
		}
		cmd, ok := decoded.(proto.CmdEvent)
		if !ok {
			commandCh <- fmt.Sprintf("unexpected command %T", decoded)
			return
		}
		var params state.RepositoryCreateParams
		if err := json.Unmarshal(cmd.Payload, &params); err != nil {
			commandCh <- "payload error: " + err.Error()
			return
		}
		commandCh <- params.Command
	}()
	deps := defaultMuxDependencies()
	deps.sessionConfigLoader = func() (*config.Config, error) { return cfg, nil }
	handler := newMux(daemon, "secret", false, deps)
	body := fmt.Sprintf(`{"intent_id":"intent-1","preparation_id":"prep-1","server_id":"srv-1","repository_id":"42","owner":"octo","name":"repo","version":2,"profile_id":"agent","profile_revision":%q,"parameters":{"model":"deep"}}`, profile.Revision)
	req := httptest.NewRequest(http.MethodPost, "/api/repository-sessions", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set(ProtocolVersionHeader, ProtocolVersion)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if got := <-commandCh; got != "agent-bin --model deep" {
		t.Fatalf("materialized command=%q", got)
	}
}

func TestGetRepositoryPreparationReturnsLatestDaemonProjection(t *testing.T) {
	daemon, fake := newDaemonPair(t)
	fake.pumpResponses(func(reqID string) {
		fake.sendResp(reqID, proto.RespRepositoryPreparation{
			PreparationID: "prep-1", IntentID: "intent-1", ServerID: "srv-1",
			Repository: proto.GitHubRepositoryWire{ID: "42", Owner: "octo", Name: "repo"},
			Stage:      "failed", FailureCode: "clone_failed", RetryOrigin: "clone", Revision: 3,
		})
	})
	handler := newMux(daemon, "secret", false, defaultMuxDependencies())
	req := httptest.NewRequest(http.MethodGet, "/api/repository-preparations/prep-1", nil)
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set(ProtocolVersionHeader, ProtocolVersion)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var response proto.RespRepositoryPreparation
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Stage != "failed" || response.FailureCode != "clone_failed" || response.Revision != 3 {
		t.Fatalf("response=%#v", response)
	}
}

func TestCreateRepositorySessionConsumesLeaseIntoBoundOneUseAuthority(t *testing.T) {
	daemon, fake := newDaemonPair(t)
	fake.pumpResponses(func(reqID string) { fake.sendResp(reqID, proto.RespOK{}) })
	leases := newLeaseStore()
	issue, failure := leases.Issue("workspace", "42", "octo", "repo", "sentinel-token", "g1")
	if failure != nil {
		t.Fatal(failure)
	}
	authority := &authorityCapture{}
	deps := defaultMuxDependencies()
	deps.leases = leases
	deps.repositoryAuthority = authority
	handler := newMux(daemon, "secret", false, deps)
	profileID, revision := repositoryProfile(t)
	body := fmt.Sprintf(`{"intent_id":"intent-1","preparation_id":"prep-1","server_id":"srv-1","repository_id":"42","owner":"octo","name":"repo","lease_id":%q,"version":2,"profile_id":%q,"profile_revision":%q}`, issue.LeaseID, profileID, revision)
	req := httptest.NewRequest(http.MethodPost, "/api/repository-sessions", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set(ProtocolVersionHeader, ProtocolVersion)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if authority.serverID != "srv-1" || authority.repositoryID != "42" || authority.token != "sentinel-token" {
		t.Fatalf("authority=%#v", authority)
	}
	if _, replay := leases.Consume(issue.LeaseID); replay == nil || replay.Code != "lease_replay" {
		t.Fatalf("lease replay=%v", replay)
	}
}
