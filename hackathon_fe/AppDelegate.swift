//
//  AppDelegate.swift
//  hackathon_fe
//
//  Bridges UIKit app-delegate callbacks that SwiftUI's `App` can't receive:
//  APNs device-token registration and notification handling. Wired in via
//  `@UIApplicationDelegateAdaptor` in hackathon_feApp.
//

import UIKit
import UserNotifications

final class AppDelegate: NSObject, UIApplicationDelegate {
    func application(_ application: UIApplication,
                     didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]? = nil) -> Bool {
        UNUserNotificationCenter.current().delegate = self
        PushManager.shared.configureCategories()
        return true
    }

    func application(_ application: UIApplication,
                     didRegisterForRemoteNotificationsWithDeviceToken deviceToken: Data) {
        PushManager.shared.didRegister(deviceToken: deviceToken)
    }

    func application(_ application: UIApplication,
                     didFailToRegisterForRemoteNotificationsWithError error: Error) {
        // No APNs in the simulator / without the entitlement — safe to ignore.
    }
}

extension AppDelegate: UNUserNotificationCenterDelegate {
    /// Show check-in/alert notifications even while the app is foregrounded.
    func userNotificationCenter(_ center: UNUserNotificationCenter,
                                willPresent notification: UNNotification,
                                withCompletionHandler completionHandler: @escaping (UNNotificationPresentationOptions) -> Void) {
        completionHandler([.banner, .sound, .list])
    }

    /// Handle taps and notification actions (e.g. the "I'm OK" check-in action).
    func userNotificationCenter(_ center: UNUserNotificationCenter,
                                didReceive response: UNNotificationResponse,
                                withCompletionHandler completionHandler: @escaping () -> Void) {
        PushManager.shared.handle(response: response)
        completionHandler()
    }
}
