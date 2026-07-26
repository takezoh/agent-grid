// This file was generated from JSON Schema using quicktype, do not modify it directly.
// To parse the JSON, add this file to your project and do:
//
//   let sessionStatus = try SessionStatus(json)

import Foundation

// MARK: - SessionStatus
public struct SessionStatus: Codable {
    public let changedAt: Date?
    public let failure: Failure
    public let humanInput: Bool
    public let lifecycle: Lifecycle
    public let signal: Signal?

    public enum CodingKeys: String, CodingKey {
        case changedAt = "changed_at"
        case failure
        case humanInput = "human_input"
        case lifecycle, signal
    }

    public init(changedAt: Date?, failure: Failure, humanInput: Bool, lifecycle: Lifecycle, signal: Signal?) {
        self.changedAt = changedAt
        self.failure = failure
        self.humanInput = humanInput
        self.lifecycle = lifecycle
        self.signal = signal
    }
}

// MARK: SessionStatus convenience initializers and mutators

public extension SessionStatus {
    init(data: Data) throws {
        self = try newJSONDecoder().decode(SessionStatus.self, from: data)
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
        changedAt: Date?? = nil,
        failure: Failure? = nil,
        humanInput: Bool? = nil,
        lifecycle: Lifecycle? = nil,
        signal: Signal?? = nil
    ) -> SessionStatus {
        return SessionStatus(
            changedAt: changedAt ?? self.changedAt,
            failure: failure ?? self.failure,
            humanInput: humanInput ?? self.humanInput,
            lifecycle: lifecycle ?? self.lifecycle,
            signal: signal ?? self.signal
        )
    }

    func jsonData() throws -> Data {
        return try newJSONEncoder().encode(self)
    }

    func jsonString(encoding: String.Encoding = .utf8) throws -> String? {
        return String(data: try self.jsonData(), encoding: encoding)
    }
}

public enum Failure: String, Codable {
    case failed = "failed"
    case none = "none"
}

public enum Lifecycle: String, Codable {
    case active = "active"
    case idle = "idle"
    case stopped = "stopped"
}

// MARK: - Signal
public struct Signal: Codable {
    public let epoch: String
    public let kind: Kind
    public let sequence: Int

    public init(epoch: String, kind: Kind, sequence: Int) {
        self.epoch = epoch
        self.kind = kind
        self.sequence = sequence
    }
}

// MARK: Signal convenience initializers and mutators

public extension Signal {
    init(data: Data) throws {
        self = try newJSONDecoder().decode(Signal.self, from: data)
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
        epoch: String? = nil,
        kind: Kind? = nil,
        sequence: Int? = nil
    ) -> Signal {
        return Signal(
            epoch: epoch ?? self.epoch,
            kind: kind ?? self.kind,
            sequence: sequence ?? self.sequence
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
    case bell = "bell"
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
