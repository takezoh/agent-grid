// From protocol/repository.schema.json — exact-version repository/preparation surface.
// SessionTarget oneOf is enforced in decode; quicktype flattens discriminators for Swift.
import Foundation

public struct GitHubRepositoryIdentity: Codable, Equatable {
    public let repositoryId: String
    public let owner: String
    public let name: String
    enum CodingKeys: String, CodingKey {
        case repositoryId = "repository_id"
        case owner, name
    }
}

public struct ProjectTarget: Codable, Equatable {
    public let serverId: String
    public let project: String
    enum CodingKeys: String, CodingKey {
        case serverId = "server_id"
        case project
    }
}

public struct SessionTarget: Codable, Equatable {
    public let source: String
    public let project: ProjectTarget?
    public let repository: GitHubRepositoryIdentity?

    enum CodingKeys: String, CodingKey {
        case source, project, repository
    }

    public init(source: String, project: ProjectTarget? = nil, repository: GitHubRepositoryIdentity? = nil) throws {
        self.source = source
        self.project = project
        self.repository = repository
        guard Self.isValid(source: source, project: project, repository: repository) else {
            throw DecodingError.dataCorrupted(.init(
                codingPath: [],
                debugDescription: "SessionTarget must be a oneOf project|repository discriminator payload"
            ))
        }
    }

    public init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        let source = try c.decode(String.self, forKey: .source)
        let project = try c.decodeIfPresent(ProjectTarget.self, forKey: .project)
        let repository = try c.decodeIfPresent(GitHubRepositoryIdentity.self, forKey: .repository)
        try self.init(source: source, project: project, repository: repository)
    }

    public func encode(to encoder: Encoder) throws {
        guard Self.isValid(source: source, project: project, repository: repository) else {
            throw EncodingError.invalidValue(self, .init(
                codingPath: [],
                debugDescription: "SessionTarget must be a oneOf project|repository discriminator payload"
            ))
        }
        var c = encoder.container(keyedBy: CodingKeys.self)
        try c.encode(source, forKey: .source)
        if source == "project" {
            try c.encodeIfPresent(project, forKey: .project)
        }
        if source == "repository" {
            try c.encodeIfPresent(repository, forKey: .repository)
        }
    }

    public static func isValid(source: String, project: ProjectTarget?, repository: GitHubRepositoryIdentity?) -> Bool {
        switch source {
        case "project":
            return project != nil
                && !(project!.serverId.isEmpty)
                && !(project!.project.isEmpty)
                && repository == nil
        case "repository":
            return repository != nil
                && !(repository!.repositoryId.isEmpty)
                && !(repository!.owner.isEmpty)
                && !(repository!.name.isEmpty)
                && project == nil
        default:
            return false
        }
    }
}

public struct RepositoryPreparation: Codable, Equatable {
    public let preparationId: String
    public let intentId: String
    public let serverId: String
    public let repository: GitHubRepositoryIdentity
    public let stage: String
    public let revision: Int
    public let failureCode: String?
    public let retryOrigin: String?
    public let environmentDisposition: String?
    public let sessionId: String?
    public let clonePath: String?
    public let environmentId: String?
    enum CodingKeys: String, CodingKey {
        case preparationId = "preparation_id"
        case intentId = "intent_id"
        case serverId = "server_id"
        case repository, stage, revision
        case failureCode = "failure_code"
        case retryOrigin = "retry_origin"
        case environmentDisposition = "environment_disposition"
        case sessionId = "session_id"
        case clonePath = "clone_path"
        case environmentId = "environment_id"
    }
}
