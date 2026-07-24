// This file was generated from JSON Schema using quicktype, do not modify it directly.
// To parse the JSON, add this file to your project and do:
//
//   let commands = try Commands(json)

import Foundation

/// Commands clients send for approval/question resolution.
// MARK: - Commands
public struct Commands: Codable {
    public let approvalID, clientInstanceID: String?
    public let decision: Decision?
    public let sessionID: String
    /// Free-text answer only; structured objects are rejected at the wire layer.
    public let answer: String?
    public let questionID: String?

    public enum CodingKeys: String, CodingKey {
        case approvalID = "approval_id"
        case clientInstanceID = "client_instance_id"
        case decision
        case sessionID = "session_id"
        case answer
        case questionID = "question_id"
    }

    public init(approvalID: String?, clientInstanceID: String?, decision: Decision?, sessionID: String, answer: String?, questionID: String?) {
        self.approvalID = approvalID
        self.clientInstanceID = clientInstanceID
        self.decision = decision
        self.sessionID = sessionID
        self.answer = answer
        self.questionID = questionID
    }
}

// MARK: Commands convenience initializers and mutators

public extension Commands {
    init(data: Data) throws {
        self = try newJSONDecoder().decode(Commands.self, from: data)
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
        approvalID: String?? = nil,
        clientInstanceID: String?? = nil,
        decision: Decision?? = nil,
        sessionID: String? = nil,
        answer: String?? = nil,
        questionID: String?? = nil
    ) -> Commands {
        return Commands(
            approvalID: approvalID ?? self.approvalID,
            clientInstanceID: clientInstanceID ?? self.clientInstanceID,
            decision: decision ?? self.decision,
            sessionID: sessionID ?? self.sessionID,
            answer: answer ?? self.answer,
            questionID: questionID ?? self.questionID
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
