//
//  PushManager.swift
//  hackathon_fe
//
//  Owns push-notification authorization, the APNs device token, and the
//  notification categories the backend's payloads line up with:
//    - CHECKIN: a periodic "are you OK?" reminder with an "I'm OK" action.
//    - ALERT:   a group safety alert (battery/range/missing) that opens Guardian.
//  See BACKEND_HANDOFF.md for the expected APNs payload conventions.
//

import Foundation
import Combine
import UserNotifications
import UIKit
import os

@MainActor
final class PushManager: NSObject, ObservableObject {
    static let shared = PushManager()

    @Published private(set) var deviceToken: String?
    @Published private(set) var authorizationGranted = false
    /// Set when the user taps a safety alert; the UI observes this to open the
    /// Guardian screen for the referenced night.
    @Published var pendingNightId: String?

    // Identifiers shared with the backend's APNs payloads.
    static let checkInCategory = "CHECKIN"
    static let alertCategory = "ALERT"
    static let okActionId = "CHECKIN_OK"

    private let api = APIClient()
    /// Supplies the signed-in user id so device registration targets the user.
    var userIdProvider: () -> String? = { nil }
    /// Routes device-registration failures to the app's shared error UI.
    var onError: (String) -> Void = { _ in }

    static let logger = Logger(subsystem: Bundle.main.bundleIdentifier ?? "hackathon_fe",
                               category: "Push")

    /// Registers the notification categories/actions. Call once at launch.
    func configureCategories() {
        let okAction = UNNotificationAction(identifier: Self.okActionId, title: "I'm OK", options: [])
        let checkIn = UNNotificationCategory(identifier: Self.checkInCategory,
                                             actions: [okAction], intentIdentifiers: [], options: [])
        let alert = UNNotificationCategory(identifier: Self.alertCategory,
                                           actions: [], intentIdentifiers: [], options: [])
        UNUserNotificationCenter.current().setNotificationCategories([checkIn, alert])
    }

    /// Asks for notification permission and, if granted, registers with APNs.
    func requestAuthorization() {
        UNUserNotificationCenter.current().requestAuthorization(options: [.alert, .sound, .badge]) { granted, _ in
            Task { @MainActor in
                self.authorizationGranted = granted
                if granted { UIApplication.shared.registerForRemoteNotifications() }
            }
        }
    }

    /// Called by the AppDelegate once APNs returns a device token.
    func didRegister(deviceToken data: Data) {
        deviceToken = data.map { String(format: "%02x", $0) }.joined()
        registerDeviceIfPossible()
    }

    /// Called by the AppDelegate when APNs registration fails at the OS level
    /// (e.g. no push entitlement, no network, or running in the Simulator).
    /// Logs the failure and surfaces a detailed reason to the UI.
    func didFailToRegister(error: Error) {
        let message = Self.apnsFailureMessage(for: error)
        Self.logger.error("APNs registration failed: \(message, privacy: .public)")
        onError(message)
    }

    /// Builds a user-facing reason for a failed APNs registration, including the
    /// underlying error domain and code to aid debugging entitlement/simulator issues.
    nonisolated static func apnsFailureMessage(for error: Error) -> String {
        let nsError = error as NSError
        return "Couldn't register for push notifications. "
            + "\(nsError.localizedDescription) (\(nsError.domain) \(nsError.code))"
    }

    /// Sends the token to the backend once both the token and a user id exist.
    /// Surfaces a detailed error to the UI if the registration request fails.
    func registerDeviceIfPossible() {
        guard let token = deviceToken, let userId = userIdProvider() else { return }
        Task { [api, weak self] in
            do {
                try await api.registerDevice(userId: userId, token: token)
            } catch {
                let message = Self.registrationFailureMessage(for: error)
                Self.logger.error("Device registration request failed: \(message, privacy: .public)")
                self?.onError(message)
            }
        }
    }

    /// Builds a user-facing reason for a failed device registration, including the
    /// HTTP status and server message when the backend supplied them.
    nonisolated static func registrationFailureMessage(for error: Error) -> String {
        let detail: String
        if let apiError = error as? APIClient.APIError {
            if let status = apiError.status, !apiError.message.contains("HTTP \(status)") {
                detail = "HTTP \(status): \(apiError.message)"
            } else {
                detail = apiError.message
            }
        } else {
            detail = error.localizedDescription
        }
        return "Couldn't register this device for notifications. \(detail)"
    }

    /// Handles a tapped notification or one of its actions.
    func handle(response: UNNotificationResponse) {
        let info = response.notification.request.content.userInfo
        let nightId = info["nightId"] as? String
        let userId = (info["userId"] as? String) ?? userIdProvider()

        if response.actionIdentifier == Self.okActionId, let nightId, let userId {
            Task { [api] in try? await api.checkIn(nightId: nightId, userId: userId, ok: true) }
            return
        }
        // A default tap (typically on an ALERT) routes to the Guardian screen.
        if let nightId { pendingNightId = nightId }
    }
}
