//
//  AppConfig.swift
//  hackathon_fe
//
//  App-wide configuration backed by UserDefaults. Currently just the backend
//  base URL, surfaced in Settings so the prototype can point at localhost or a
//  deployed server without a rebuild.
//

import Foundation

enum AppConfig {
    /// Backend used when nothing has been configured yet.
    static let defaultBaseURL = URL(string: "http://localhost:8080")!

    private static let baseURLKey = "apiBaseURL"

    /// Backend base URL for every API request. Reads/writes UserDefaults so a
    /// change in Settings is picked up immediately by all APIClient instances
    /// (APIClient.baseURL is computed from this).
    static var baseURL: URL {
        get {
            guard let stored = UserDefaults.standard.string(forKey: baseURLKey),
                  let url = URL(string: stored) else {
                return defaultBaseURL
            }
            return url
        }
        set {
            UserDefaults.standard.set(newValue.absoluteString, forKey: baseURLKey)
        }
    }
}
