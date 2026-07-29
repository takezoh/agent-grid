// Executable SessionTarget discriminator conformance for the generated Kotlin SDK.
// Invoked via clients/sdk/scripts/run-session-target-conformance.sh (and CI).
package dev.agentgrid.client.harness

import dev.agentgrid.client.models.GitHubRepositoryIdentity
import dev.agentgrid.client.models.ProjectTarget
import dev.agentgrid.client.models.SessionTarget

private fun expectValid(label: String, block: () -> SessionTarget) {
    try {
        val target = block()
        check(target.isValid()) { "$label: constructed target failed isValid()" }
        println("PASS $label")
    } catch (e: Exception) {
        throw IllegalStateException("$label: expected success, got ${e::class.simpleName}: ${e.message}", e)
    }
}

private fun expectInvalid(label: String, block: () -> SessionTarget) {
    try {
        block()
        throw IllegalStateException("$label: expected rejection, constructor accepted payload")
    } catch (e: IllegalArgumentException) {
        println("PASS $label")
    } catch (e: IllegalStateException) {
        throw e
    } catch (e: Exception) {
        // parseOrThrow wraps into SerializationException; accept any construction failure.
        println("PASS $label")
    }
}

fun main() {
    val project = ProjectTarget(serverId = "srv-a", project = "/repo/demo")
    val repository = GitHubRepositoryIdentity(repositoryId = "42", owner = "acme", name = "widgets")

    // valid project
    expectValid("valid-project") {
        SessionTarget(source = "project", project = project)
    }

    // valid repository
    expectValid("valid-repository") {
        SessionTarget(source = "repository", repository = repository)
    }

    // collapsed project+repository
    expectInvalid("collapsed-project-repository") {
        SessionTarget(source = "project", project = project, repository = repository)
    }

    // mixed: source repository with project body
    expectInvalid("mixed-repository-source-project-body") {
        SessionTarget(source = "repository", project = project)
    }

    // mixed: source project with repository body only
    expectInvalid("mixed-project-source-repository-body") {
        SessionTarget(source = "project", repository = repository)
    }

    // missing / unknown source
    expectInvalid("missing-source") {
        SessionTarget(source = "")
    }
    expectInvalid("unknown-source") {
        SessionTarget(source = "other", project = project)
    }

    // empty required identity fields (project)
    expectInvalid("empty-project-identity") {
        SessionTarget(
            source = "project",
            project = ProjectTarget(serverId = "", project = "/repo/demo"),
        )
    }
    expectInvalid("empty-project-path") {
        SessionTarget(
            source = "project",
            project = ProjectTarget(serverId = "srv-a", project = ""),
        )
    }

    // empty required identity fields (repository)
    expectInvalid("empty-repository-id") {
        SessionTarget(
            source = "repository",
            repository = GitHubRepositoryIdentity(repositoryId = "", owner = "acme", name = "widgets"),
        )
    }
    expectInvalid("empty-repository-owner") {
        SessionTarget(
            source = "repository",
            repository = GitHubRepositoryIdentity(repositoryId = "42", owner = "", name = "widgets"),
        )
    }
    expectInvalid("empty-repository-name") {
        SessionTarget(
            source = "repository",
            repository = GitHubRepositoryIdentity(repositoryId = "42", owner = "acme", name = ""),
        )
    }

    // companion parseOrThrow rejects collapsed payload
    expectInvalid("parseOrThrow-collapsed") {
        SessionTarget.parseOrThrow("project", project, repository)
    }

    println("SessionTarget Kotlin conformance: all cases passed")
}
