// To parse the JSON, install kotlin's serialization plugin and do:
//
// val json          = Json { allowStructuredMapKeys = true }
// val sessionStatus = json.parse(SessionStatus.serializer(), jsonString)

package dev.agentgrid.client.models

import kotlinx.serialization.*
import kotlinx.serialization.json.*
import kotlinx.serialization.descriptors.*
import kotlinx.serialization.encoding.*

@Serializable
data class SessionStatus (
    @SerialName("changed_at")
    val changedAt: String? = null,

    val failure: Failure,

    @SerialName("human_input")
    val humanInput: Boolean,

    val lifecycle: Lifecycle,
    val signal: Signal? = null
)

@Serializable
enum class Failure(val value: String) {
    @SerialName("failed") Failed("failed"),
    @SerialName("none") None("none");
}

@Serializable
enum class Lifecycle(val value: String) {
    @SerialName("active") Active("active"),
    @SerialName("idle") Idle("idle"),
    @SerialName("stopped") Stopped("stopped");
}

@Serializable
data class Signal (
    val epoch: String,
    val kind: Kind,
    val sequence: Long
)

@Serializable
enum class Kind(val value: String) {
    @SerialName("bell") Bell("bell");
}
