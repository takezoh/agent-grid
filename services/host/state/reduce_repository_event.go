package state

func init() {
	RegisterEvent[RepositoryCreateParams](EventCreateRepositorySession, reduceRepositoryCreateEvent)
	RegisterEvent[RepositoryPreparationParams](EventGetRepositoryPreparation, reduceRepositoryPreparationEvent)
}

type RepositoryCreateParams struct {
	IntentID      CreationIntentID         `json:"intent_id"`
	PreparationID PreparationID            `json:"preparation_id"`
	ServerID      ExecutionServerID        `json:"server_id"`
	Repository    GitHubRepositoryIdentity `json:"repository"`
	Fingerprint   string                   `json:"fingerprint,omitempty"`
	Command       string                   `json:"command"`
}

type RepositoryPreparationParams struct {
	PreparationID PreparationID `json:"preparation_id"`
}

type RepositoryPreparationReply struct {
	PreparationID          string                   `json:"preparation_id"`
	IntentID               string                   `json:"intent_id"`
	ServerID               string                   `json:"server_id"`
	Repository             GitHubRepositoryIdentity `json:"repository"`
	Stage                  string                   `json:"stage"`
	FailureCode            string                   `json:"failure_code,omitempty"`
	RetryOrigin            string                   `json:"retry_origin,omitempty"`
	EnvironmentDisposition string                   `json:"environment_disposition,omitempty"`
	Revision               uint64                   `json:"revision"`
	SessionID              string                   `json:"session_id,omitempty"`
	ClonePath              string                   `json:"clone_path,omitempty"`
	EnvironmentID          string                   `json:"environment_id,omitempty"`
}

func reduceRepositoryCreateEvent(s State, connID ConnID, reqID string, p RepositoryCreateParams) (State, []Effect) {
	return reduceRepositoryCreate(s, EvRepository{
		ConnID: connID, ReqID: reqID, Kind: "create",
		IntentID: p.IntentID, PreparationID: p.PreparationID, ServerID: p.ServerID,
		Repository: p.Repository, Fingerprint: p.Fingerprint, Command: p.Command,
	})
}

func reduceRepositoryPreparationEvent(s State, connID ConnID, reqID string, p RepositoryPreparationParams) (State, []Effect) {
	prep, ok := FindPreparation(s, p.PreparationID)
	if !ok {
		return s, rejectPrep(connID, reqID, PrepFailureInvalidIdentity, "preparation not found")
	}
	return s, []Effect{okResp(connID, reqID, prepSnapshot(prep))}
}
