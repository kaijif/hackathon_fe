//
//  APIClient.swift
//  hackathon_fe
//
//  Thin networking layer for the NightWatch backend (see openapi.yaml).
//

import Foundation

nonisolated struct APIClient: Sendable {
    /// Base URL for all requests, configurable in Settings (see AppConfig).
    /// Computed so a change takes effect immediately for every APIClient,
    /// regardless of when it was constructed.
    var baseURL: URL { AppConfig.baseURL }

    struct APIError: LocalizedError {
        let status: Int?
        let message: String
        var errorDescription: String? { message }
    }

    // MARK: - Users

    func createUser(name: String, trustedContact: String?) async throws -> User {
        try await post("/users", body: CreateUserRequest(
            name: name,
            trustedContact: trustedContact,
            lat: nil,
            lng: nil,
            batteryLevel: nil
        ))
    }

    func getUser(id: String) async throws -> User {
        try await get("/users/\(id)")
    }

    func listUserGroups(userId: String) async throws -> [Group] {
        try await get("/users/\(userId)/groups")
    }

    /// Only groups that currently have an active night (User.listGroupsWithActiveNight).
    func listUserGroupsWithActiveNight(userId: String) async throws -> [Group] {
        try await get("/users/\(userId)/groups?activeNight=true")
    }

    // MARK: - User location, battery & devices

    /// Reports the user's current coordinates (and optionally battery). The server
    /// propagates these into every active night the user participates in.
    @discardableResult
    func updateLocation(userId: String, lat: Double, lng: Double, batteryLevel: Int?) async throws -> User {
        try await put("/users/\(userId)/location",
                      body: SetLocationRequest(lat: lat, lng: lng, batteryLevel: batteryLevel))
    }

    func getLocation(userId: String) async throws -> Location {
        try await get("/users/\(userId)/location")
    }

    @discardableResult
    func updateBattery(userId: String, batteryLevel: Int) async throws -> User {
        try await put("/users/\(userId)/battery", body: SetBatteryRequest(batteryLevel: batteryLevel))
    }

    /// Registers an APNs device token so the server can push alerts and check-ins.
    @discardableResult
    func registerDevice(userId: String, token: String, platform: String = "ios") async throws -> DeviceToken {
        try await post("/users/\(userId)/devices",
                       body: RegisterDeviceRequest(platform: platform, token: token))
    }

    func listDevices(userId: String) async throws -> [DeviceToken] {
        try await get("/users/\(userId)/devices")
    }

    func unregisterDevice(userId: String, token: String) async throws {
        try await deleteNoContent("/users/\(userId)/devices/\(token)")
    }

    // MARK: - Groups

    func createGroup(name: String, creatorUserId: String) async throws -> Group {
        try await post("/groups", body: CreateGroupRequest(name: name, creatorUserId: creatorUserId))
    }

    func getGroup(id: String) async throws -> Group {
        try await get("/groups/\(id)")
    }

    func joinGroup(groupId: String, userId: String) async throws {
        try await postNoContent("/groups/\(groupId)/join", body: JoinGroupRequest(userId: userId))
    }

    func listMembers(groupId: String) async throws -> [Member] {
        try await get("/groups/\(groupId)/members")
    }

    /// The group's current night (Group.currNight). Throws 404 when none exists.
    func getCurrentNight(groupId: String) async throws -> Night {
        try await get("/groups/\(groupId)/night")
    }

    // MARK: - Nights

    /// Creates a (pending) night for a group. Start it with `startNight`.
    func createNight(groupId: String, request: CreateNightRequest) async throws -> Night {
        try await post("/groups/\(groupId)/nights", body: request)
    }

    /// Full night view: night fields + current locations + participant statuses.
    func getNight(id: String) async throws -> NightView {
        try await get("/nights/\(id)")
    }

    func startNight(id: String) async throws -> Night {
        try await postNoBody("/nights/\(id)/start")
    }

    func endNight(id: String) async throws -> Night {
        try await postNoBody("/nights/\(id)/end")
    }

    func deleteNight(id: String) async throws {
        try await deleteNoContent("/nights/\(id)")
    }

    @discardableResult
    func setNightRange(nightId: String, maxRangeM: Int) async throws -> Night {
        try await put("/nights/\(nightId)/range", body: SetRangeRequest(maxRangeM: maxRangeM))
    }

    /// Manually runs the monitoring loop (also runs server-side in the background).
    @discardableResult
    func runCheck(nightId: String) async throws -> CheckResult {
        try await postNoBody("/nights/\(nightId)/check")
    }

    func listNightLocations(nightId: String) async throws -> [NightLocation] {
        try await get("/nights/\(nightId)/locations")
    }

    @discardableResult
    func reportNightLocation(nightId: String, userId: String, lat: Double, lng: Double, batteryLevel: Int?) async throws -> NightLocation {
        try await put("/nights/\(nightId)/locations/\(userId)",
                      body: SetLocationRequest(lat: lat, lng: lng, batteryLevel: batteryLevel))
    }

    func listStatuses(nightId: String) async throws -> [ParticipantStatus] {
        try await get("/nights/\(nightId)/statuses")
    }

    @discardableResult
    func notifyAll(nightId: String, body: String) async throws -> [Message] {
        try await post("/nights/\(nightId)/notify", body: NotifyRequest(body: body))
    }

    /// Acknowledge an "are you OK?" check-in. Backend endpoint to be added — see
    /// BACKEND_HANDOFF.md (`POST /nights/{id}/checkin/{userId}`).
    func checkIn(nightId: String, userId: String, ok: Bool = true,
                 lat: Double? = nil, lng: Double? = nil, batteryLevel: Int? = nil) async throws {
        try await postNoContent("/nights/\(nightId)/checkin/\(userId)",
                                body: CheckInRequest(ok: ok, lat: lat, lng: lng, batteryLevel: batteryLevel))
    }

    // MARK: - Core

    private func get<T: Decodable>(_ path: String) async throws -> T {
        try await send(makeRequest(path: path, method: "GET", body: nil))
    }

    private func post<T: Decodable, B: Encodable>(_ path: String, body: B) async throws -> T {
        let data = try JSONEncoder().encode(body)
        return try await send(makeRequest(path: path, method: "POST", body: data))
    }

    private func postNoContent<B: Encodable>(_ path: String, body: B) async throws {
        let data = try JSONEncoder().encode(body)
        try await sendNoContent(makeRequest(path: path, method: "POST", body: data))
    }

    private func postNoBody<T: Decodable>(_ path: String) async throws -> T {
        try await send(makeRequest(path: path, method: "POST", body: nil))
    }

    private func put<T: Decodable, B: Encodable>(_ path: String, body: B) async throws -> T {
        let data = try JSONEncoder().encode(body)
        return try await send(makeRequest(path: path, method: "PUT", body: data))
    }

    private func deleteNoContent(_ path: String) async throws {
        try await sendNoContent(makeRequest(path: path, method: "DELETE", body: nil))
    }

    private func makeRequest(path: String, method: String, body: Data?) -> URLRequest {
        let url = URL(string: path, relativeTo: baseURL)!
        var request = URLRequest(url: url)
        request.httpMethod = method
        request.setValue("application/json", forHTTPHeaderField: "Accept")
        if let body {
            request.httpBody = body
            request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        }
        return request
    }

    private func send<T: Decodable>(_ request: URLRequest) async throws -> T {
        let (data, response) = try await URLSession.shared.data(for: request)
        try Self.validate(data: data, response: response)
        do {
            return try JSONDecoder().decode(T.self, from: data)
        } catch {
            throw APIError(status: nil, message: "Couldn't read the server response.")
        }
    }

    private func sendNoContent(_ request: URLRequest) async throws {
        let (data, response) = try await URLSession.shared.data(for: request)
        try Self.validate(data: data, response: response)
    }

    private static func validate(data: Data, response: URLResponse) throws {
        guard let http = response as? HTTPURLResponse else {
            throw APIError(status: nil, message: "No response from the server.")
        }
        guard (200..<300).contains(http.statusCode) else {
            let serverMessage = (try? JSONDecoder().decode(APIErrorResponse.self, from: data))?.error
            throw APIError(status: http.statusCode,
                           message: serverMessage ?? "Request failed (HTTP \(http.statusCode)).")
        }
    }
}
