//
//  Models.swift
//  hackathon_fe
//
//  Codable models mirroring the NightWatch API (see openapi.yaml).
//

import Foundation

// Marked `nonisolated` so these value types can be encoded/decoded and conform
// to Codable/Hashable/Identifiable off the main actor, even though the project
// defaults to MainActor isolation.

nonisolated struct User: Codable, Identifiable, Hashable, Sendable {
    let id: String
    let name: String
    var trustedContact: String?
    var lat: Double?
    var lng: Double?
    var batteryLevel: Int?
    var locationUpdatedAt: String?
    var createdAt: String?
    var updatedAt: String?
}

nonisolated struct Group: Codable, Identifiable, Hashable, Sendable {
    let id: String
    let name: String
    var active: Bool?
    var currNightId: String?
    var createdAt: String?
    var updatedAt: String?
}

nonisolated struct Member: Codable, Identifiable, Hashable, Sendable {
    let userId: String
    var name: String?
    var isAdmin: Bool?
    var joinedAt: String?

    var id: String { userId }
}

// MARK: - Request bodies

nonisolated struct CreateUserRequest: Codable, Sendable {
    let name: String
    var trustedContact: String?
    var lat: Double?
    var lng: Double?
    var batteryLevel: Int?
}

nonisolated struct CreateGroupRequest: Codable, Sendable {
    let name: String
    let creatorUserId: String
}

nonisolated struct JoinGroupRequest: Codable, Sendable {
    let userId: String
}

// MARK: - Nights & monitoring

nonisolated enum NightStatus: String, Codable, Sendable, Hashable {
    case pending, active, ended
}

nonisolated struct Night: Codable, Identifiable, Hashable, Sendable {
    let id: String
    var groupId: String?
    var agentId: String?
    var centerLat: Double?
    var centerLng: Double?
    var timeLimitMin: Int?
    var checkInLimitMin: Int?
    var checkInEveryMin: Int?
    var maxRangeM: Int?
    var lowBatteryThreshold: Int?
    var status: NightStatus?
    var startedAt: String?
    var endedAt: String?
    var lastCheckedAt: String?
    var createdAt: String?
    var updatedAt: String?
}

nonisolated struct NightLocation: Codable, Identifiable, Hashable, Sendable {
    let nightId: String
    let userId: String
    var lat: Double
    var lng: Double
    var batteryLevel: Int?
    var reportedAt: String?

    var id: String { userId }
}

/// Mirrors the `ParticipantStatus.status` enum in openapi.yaml. The `unknown`
/// case also absorbs any future server value so decoding never fails.
nonisolated enum ParticipantStatusKind: String, Codable, Sendable, Hashable {
    case ok
    case outOfRange = "out_of_range"
    case lowBattery = "low_battery"
    case missing
    case unknown

    init(from decoder: Decoder) throws {
        let raw = try decoder.singleValueContainer().decode(String.self)
        self = ParticipantStatusKind(rawValue: raw) ?? .unknown
    }
}

nonisolated struct ParticipantStatus: Codable, Identifiable, Hashable, Sendable {
    let nightId: String
    let userId: String
    var status: ParticipantStatusKind
    var detail: String?
    var distanceM: Double?
    var updatedAt: String?

    var id: String { userId }
}

/// `GET /nights/{id}` returns a Night flattened together with its current
/// locations and participant statuses (openapi `NightView` is an allOf of
/// `Night` + the two arrays), so we decode the Night from the same container.
nonisolated struct NightView: Decodable, Hashable, Sendable {
    let night: Night
    var currentLocations: [NightLocation]
    var participantStatuses: [ParticipantStatus]

    private enum CodingKeys: String, CodingKey {
        case currentLocations, participantStatuses
    }

    init(from decoder: Decoder) throws {
        night = try Night(from: decoder)
        let container = try decoder.container(keyedBy: CodingKeys.self)
        currentLocations = try container.decodeIfPresent([NightLocation].self, forKey: .currentLocations) ?? []
        participantStatuses = try container.decodeIfPresent([ParticipantStatus].self, forKey: .participantStatuses) ?? []
    }
}

nonisolated struct CheckResult: Codable, Sendable, Hashable {
    var nightId: String?
    var ended: Bool?
    var alerts: Int?
    var statuses: [ParticipantStatus]?
}

nonisolated struct Location: Codable, Sendable, Hashable {
    var lat: Double?
    var lng: Double?
    var batteryLevel: Int?
    var updatedAt: String?
}

nonisolated struct Coords: Codable, Sendable, Hashable {
    let lat: Double
    let lng: Double
}

nonisolated struct DeviceToken: Codable, Identifiable, Hashable, Sendable {
    let id: String
    var userId: String?
    var platform: String?
    var token: String?
    var createdAt: String?
    var updatedAt: String?
}

nonisolated struct Message: Codable, Identifiable, Hashable, Sendable {
    let id: String
    var nightId: String?
    var groupId: String?
    var recipientUserId: String?
    var recipientContact: String?
    var kind: String?
    var body: String?
    var sender: String?
    var createdAt: String?
}

// MARK: - Request bodies (nights, location, devices)

nonisolated struct SetLocationRequest: Codable, Sendable {
    let lat: Double
    let lng: Double
    var batteryLevel: Int?
}

nonisolated struct SetBatteryRequest: Codable, Sendable {
    let batteryLevel: Int
}

nonisolated struct RegisterDeviceRequest: Codable, Sendable {
    let platform: String
    let token: String
}

nonisolated struct CreateNightRequest: Codable, Sendable {
    var agentId: String?
    var center: Coords?
    var timeLimitMin: Int?
    var checkInLimitMin: Int?
    var checkInEveryMin: Int?
    var maxRangeM: Int?
    var lowBatteryThreshold: Int?
}

nonisolated struct SetRangeRequest: Codable, Sendable {
    let maxRangeM: Int
}

nonisolated struct NotifyRequest: Codable, Sendable {
    let body: String
}

/// Body for the (backend TODO) check-in acknowledgement endpoint.
/// See BACKEND_HANDOFF.md — `POST /nights/{id}/checkin/{userId}`.
nonisolated struct CheckInRequest: Codable, Sendable {
    var ok: Bool = true
    var lat: Double?
    var lng: Double?
    var batteryLevel: Int?
}

// MARK: - Error envelope

nonisolated struct APIErrorResponse: Codable, Sendable {
    let error: String?
}
