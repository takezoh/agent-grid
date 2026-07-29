// From protocol/repository.schema.json — exact-version repository/preparation surface.
package dev.agentgrid.client.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

@Serializable
data class GitHubRepositoryIdentity(
    @SerialName("repository_id") val repositoryId: String,
    val owner: String,
    val name: String,
)

@Serializable
data class ProjectTarget(
    @SerialName("server_id") val serverId: String,
    val project: String,
)

@Serializable
data class SessionTarget(
    val source: String,
    val project: ProjectTarget? = null,
    val repository: GitHubRepositoryIdentity? = null,
)

@Serializable
data class RepositoryPreparation(
    @SerialName("preparation_id") val preparationId: String,
    @SerialName("intent_id") val intentId: String,
    @SerialName("server_id") val serverId: String,
    val repository: GitHubRepositoryIdentity,
    val stage: String,
    val revision: Long,
    @SerialName("failure_code") val failureCode: String? = null,
    @SerialName("retry_origin") val retryOrigin: String? = null,
    @SerialName("environment_disposition") val environmentDisposition: String? = null,
    @SerialName("session_id") val sessionId: String? = null,
    @SerialName("clone_path") val clonePath: String? = null,
    @SerialName("environment_id") val environmentId: String? = null,
)
