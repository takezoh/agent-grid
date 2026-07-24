// To parse the JSON, install kotlin's serialization plugin and do:
//
// val json         = Json { allowStructuredMapKeys = true }
// val capabilities = json.parse(Capabilities.serializer(), jsonString)

package dev.agentgrid.client.models

import kotlinx.serialization.*
import kotlinx.serialization.json.*
import kotlinx.serialization.descriptors.*
import kotlinx.serialization.encoding.*

/**
 * Capability negotiation declaration for bundled vs remote clients (FR-P1-03, FR-P1-04).
 */
@Serializable
data class Capabilities (
    /**
     * Two-axis compatibility policy skeleton.
     */
    val axis: Axis? = null,

    /**
     * Feature flags the peer supports (e.g. approval.respond).
     */
    val capabilities: List<String>,

    /**
     * Daemon/client contract version. Bundled clients match this string and skip per-capability
     * negotiation.
     */
    val protocolVersion: String
)

/**
 * Two-axis compatibility policy skeleton.
 */
@Serializable
enum class Axis(val value: String) {
    @SerialName("bundled") Bundled("bundled"),
    @SerialName("remote") Remote("remote");
}
