//
//  hackathon_feApp.swift
//  hackathon_fe
//
//  Created by Kaiji Fu on 6/23/26.
//

import SwiftUI

@main
struct hackathon_feApp: App {
    @UIApplicationDelegateAdaptor(AppDelegate.self) private var appDelegate
    @StateObject private var appState = AppState()

    var body: some Scene {
        WindowGroup {
            ContentView()
                .environmentObject(appState)
                .onOpenURL { url in
                    appState.handleDeepLink(url)
                }
        }
    }
}
