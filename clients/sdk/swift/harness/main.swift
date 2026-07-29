// Executable SessionTarget discriminator conformance for the generated Swift SDK.
// Invoked via clients/sdk/scripts/run-session-target-conformance.sh (and CI).
import Foundation

enum HarnessError: Error, CustomStringConvertible {
    case unexpectedSuccess(String)
    case unexpectedFailure(String, String)

    var description: String {
        switch self {
        case .unexpectedSuccess(let label):
            return "\(label): expected rejection"
        case .unexpectedFailure(let label, let detail):
            return "\(label): expected success, got \(detail)"
        }
    }
}

func expectValid(_ label: String, json: String) throws {
    do {
        let data = Data(json.utf8)
        let target = try JSONDecoder().decode(SessionTarget.self, from: data)
        guard SessionTarget.isValid(
            source: target.source,
            project: target.project,
            repository: target.repository
        ) else {
            throw HarnessError.unexpectedFailure(label, "isValid=false")
        }
        print("PASS \(label)")
    } catch let err as HarnessError {
        throw err
    } catch {
        throw HarnessError.unexpectedFailure(label, String(describing: error))
    }
}

func expectInvalid(_ label: String, json: String) throws {
    do {
        let data = Data(json.utf8)
        _ = try JSONDecoder().decode(SessionTarget.self, from: data)
        throw HarnessError.unexpectedSuccess(label)
    } catch is HarnessError {
        throw HarnessError.unexpectedSuccess(label)
    } catch {
        print("PASS \(label)")
    }
}

func expectInvalidInit(_ label: String, build: () throws -> SessionTarget) throws {
    do {
        _ = try build()
        throw HarnessError.unexpectedSuccess(label)
    } catch is HarnessError {
        throw HarnessError.unexpectedSuccess(label)
    } catch {
        print("PASS \(label)")
    }
}

do {
    try expectValid(
        "valid-project",
        json: #"{"source":"project","project":{"server_id":"srv-a","project":"/repo/demo"}}"#
    )
    try expectValid(
        "valid-repository",
        json: #"{"source":"repository","repository":{"repository_id":"42","owner":"acme","name":"widgets"}}"#
    )
    try expectInvalid(
        "collapsed-project-repository",
        json: #"{"source":"project","project":{"server_id":"srv-a","project":"/repo/demo"},"repository":{"repository_id":"42","owner":"acme","name":"widgets"}}"#
    )
    try expectInvalid(
        "mixed-repository-source-project-body",
        json: #"{"source":"repository","project":{"server_id":"srv-a","project":"/repo/demo"}}"#
    )
    try expectInvalid(
        "mixed-project-source-repository-body",
        json: #"{"source":"project","repository":{"repository_id":"42","owner":"acme","name":"widgets"}}"#
    )
    try expectInvalid("missing-source", json: "{}")
    try expectInvalid(
        "unknown-source",
        json: #"{"source":"other","project":{"server_id":"srv-a","project":"/repo/demo"}}"#
    )
    try expectInvalid(
        "empty-project-identity",
        json: #"{"source":"project","project":{"server_id":"","project":"/repo/demo"}}"#
    )
    try expectInvalid(
        "empty-project-path",
        json: #"{"source":"project","project":{"server_id":"srv-a","project":""}}"#
    )
    try expectInvalid(
        "empty-repository-id",
        json: #"{"source":"repository","repository":{"repository_id":"","owner":"acme","name":"widgets"}}"#
    )
    try expectInvalid(
        "empty-repository-owner",
        json: #"{"source":"repository","repository":{"repository_id":"42","owner":"","name":"widgets"}}"#
    )
    try expectInvalid(
        "empty-repository-name",
        json: #"{"source":"repository","repository":{"repository_id":"42","owner":"acme","name":""}}"#
    )

    // Direct init path (same isValid gate as decode)
    try expectInvalidInit("init-collapsed") {
        try SessionTarget(
            source: "project",
            project: ProjectTarget(serverId: "srv-a", project: "/repo/demo"),
            repository: GitHubRepositoryIdentity(repositoryId: "42", owner: "acme", name: "widgets")
        )
    }

    print("SessionTarget Swift conformance: all cases passed")
    // Stable success sentinel consumed by run-session-target-conformance.sh.
    print("AG_SDK_SESSION_TARGET_CONFORMANCE_OK:swift")
} catch {
    fputs("FAIL \(error)\n", stderr)
    exit(1)
}
