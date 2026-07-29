// From protocol/repository.schema.json — exact-version repository/preparation surface.
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
