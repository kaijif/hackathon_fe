//
//  NightStore.swift
//  hackathon_fe
//
//  Observable state for a single active night, backing the Guardian screen.
//  Polls the server for the night's locations and participant statuses, runs
//  the monitoring loop, and exposes ready-to-render rows combining each
//  member's name, status, distance, battery, and coordinate.
//

import Foundation
import Combine
internal import CoreLocation

@MainActor
final class NightStore: ObservableObject {
    @Published private(set) var night: Night?
    @Published private(set) var locations: [NightLocation] = []
    @Published private(set) var statuses: [ParticipantStatus] = []
    @Published private(set) var members: [Member] = []
    @Published private(set) var isLoading = false
    @Published var errorMessage: String?

    let nightId: String
    private let currentUserId: String?
    private let api: APIClient
    private var pollTask: Task<Void, Never>?

    /// How often the Guardian screen refreshes participant locations, statuses
    /// and the night's state from the server. Lower = more real-time, at the
    /// cost of more requests. Tune here.
    private let pollInterval: Duration = .seconds(3)

    init(nightId: String, currentUserId: String?, api: APIClient = APIClient()) {
        self.nightId = nightId
        self.currentUserId = currentUserId
        self.api = api
    }

    // MARK: - Polling lifecycle

    func startPolling() {
        guard pollTask == nil else { return }
        pollTask = Task { [weak self] in
            guard let self else { return }
            while !Task.isCancelled {
                await self.refresh()
                _ = try? await self.api.runCheck(nightId: self.nightId)
                try? await Task.sleep(for: self.pollInterval)
            }
        }
    }

    func stopPolling() {
        pollTask?.cancel()
        pollTask = nil
    }

    // MARK: - Data

    func refresh() async {
        isLoading = true
        defer { isLoading = false }
        do {
            let view = try await api.getNight(id: nightId)
            night = view.night
            locations = view.currentLocations
            statuses = view.participantStatuses
            // Load member names once we know the group.
            if members.isEmpty, let groupId = view.night.groupId {
                members = (try? await api.listMembers(groupId: groupId)) ?? []
            }
            errorMessage = nil
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    /// Acknowledge the current user's "are you OK?" check-in.
    func checkIn(ok: Bool = true) async {
        guard let userId = currentUserId else { return }
        let loc = locations.first { $0.userId == userId }
        try? await api.checkIn(nightId: nightId, userId: userId, ok: ok,
                               lat: loc?.lat, lng: loc?.lng, batteryLevel: loc?.batteryLevel)
        await refresh()
    }

    /// Ends the night.
    @discardableResult
    func endNight() async -> Night? {
        do {
            let ended = try await api.endNight(id: nightId)
            night = ended
            stopPolling()
            return ended
        } catch {
            errorMessage = error.localizedDescription
            return nil
        }
    }

    // MARK: - Derived view data

    var centerCoordinate: CLLocationCoordinate2D? {
        guard let lat = night?.centerLat, let lng = night?.centerLng else { return nil }
        return CLLocationCoordinate2D(latitude: lat, longitude: lng)
    }

    var maxRangeMeters: Double? {
        night?.maxRangeM.map(Double.init)
    }

    var isCurrentUserAdmin: Bool {
        guard let userId = currentUserId else { return false }
        return members.first { $0.userId == userId }?.isAdmin == true
    }

    func memberName(for userId: String) -> String {
        if let name = members.first(where: { $0.userId == userId })?.name, !name.isEmpty {
            return name
        }
        if userId == currentUserId { return "You" }
        return "Member"
    }

    /// One row per participant, combining membership, status and last location.
    nonisolated struct Row: Identifiable, Hashable {
        let id: String
        let name: String
        let status: ParticipantStatusKind
        let detail: String?
        let distanceM: Double?
        let batteryLevel: Int?
        let reportedAt: String?
        let coordinate: CLLocationCoordinate2D?
        let isCurrentUser: Bool
        let isAdmin: Bool

        static func == (lhs: Row, rhs: Row) -> Bool {
            lhs.id == rhs.id && lhs.status == rhs.status && lhs.distanceM == rhs.distanceM
                && lhs.batteryLevel == rhs.batteryLevel
                && lhs.reportedAt == rhs.reportedAt
                && lhs.isAdmin == rhs.isAdmin
                && lhs.coordinate?.latitude == rhs.coordinate?.latitude
                && lhs.coordinate?.longitude == rhs.coordinate?.longitude
        }

        func hash(into hasher: inout Hasher) {
            hasher.combine(id)
            hasher.combine(status)
        }
    }

    var rows: [Row] {
        var ids = Set<String>()
        ids.formUnion(statuses.map(\.userId))
        ids.formUnion(locations.map(\.userId))
        ids.formUnion(members.map(\.userId))

        return ids.map { userId in
            let status = statuses.first { $0.userId == userId }
            let loc = locations.first { $0.userId == userId }
            let coordinate = loc.map { CLLocationCoordinate2D(latitude: $0.lat, longitude: $0.lng) }
            return Row(
                id: userId,
                name: memberName(for: userId),
                status: status?.status ?? .unknown,
                detail: status?.detail,
                distanceM: status?.distanceM,
                batteryLevel: loc?.batteryLevel,
                reportedAt: loc?.reportedAt,
                coordinate: coordinate,
                isCurrentUser: userId == currentUserId,
                isAdmin: members.first { $0.userId == userId }?.isAdmin == true
            )
        }
        .sorted { lhs, rhs in
            // Surface problems first, then alphabetical.
            if lhs.status.severity != rhs.status.severity {
                return lhs.status.severity > rhs.status.severity
            }
            return lhs.name < rhs.name
        }
    }
}

extension ParticipantStatusKind {
    /// Higher = more urgent; used to sort and color the Guardian list.
    var severity: Int {
        switch self {
        case .missing: return 5
        case .outOfRange: return 4
        case .lowBattery: return 3
        case .unknown: return 2
        case .outOfRangeSafe: return 1
        case .ok: return 0
        }
    }

    var label: String {
        switch self {
        case .ok: return "OK"
        case .outOfRange: return "Out of range"
        case .outOfRangeSafe: return "Safe (out of range)"
        case .lowBattery: return "Low battery"
        case .missing: return "Missing"
        case .unknown: return "Unknown"
        }
    }
}
