package codexschema

// SupportedVersion is the app-server protocol version used to generate the
// checked-in schema and types.
const SupportedVersion = "0.145.0"

// ServerNotifications is the exhaustive method inventory generated from the
// Codex 0.145.0 ServerNotification union. Router tests compare this list with
// the checked-in schema so a protocol bump cannot silently add a method.
var ServerNotifications = []string{
	"account/login/completed",
	"account/rateLimits/updated",
	"account/updated",
	"app/list/updated",
	"command/exec/outputDelta",
	"configWarning",
	"deprecationNotice",
	"error",
	"externalAgentConfig/import/completed",
	"externalAgentConfig/import/progress",
	"fs/changed",
	"fuzzyFileSearch/sessionCompleted",
	"fuzzyFileSearch/sessionUpdated",
	"guardianWarning",
	"hook/completed",
	"hook/started",
	"item/agentMessage/delta",
	"item/autoApprovalReview/completed",
	"item/autoApprovalReview/started",
	"item/commandExecution/outputDelta",
	"item/commandExecution/terminalInteraction",
	"item/completed",
	"item/fileChange/outputDelta",
	"item/fileChange/patchUpdated",
	"item/mcpToolCall/progress",
	"item/plan/delta",
	"item/reasoning/summaryPartAdded",
	"item/reasoning/summaryTextDelta",
	"item/reasoning/textDelta",
	"item/started",
	"mcpServer/oauthLogin/completed",
	"mcpServer/startupStatus/updated",
	"model/rerouted",
	"model/safetyBuffering/updated",
	"model/verification",
	"process/exited",
	"process/outputDelta",
	"remoteControl/status/changed",
	"serverRequest/resolved",
	"skills/changed",
	"thread/archived",
	"thread/closed",
	"thread/compacted",
	"thread/deleted",
	"thread/environment/connected",
	"thread/environment/disconnected",
	"thread/goal/cleared",
	"thread/goal/updated",
	"thread/name/updated",
	"thread/realtime/closed",
	"thread/realtime/error",
	"thread/realtime/itemAdded",
	"thread/realtime/outputAudio/delta",
	"thread/realtime/sdp",
	"thread/realtime/started",
	"thread/realtime/transcript/delta",
	"thread/realtime/transcript/done",
	"thread/settings/updated",
	"thread/started",
	"thread/status/changed",
	"thread/tokenUsage/updated",
	"thread/unarchived",
	"turn/completed",
	"turn/diff/updated",
	"turn/moderationMetadata",
	"turn/plan/updated",
	"turn/started",
	"warning",
	"windows/worldWritableWarning",
	"windowsSandbox/setupCompleted",
}

// ServerRequests is the exhaustive Codex 0.145.0 ServerRequest union.
var ServerRequests = []string{
	"account/chatgptAuthTokens/refresh",
	"applyPatchApproval",
	"attestation/generate",
	"execCommandApproval",
	"item/commandExecution/requestApproval",
	"item/fileChange/requestApproval",
	"item/permissions/requestApproval",
	"item/tool/call",
	"item/tool/requestUserInput",
	"mcpServer/elicitation/request",
}

var notificationSet = stringSet(ServerNotifications)
var requestSet = stringSet(ServerRequests)

func IsKnownServerNotification(method string) bool {
	_, ok := notificationSet[method]
	return ok
}

func IsKnownServerRequest(method string) bool {
	_, ok := requestSet[method]
	return ok
}

func stringSet(methods []string) map[string]struct{} {
	out := make(map[string]struct{}, len(methods))
	for _, method := range methods {
		out[method] = struct{}{}
	}
	return out
}
