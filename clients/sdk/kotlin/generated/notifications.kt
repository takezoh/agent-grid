// To parse the JSON, install kotlin's serialization plugin and do:
//
// val json          = Json { allowStructuredMapKeys = true }
// val notifications = json.parse(Notifications.serializer(), jsonString)

package dev.agentgrid.client.models

import kotlinx.serialization.*
import kotlinx.serialization.json.*
import kotlinx.serialization.descriptors.*
import kotlinx.serialization.encoding.*

/**
 * Notification payload skeleton for Phase 0/1 (policy details in
 * contracts/notification-policy.md).
 */
@Serializable
data class Notifications (
    val body: String? = null,

    @SerialName("deep_link")
    val deepLink: String? = null,

    val kind: Kind,

    @SerialName("session_id")
    val sessionID: String,

    val title: String? = null
)

@Serializable
enum class Kind(val value: String) {
    @SerialName("agent_notification") AgentNotification("agent_notification"),
    @SerialName("approval_pending") ApprovalPending("approval_pending"),
    @SerialName("question_pending") QuestionPending("question_pending");
}
