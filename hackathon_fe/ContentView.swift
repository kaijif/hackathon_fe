//
//  ContentView.swift
//  hackathon_fe
//
//  Created by Kaiji Fu on 6/23/26.
//

import SwiftUI

struct ContentView: View {
    @EnvironmentObject private var appState: AppState

    @State private var showingNewGroup = false
    @State private var showingSettings = false

    var body: some View {
        NavigationStack {
            content
                .navigationTitle("Groups")
                .toolbar {
                    ToolbarItem(placement: .topBarLeading) {
                        Button {
                            showingSettings = true
                        } label: {
                            Image(systemName: "gearshape")
                        }
                        .accessibilityLabel("Settings")
                    }

                    ToolbarItem(placement: .topBarTrailing) {
                        Button {
                            showingNewGroup = true
                        } label: {
                            Image(systemName: "plus")
                        }
                        .accessibilityLabel("Create a group")
                    }
                }
                .refreshable { await appState.refreshGroups() }
                .sheet(isPresented: $showingNewGroup) {
                    NewGroupView()
                }
                .navigationDestination(for: Group.self) { group in
                    GroupView(group: group)
                }
        }
        .sheet(item: $appState.pendingJoin) { request in
            JoinGroupView(groupId: request.id)
        }
        .sheet(isPresented: $showingSettings) {
            NavigationStack {
                SettingsView()
            }
        }
        .sheet(item: guardianRoute) { route in
            NavigationStack {
                GuardianView(nightId: route.id, currentUserId: appState.currentUser?.id)
            }
        }
        .fullScreenCover(isPresented: $appState.needsOnboarding) {
            OnboardingView()
        }
        .task { await appState.bootstrap() }
    }

    /// Bridges `appState.pendingNightId` (set when a safety alert is tapped) into
    /// an `Identifiable` so it can drive the Guardian sheet.
    private var guardianRoute: Binding<GuardianRoute?> {
        Binding(
            get: { appState.pendingNightId.map(GuardianRoute.init) },
            set: { appState.pendingNightId = $0?.id }
        )
    }

    @ViewBuilder
    private var content: some View {
        if appState.groups.isEmpty {
            emptyState
        } else {
            groupList
        }
    }

    private var groupList: some View {
        List {
            ForEach(appState.groups) { group in
                NavigationLink(value: group) {
                    HStack {
                        VStack(alignment: .leading, spacing: 2) {
                            Text(group.name)
                                .font(.body)
                                .foregroundStyle(.primary)
                            if group.active == true {
                                Label("Active night", systemImage: "moon.stars.fill")
                                    .font(.caption)
                                    .foregroundStyle(.green)
                            }
                        }
                        Spacer()
                    }
                }
            }
        }
    }

    private var emptyState: some View {
        ContentUnavailableView {
            Label("No Groups Yet", systemImage: "person.3")
        } description: {
            Text("Tap + to create a group and share its QR code, or scan a friend's QR code to join.")
        } actions: {
            Button {
                showingNewGroup = true
            } label: {
                Label("Create a Group", systemImage: "plus")
            }
            .buttonStyle(.borderedProminent)
        }
    }
}

/// Identifiable wrapper for a night id so a tapped safety alert can present the
/// Guardian screen via `.sheet(item:)`.
struct GuardianRoute: Identifiable {
    let id: String
}

#Preview {
    ContentView()
        .environmentObject(AppState())
}
