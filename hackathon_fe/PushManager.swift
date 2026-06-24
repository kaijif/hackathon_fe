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
import UserNotifications
import UIKit

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

    /// Sends the token to the backend once both the token and a user id exist.
    func registerDeviceIfPossible() {
        guard let token = deviceToken, let userId = userIdProvider() else { return }
        Task { [api] in try? await api.registerDevice(userId: userId, token: token) }
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
