//
//  AppState.swift
//  hackathon_fe
//
//  Observable app-wide state: the current user, their groups, and the
//  pending QR-code join request.
//

import Foundation
import SwiftUI
import Combine

/// Identifiable wrapper so a pending join can drive a `.sheet(item:)`.
nonisolated struct JoinRequest: Identifiable, Hashable, Sendable {
    let id: String // groupId
}

@MainActor
final class AppState: ObservableObject {
    @Published var currentUser: User?
    @Published var groups: [Group] = []
    @Published var isLoadingGroups = false
    @Published var errorMessage: String?

    /// Drives the first-launch profile sheet.
    @Published var needsOnboarding = false
    /// Drives the join-group confirmation sheet (only set once a user exists).
    @Published var pendingJoin: JoinRequest?

    private let api = APIClient()
    private let userIdKey = "currentUserId"
    private let userNameKey = "currentUserName"

    /// A join link that arrived before a user existed; flushed after onboarding.
    private var queuedJoinGroupId: String?

    // MARK: - Managers & monitoring state

    let locationManager = LocationManager()
    let pushManager = PushManager.shared

    /// The night whose locations should also receive direct location reports
    /// (the one currently being started or viewed on the Guardian screen).
    @Published var trackedNightId: String?

    /// Drives navigation to the Guardian screen when a safety alert is tapped.
    var pendingNightId: String? {
        get { pushManager.pendingNightId }
        set { pushManager.pendingNightId = newValue }
    }

    private var cancellables = Set<AnyCancellable>()

    init() {
        locationManager.userIdProvider = { [weak self] in self?.currentUser?.id }
        locationManager.activeNightIdProvider = { [weak self] in self?.trackedNightId }
        pushManager.userIdProvider = { [weak self] in self?.currentUser?.id }
        // Re-publish nested manager changes so views observing AppState refresh.
        locationManager.objectWillChange
            .sink { [weak self] in self?.objectWillChange.send() }
            .store(in: &cancellables)
        pushManager.objectWillChange
            .sink { [weak self] in self?.objectWillChange.send() }
            .store(in: &cancellables)
    }

    // MARK: - Lifecycle

    func bootstrap() async {
        guard currentUser == nil else { return }

        if let storedId = UserDefaults.standard.string(forKey: userIdKey) {
            do {
                currentUser = try await api.getUser(id: storedId)
            } catch {
                // Server unreachable or user missing: fall back to the cached name
                // so the app still works against a freshly restarted backend.
                let name = UserDefaults.standard.string(forKey: userNameKey) ?? "Me"
                currentUser = User(id: storedId, name: name)
            }
            await refreshGroups()
            flushQueuedJoin()
            onUserReady()
        } else {
            needsOnboarding = true
        }
    }

    func completeOnboarding(name: String, trustedContact: String?) async {
        errorMessage = nil
        do {
            let user = try await api.createUser(name: name, trustedContact: trustedContact)
            persist(user)
            currentUser = user
            needsOnboarding = false
            await refreshGroups()
            flushQueuedJoin()
            onUserReady()
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    // MARK: - Groups

    func refreshGroups() async {
        guard let userId = currentUser?.id else { return }
        isLoadingGroups = true
        defer { isLoadingGroups = false }
        do {
            groups = try await api.listUserGroups(userId: userId)
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    func createGroup(name: String) async -> Group? {
        guard let userId = currentUser?.id else { return nil }
        errorMessage = nil
        do {
            let group = try await api.createGroup(name: name, creatorUserId: userId)
            await refreshGroups()
            return group
        } catch {
            errorMessage = error.localizedDescription
            return nil
        }
    }

    func fetchGroup(id: String) async throws -> Group {
        try await api.getGroup(id: id)
    }

    func join(groupId: String) async throws {
        guard let userId = currentUser?.id else {
            throw APIClient.APIError(status: nil, message: "You need a profile before joining a group.")
        }
        try await api.joinGroup(groupId: groupId, userId: userId)
        await refreshGroups()
    }

    // MARK: - Nights & monitoring

    func members(ofGroup groupId: String) async -> [Member] {
        (try? await api.listMembers(groupId: groupId)) ?? []
    }

    /// The group's current night, or nil if none is active (the server returns 404).
    func currentNight(forGroup groupId: String) async -> Night? {
        try? await api.getCurrentNight(groupId: groupId)
    }

    /// Creates and immediately starts a night for the group.
    func startNight(forGroup groupId: String, config: CreateNightRequest) async -> Night? {
        errorMessage = nil
        do {
            let created = try await api.createNight(groupId: groupId, request: config)
            let started = try await api.startNight(id: created.id)
            trackedNightId = started.id
            locationManager.uploadNow()
            await refreshGroups()
            return started
        } catch {
            errorMessage = error.localizedDescription
            return nil
        }
    }

    @discardableResult
    func endNight(_ nightId: String) async -> Night? {
        do {
            let ended = try await api.endNight(id: nightId)
            if trackedNightId == nightId { trackedNightId = nil }
            await refreshGroups()
            return ended
        } catch {
            errorMessage = error.localizedDescription
            return nil
        }
    }

    /// Marks a night as the one currently in focus so background location updates
    /// also report directly into it.
    func track(nightId: String?) {
        trackedNightId = nightId
    }

    // MARK: - Permissions

    private func onUserReady() {
        primePermissions()
        pushManager.registerDeviceIfPossible()
    }

    /// Requests background location + notification permission and starts tracking.
    func primePermissions() {
        switch locationManager.authorizationStatus {
        case .authorizedAlways, .authorizedWhenInUse:
            locationManager.startTracking()
        default:
            locationManager.requestAuthorization()
        }
        pushManager.requestAuthorization()
    }

    // MARK: - Deep links

    func handleDeepLink(_ url: URL) {
        guard let groupId = DeepLink.groupId(fromJoinURL: url) else { return }
        if currentUser != nil {
            pendingJoin = JoinRequest(id: groupId)
        } else {
            // Wait until the user finishes onboarding before prompting to join.
            queuedJoinGroupId = groupId
        }
    }

    private func flushQueuedJoin() {
        guard let groupId = queuedJoinGroupId else { return }
        queuedJoinGroupId = nil
        pendingJoin = JoinRequest(id: groupId)
    }

    // MARK: - Persistence

    private func persist(_ user: User) {
        UserDefaults.standard.set(user.id, forKey: userIdKey)
        UserDefaults.standard.set(user.name, forKey: userNameKey)
    }
}
