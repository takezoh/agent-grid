// To parse the JSON, install kotlin's serialization plugin and do:
//
// val json   = Json { allowStructuredMapKeys = true }
// val events = json.parse(Events.serializer(), jsonString)

package dev.agentgrid.client.models

import kotlinx.serialization.*
import kotlinx.serialization.json.*
import kotlinx.serialization.descriptors.*
import kotlinx.serialization.encoding.*

/**
 * WS/event frames the daemon pushes to clients. Discriminated by k on the lifecycle surface.
 */
@Serializable
data class Events (
    val approval: Approval? = null,
    val k: K,
    val question: Question? = null
)

@Serializable
data class Approval (
    val command: String? = null,

    @SerialName("created_at")
    val createdAt: String? = null,

    val decision: Decision? = null,

    @SerialName("default_decision")
    val defaultDecision: Decision? = null,

    @SerialName("expires_at")
    val expiresAt: String? = null,

    @SerialName("frame_id")
    val frameID: String? = null,

    val id: String,
    val kind: Kind? = null,
    val path: String? = null,
    val reason: String? = null,

    @SerialName("resolution_reason")
    val resolutionReason: ApprovalResolutionReason? = null,

    @SerialName("resolving_client_instance_id")
    val resolvingClientInstanceID: String? = null,

    @SerialName("session_id")
    val sessionID: String,

    val status: Status
)

@Serializable
enum class Decision(val value: String) {
    @SerialName("accept") Accept("accept"),
    @SerialName("deny") Deny("deny");
}

@Serializable
enum class Kind(val value: String) {
    @SerialName("command") Command("command"),
    @SerialName("file_change") FileChange("file_change");
}

@Serializable
enum class ApprovalResolutionReason(val value: String) {
    @SerialName("auto") Auto("auto"),
    @SerialName("cancelled") Cancelled("cancelled"),
    @SerialName("client") Client("client"),
    @SerialName("expired") Expired("expired");
}

@Serializable
enum class Status(val value: String) {
    @SerialName("cancelled") Cancelled("cancelled"),
    @SerialName("expired") Expired("expired"),
    @SerialName("pending") Pending("pending"),
    @SerialName("resolved") Resolved("resolved");
}

@Serializable
enum class K(val value: String) {
    @SerialName("ar") Ar("ar"),
    @SerialName("ax") Ax("ax"),
    @SerialName("qr") Qr("qr"),
    @SerialName("qx") Qx("qx");
}

@Serializable
data class Question (
    /**
     * Free-text only (HumanInputRequest.free_text).
     */
    val answer: String? = null,

    @SerialName("created_at")
    val createdAt: String? = null,

    @SerialName("expires_at")
    val expiresAt: String? = null,

    @SerialName("frame_id")
    val frameID: String? = null,

    val id: String,
    val prompt: String? = null,

    @SerialName("resolution_reason")
    val resolutionReason: QuestionResolutionReason? = null,

    @SerialName("resolving_client_instance_id")
    val resolvingClientInstanceID: String? = null,

    @SerialName("session_id")
    val sessionID: String,

    val status: Status
)

@Serializable
enum class QuestionResolutionReason(val value: String) {
    @SerialName("cancelled") Cancelled("cancelled"),
    @SerialName("client") Client("client"),
    @SerialName("expired") Expired("expired");
}
