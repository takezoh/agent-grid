// This file was generated from JSON Schema using quicktype, do not modify it directly.
// To parse the JSON, add this file to your project and do:
//
//   let events = try Events(json)

import Foundation

/// WS/event frames the daemon pushes to clients. Discriminated by k on the lifecycle surface.
// MARK: - Events
public struct Events: Codable {
    public let approval: Approval?
    public let k: K
    public let question: Question?

    public init(approval: Approval?, k: K, question: Question?) {
        self.approval = approval
        self.k = k
        self.question = question
    }
}

// MARK: Events convenience initializers and mutators

public extension Events {
    init(data: Data) throws {
        self = try newJSONDecoder().decode(Events.self, from: data)
    }

    init(_ json: String, using encoding: String.Encoding = .utf8) throws {
        guard let data = json.data(using: encoding) else {
            throw NSError(domain: "JSONDecoding", code: 0, userInfo: nil)
        }
        try self.init(data: data)
    }

    init(fromURL url: URL) throws {
        try self.init(data: try Data(contentsOf: url))
    }

    func with(
        approval: Approval?? = nil,
        k: K? = nil,
        question: Question?? = nil
    ) -> Events {
        return Events(
            approval: approval ?? self.approval,
            k: k ?? self.k,
            question: question ?? self.question
        )
    }

    func jsonData() throws -> Data {
        return try newJSONEncoder().encode(self)
    }

    func jsonString(encoding: String.Encoding = .utf8) throws -> String? {
        return String(data: try self.jsonData(), encoding: encoding)
    }
}

// MARK: - Approval
public struct Approval: Codable {
    public let command: String?
    public let createdAt: Date?
    public let decision, defaultDecision: Decision?
    public let expiresAt: Date?
    public let frameID: String?
    public let id: String
    public let kind: Kind?
    public let path, reason: String?
    public let resolutionReason: ApprovalResolutionReason?
    public let resolvingClientInstanceID: String?
    public let sessionID: String
    public let status: Status

    public enum CodingKeys: String, CodingKey {
        case command
        case createdAt = "created_at"
        case decision
        case defaultDecision = "default_decision"
        case expiresAt = "expires_at"
        case frameID = "frame_id"
        case id, kind, path, reason
        case resolutionReason = "resolution_reason"
        case resolvingClientInstanceID = "resolving_client_instance_id"
        case sessionID = "session_id"
        case status
    }

    public init(command: String?, createdAt: Date?, decision: Decision?, defaultDecision: Decision?, expiresAt: Date?, frameID: String?, id: String, kind: Kind?, path: String?, reason: String?, resolutionReason: ApprovalResolutionReason?, resolvingClientInstanceID: String?, sessionID: String, status: Status) {
        self.command = command
        self.createdAt = createdAt
        self.decision = decision
        self.defaultDecision = defaultDecision
        self.expiresAt = expiresAt
        self.frameID = frameID
        self.id = id
        self.kind = kind
        self.path = path
        self.reason = reason
        self.resolutionReason = resolutionReason
        self.resolvingClientInstanceID = resolvingClientInstanceID
        self.sessionID = sessionID
        self.status = status
    }
}

// MARK: Approval convenience initializers and mutators

public extension Approval {
    init(data: Data) throws {
        self = try newJSONDecoder().decode(Approval.self, from: data)
    }

    init(_ json: String, using encoding: String.Encoding = .utf8) throws {
        guard let data = json.data(using: encoding) else {
            throw NSError(domain: "JSONDecoding", code: 0, userInfo: nil)
        }
        try self.init(data: data)
    }

    init(fromURL url: URL) throws {
        try self.init(data: try Data(contentsOf: url))
    }

    func with(
        command: String?? = nil,
        createdAt: Date?? = nil,
        decision: Decision?? = nil,
        defaultDecision: Decision?? = nil,
        expiresAt: Date?? = nil,
        frameID: String?? = nil,
        id: String? = nil,
        kind: Kind?? = nil,
        path: String?? = nil,
        reason: String?? = nil,
        resolutionReason: ApprovalResolutionReason?? = nil,
        resolvingClientInstanceID: String?? = nil,
        sessionID: String? = nil,
        status: Status? = nil
    ) -> Approval {
        return Approval(
            command: command ?? self.command,
            createdAt: createdAt ?? self.createdAt,
            decision: decision ?? self.decision,
            defaultDecision: defaultDecision ?? self.defaultDecision,
            expiresAt: expiresAt ?? self.expiresAt,
            frameID: frameID ?? self.frameID,
            id: id ?? self.id,
            kind: kind ?? self.kind,
            path: path ?? self.path,
            reason: reason ?? self.reason,
            resolutionReason: resolutionReason ?? self.resolutionReason,
            resolvingClientInstanceID: resolvingClientInstanceID ?? self.resolvingClientInstanceID,
            sessionID: sessionID ?? self.sessionID,
            status: status ?? self.status
        )
    }

    func jsonData() throws -> Data {
        return try newJSONEncoder().encode(self)
    }

    func jsonString(encoding: String.Encoding = .utf8) throws -> String? {
        return String(data: try self.jsonData(), encoding: encoding)
    }
}

public enum Decision: String, Codable {
    case accept = "accept"
    case deny = "deny"
}

public enum Kind: String, Codable {
    case command = "command"
    case fileChange = "file_change"
}

public enum ApprovalResolutionReason: String, Codable {
    case auto = "auto"
    case cancelled = "cancelled"
    case client = "client"
    case expired = "expired"
}

public enum Status: String, Codable {
    case cancelled = "cancelled"
    case expired = "expired"
    case pending = "pending"
    case resolved = "resolved"
}

public enum K: String, Codable {
    case ar = "ar"
    case ax = "ax"
    case qr = "qr"
    case qx = "qx"
}

// MARK: - Question
public struct Question: Codable {
    /// Free-text only (HumanInputRequest.free_text).
    public let answer: String?
    public let createdAt, expiresAt: Date?
    public let frameID: String?
    public let id: String
    public let prompt: String?
    public let resolutionReason: QuestionResolutionReason?
    public let resolvingClientInstanceID: String?
    public let sessionID: String
    public let status: Status

    public enum CodingKeys: String, CodingKey {
        case answer
        case createdAt = "created_at"
        case expiresAt = "expires_at"
        case frameID = "frame_id"
        case id, prompt
        case resolutionReason = "resolution_reason"
        case resolvingClientInstanceID = "resolving_client_instance_id"
        case sessionID = "session_id"
        case status
    }

    public init(answer: String?, createdAt: Date?, expiresAt: Date?, frameID: String?, id: String, prompt: String?, resolutionReason: QuestionResolutionReason?, resolvingClientInstanceID: String?, sessionID: String, status: Status) {
        self.answer = answer
        self.createdAt = createdAt
        self.expiresAt = expiresAt
        self.frameID = frameID
        self.id = id
        self.prompt = prompt
        self.resolutionReason = resolutionReason
        self.resolvingClientInstanceID = resolvingClientInstanceID
        self.sessionID = sessionID
        self.status = status
    }
}

// MARK: Question convenience initializers and mutators

public extension Question {
    init(data: Data) throws {
        self = try newJSONDecoder().decode(Question.self, from: data)
    }

    init(_ json: String, using encoding: String.Encoding = .utf8) throws {
        guard let data = json.data(using: encoding) else {
            throw NSError(domain: "JSONDecoding", code: 0, userInfo: nil)
        }
        try self.init(data: data)
    }

    init(fromURL url: URL) throws {
        try self.init(data: try Data(contentsOf: url))
    }

    func with(
        answer: String?? = nil,
        createdAt: Date?? = nil,
        expiresAt: Date?? = nil,
        frameID: String?? = nil,
        id: String? = nil,
        prompt: String?? = nil,
        resolutionReason: QuestionResolutionReason?? = nil,
        resolvingClientInstanceID: String?? = nil,
        sessionID: String? = nil,
        status: Status? = nil
    ) -> Question {
        return Question(
            answer: answer ?? self.answer,
            createdAt: createdAt ?? self.createdAt,
            expiresAt: expiresAt ?? self.expiresAt,
            frameID: frameID ?? self.frameID,
            id: id ?? self.id,
            prompt: prompt ?? self.prompt,
            resolutionReason: resolutionReason ?? self.resolutionReason,
            resolvingClientInstanceID: resolvingClientInstanceID ?? self.resolvingClientInstanceID,
            sessionID: sessionID ?? self.sessionID,
            status: status ?? self.status
        )
    }

    func jsonData() throws -> Data {
        return try newJSONEncoder().encode(self)
    }

    func jsonString(encoding: String.Encoding = .utf8) throws -> String? {
        return String(data: try self.jsonData(), encoding: encoding)
    }
}

public enum QuestionResolutionReason: String, Codable {
    case cancelled = "cancelled"
    case client = "client"
    case expired = "expired"
}

// MARK: - Helper functions for creating encoders and decoders

func newJSONDecoder() -> JSONDecoder {
    let decoder = JSONDecoder()
    if #available(iOS 10.0, OSX 10.12, tvOS 10.0, watchOS 3.0, *) {
        decoder.dateDecodingStrategy = .iso8601
    }
    return decoder
}

func newJSONEncoder() -> JSONEncoder {
    let encoder = JSONEncoder()
    if #available(iOS 10.0, OSX 10.12, tvOS 10.0, watchOS 3.0, *) {
        encoder.dateEncodingStrategy = .iso8601
    }
    return encoder
}
