// To parse the JSON, install kotlin's serialization plugin and do:
//
// val json     = Json { allowStructuredMapKeys = true }
// val commands = json.parse(Commands.serializer(), jsonString)

package dev.agentgrid.client.models

import kotlinx.serialization.*
import kotlinx.serialization.json.*
import kotlinx.serialization.descriptors.*
import kotlinx.serialization.encoding.*

/**
 * Commands clients send for approval/question resolution.
 */
@Serializable
data class Commands (
    @SerialName("approval_id")
    val approvalID: String? = null,

    @SerialName("client_instance_id")
    val clientInstanceID: String? = null,

    val decision: Decision? = null,

    @SerialName("session_id")
    val sessionID: String,

    /**
     * Free-text answer only; structured objects are rejected at the wire layer.
     */
    val answer: String? = null,

    @SerialName("question_id")
    val questionID: String? = null
)

@Serializable
enum class Decision(val value: String) {
    @SerialName("accept") Accept("accept"),
    @SerialName("deny") Deny("deny");
}
