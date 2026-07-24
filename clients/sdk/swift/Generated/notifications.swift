// This file was generated from JSON Schema using quicktype, do not modify it directly.
// To parse the JSON, add this file to your project and do:
//
//   let notifications = try Notifications(json)

import Foundation

/// Notification payload skeleton for Phase 0/1 (policy details in
/// contracts/notification-policy.md).
// MARK: - Notifications
public struct Notifications: Codable {
    public let body: String?
    public let deepLink: String?
    public let kind: Kind
    public let sessionID: String
    public let title: String?

    public enum CodingKeys: String, CodingKey {
        case body
        case deepLink = "deep_link"
        case kind
        case sessionID = "session_id"
        case title
    }

    public init(body: String?, deepLink: String?, kind: Kind, sessionID: String, title: String?) {
        self.body = body
        self.deepLink = deepLink
        self.kind = kind
        self.sessionID = sessionID
        self.title = title
    }
}

// MARK: Notifications convenience initializers and mutators

public extension Notifications {
    init(data: Data) throws {
        self = try newJSONDecoder().decode(Notifications.self, from: data)
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
        body: String?? = nil,
        deepLink: String?? = nil,
        kind: Kind? = nil,
        sessionID: String? = nil,
        title: String?? = nil
    ) -> Notifications {
        return Notifications(
            body: body ?? self.body,
            deepLink: deepLink ?? self.deepLink,
            kind: kind ?? self.kind,
            sessionID: sessionID ?? self.sessionID,
            title: title ?? self.title
        )
    }

    func jsonData() throws -> Data {
        return try newJSONEncoder().encode(self)
    }

    func jsonString(encoding: String.Encoding = .utf8) throws -> String? {
        return String(data: try self.jsonData(), encoding: encoding)
    }
}

public enum Kind: String, Codable {
    case agentNotification = "agent_notification"
    case approvalPending = "approval_pending"
    case questionPending = "question_pending"
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
