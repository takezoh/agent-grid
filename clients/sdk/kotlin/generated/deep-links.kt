// To parse the JSON, install kotlin's serialization plugin and do:
//
// val json      = Json { allowStructuredMapKeys = true }
// val deepLinks = json.parse(DeepLinks.serializer(), jsonString)

package dev.agentgrid.client.models

import kotlinx.serialization.*
import kotlinx.serialization.json.*
import kotlinx.serialization.descriptors.*
import kotlinx.serialization.encoding.*

/**
 * agent-grid:// URI shapes adopted from plans/remote-control-mobile-session-deep-link.md
 * (FR-P1-09).
 */
@Serializable
data class DeepLinks (
    /**
     * Session id or ApprovalRequest id.
     */
    val id: String,

    /**
     * Path kind: agent-grid://session/<id> or agent-grid://approval/<id>.
     */
    val kind: Kind,

    val scheme: Scheme,

    /**
     * Full URI form for round-trip helpers.
     */
    val uri: String? = null
)

/**
 * Path kind: agent-grid://session/<id> or agent-grid://approval/<id>.
 */
@Serializable
enum class Kind(val value: String) {
    @SerialName("approval") Approval("approval"),
    @SerialName("session") Session("session");
}

@Serializable
enum class Scheme(val value: String) {
    @SerialName("agent-grid") AgentGrid("agent-grid");
}
