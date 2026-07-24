// Hand-written thin REST transport for the Swift SDK.
// Routes: protocol/openapi.yaml (REST-binding annex).
// Models: ../Generated (quicktype).

import Foundation

public final class GatewayClient: @unchecked Sendable {
    private let baseURL: URL
    private let token: String?
    private let session: URLSession
    public private(set) var clientInstanceId: String?

    public init(baseURL: URL, token: String? = nil, session: URLSession = .shared) {
        self.baseURL = baseURL
        self.token = token
        self.session = session
    }

    public struct WsTicket: Decodable, Sendable {
        public let ticket: String
        public let client_instance_id: String
    }

    public func mintWsTicket() async throws -> WsTicket {
        var req = URLRequest(url: baseURL.appendingPathComponent("api/ws-ticket"))
        req.httpMethod = "POST"
        applyAuth(&req)
        let (data, res) = try await session.data(for: req)
        try throwIfNeeded(res, data: data)
        let ticket = try JSONDecoder().decode(WsTicket.self, from: data)
        clientInstanceId = ticket.client_instance_id
        return ticket
    }

    public func respondApproval(sessionId: String, approvalId: String, decision: String) async throws {
        precondition(decision == "accept" || decision == "deny")
        let path = "api/sessions/\(sessionId)/approvals/\(approvalId)"
        var req = URLRequest(url: baseURL.appendingPathComponent(path))
        req.httpMethod = "POST"
        req.setValue("application/json", forHTTPHeaderField: "Content-Type")
        applyAuth(&req)
        var body: [String: String] = ["decision": decision]
        if let ci = clientInstanceId {
            body["client_instance_id"] = ci
        }
        req.httpBody = try JSONEncoder().encode(body)
        let (data, res) = try await session.data(for: req)
        try throwIfNeeded(res, data: data)
    }

    private func applyAuth(_ req: inout URLRequest) {
        req.setValue("application/json", forHTTPHeaderField: "Accept")
        if let token {
            req.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        }
        if let clientInstanceId {
            req.setValue(clientInstanceId, forHTTPHeaderField: "X-Client-Instance-ID")
        }
    }

    private func throwIfNeeded(_ res: URLResponse, data: Data) throws {
        guard let http = res as? HTTPURLResponse else { return }
        guard (200...299).contains(http.statusCode) else {
            let text = String(data: data, encoding: .utf8) ?? ""
            throw NSError(
                domain: "AgentGrid.GatewayClient",
                code: http.statusCode,
                userInfo: [NSLocalizedDescriptionKey: text]
            )
        }
    }
}
