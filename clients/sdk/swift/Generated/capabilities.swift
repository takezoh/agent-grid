// This file was generated from JSON Schema using quicktype, do not modify it directly.
// To parse the JSON, add this file to your project and do:
//
//   let capabilities = try Capabilities(json)

import Foundation

/// Capability negotiation declaration for bundled vs remote clients (FR-P1-03, FR-P1-04).
// MARK: - Capabilities
public struct Capabilities: Codable {
    /// Two-axis compatibility policy skeleton.
    public let axis: Axis?
    /// Feature flags the peer supports (e.g. approval.respond).
    public let capabilities: [String]
    /// Daemon/client contract version. Bundled clients match this string and skip per-capability
    /// negotiation.
    public let protocolVersion: String

    public init(axis: Axis?, capabilities: [String], protocolVersion: String) {
        self.axis = axis
        self.capabilities = capabilities
        self.protocolVersion = protocolVersion
    }
}

// MARK: Capabilities convenience initializers and mutators

public extension Capabilities {
    init(data: Data) throws {
        self = try newJSONDecoder().decode(Capabilities.self, from: data)
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
        axis: Axis?? = nil,
        capabilities: [String]? = nil,
        protocolVersion: String? = nil
    ) -> Capabilities {
        return Capabilities(
            axis: axis ?? self.axis,
            capabilities: capabilities ?? self.capabilities,
            protocolVersion: protocolVersion ?? self.protocolVersion
        )
    }

    func jsonData() throws -> Data {
        return try newJSONEncoder().encode(self)
    }

    func jsonString(encoding: String.Encoding = .utf8) throws -> String? {
        return String(data: try self.jsonData(), encoding: encoding)
    }
}

/// Two-axis compatibility policy skeleton.
public enum Axis: String, Codable {
    case bundled = "bundled"
    case remote = "remote"
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
