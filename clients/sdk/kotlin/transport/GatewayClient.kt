// Hand-written thin REST transport for the Kotlin SDK.
// Routes: protocol/openapi.yaml (REST-binding annex).
// Models: ../generated (quicktype).

package dev.agentgrid.client.transport

import java.net.URI
import java.net.http.HttpClient
import java.net.http.HttpRequest
import java.net.http.HttpResponse
import java.time.Duration

class GatewayClient(
    private val baseUrl: String,
    private val token: String? = null,
    private val http: HttpClient = HttpClient.newBuilder()
        .connectTimeout(Duration.ofSeconds(10))
        .build(),
) {
    var clientInstanceId: String? = null
        private set

    fun mintWsTicket(): Pair<String, String> {
        val req = requestBuilder("/api/ws-ticket").POST(HttpRequest.BodyPublishers.noBody()).build()
        val body = http.send(req, HttpResponse.BodyHandlers.ofString()).body()
        // Minimal JSON field scrape to avoid a hard dependency in the skeleton.
        val ticket = field(body, "ticket")
        val ci = field(body, "client_instance_id")
        clientInstanceId = ci
        return ticket to ci
    }

    fun respondApproval(sessionId: String, approvalId: String, decision: String) {
        require(decision == "accept" || decision == "deny")
        val payload =
            """{"decision":"$decision","client_instance_id":${jsonStringOrNull(clientInstanceId)}}"""
        val path =
            "/api/sessions/${enc(sessionId)}/approvals/${enc(approvalId)}"
        val req = requestBuilder(path)
            .header("Content-Type", "application/json")
            .POST(HttpRequest.BodyPublishers.ofString(payload))
            .build()
        val res = http.send(req, HttpResponse.BodyHandlers.ofString())
        require(res.statusCode() in 200..299) { "respondApproval: ${res.statusCode()} ${res.body()}" }
    }

    private fun requestBuilder(path: String): HttpRequest.Builder {
        val b = HttpRequest.newBuilder(URI.create(baseUrl.trimEnd('/') + path))
            .timeout(Duration.ofSeconds(10))
            .header("Accept", "application/json")
        if (token != null) b.header("Authorization", "Bearer $token")
        clientInstanceId?.let { b.header("X-Client-Instance-ID", it) }
        return b
    }

    private fun enc(s: String) = java.net.URLEncoder.encode(s, Charsets.UTF_8)
    private fun jsonStringOrNull(s: String?) = if (s == null) "null" else "\"$s\""
    private fun field(json: String, name: String): String {
        val re = Regex("\"$name\"\\s*:\\s*\"([^\"]*)\"")
        return re.find(json)?.groupValues?.get(1)
            ?: error("missing field $name in $json")
    }
}
