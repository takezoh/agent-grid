// This file was generated from JSON Schema using quicktype, do not modify it directly.
// To parse the JSON, add this file to your project and do:
//
//   let deepLinks = try DeepLinks(json)

import Foundation

/// agent-grid:// URI shapes adopted from plans/remote-control-mobile-session-deep-link.md
/// (FR-P1-09).
// MARK: - DeepLinks
public struct DeepLinks: Codable {
    /// Session id or ApprovalRequest id.
    public let id: String
    /// Path kind: agent-grid://session/<id> or agent-grid://approval/<id>.
    public let kind: Kind
    public let scheme: Scheme
    /// Full URI form for round-trip helpers.
    public let uri: String?

    public init(id: String, kind: Kind, scheme: Scheme, uri: String?) {
        self.id = id
        self.kind = kind
        self.scheme = scheme
        self.uri = uri
    }
}

// MARK: DeepLinks convenience initializers and mutators

public extension DeepLinks {
    init(data: Data) throws {
        self = try newJSONDecoder().decode(DeepLinks.self, from: data)
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
        id: String? = nil,
        kind: Kind? = nil,
        scheme: Scheme? = nil,
        uri: String?? = nil
    ) -> DeepLinks {
        return DeepLinks(
            id: id ?? self.id,
            kind: kind ?? self.kind,
            scheme: scheme ?? self.scheme,
            uri: uri ?? self.uri
        )
    }

    func jsonData() throws -> Data {
        return try newJSONEncoder().encode(self)
    }

    func jsonString(encoding: String.Encoding = .utf8) throws -> String? {
        return String(data: try self.jsonData(), encoding: encoding)
    }
}

/// Path kind: agent-grid://session/<id> or agent-grid://approval/<id>.
public enum Kind: String, Codable {
    case approval = "approval"
    case session = "session"
}

public enum Scheme: String, Codable {
    case agentGrid = "agent-grid"
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
