//
//  GroupView.swift
//  hackathon_fe
//
//  A group's home screen. Opening a group lands here: it shows the live Guardian
//  monitor when a night is active, otherwise the group's members with a Start
//  Night button and a + to invite.
//

import SwiftUI

struct GroupView: View {
    let group: Group

    @EnvironmentObject private var appState: AppState
    @Environment(\.dismiss) private var dismiss

    @State private var currentNight: Night?
    @State private var hasLoaded = false

    /// How often the group home re-checks whether a night has started or ended,
    /// so it flips to/from the live Guardian monitor without a manual refresh.
    /// Lower = more real-time, at the cost of more requests. Tune here.
    private static let nightPollInterval: Duration = .seconds(4)

    var body: some View {
        content
            .task { await pollNight() }
    }

    @ViewBuilder
    private var content: some View {
        if let night = currentNight, isActive(night) {
            // Active night → live Guardian monitor (controls include End Night).
            GuardianView(
                nightId: night.id,
                currentUserId: appState.currentUser?.id,
                navigationTitle: group.name,
                group: group,
                onNightEnded: { currentNight = nil }
            )
        } else if !hasLoaded {
            ProgressView("Loading\u{2026}")
                .frame(maxWidth: .infinity, maxHeight: .infinity)
                .navigationTitle(group.name)
                .navigationBarTitleDisplayMode(.inline)
        } else {
            // No active night → the group's members in the main content, with a
            // Start Night button and a + to invite (both handled by GroupDetailView).
            GroupDetailView(
                group: group,
                onLeave: { dismiss() },
                title: group.name,
                showsStartNight: true
            )
        }
    }

    private func isActive(_ night: Night) -> Bool {
        night.status == .active || night.status == .pending
    }

    /// Periodically re-checks the group's night so the home flips to the live
    /// monitor when anyone starts a night, and back to the roster when the night
    /// ends (here or on another member's device). Runs until the view goes away.
    private func pollNight() async {
        while !Task.isCancelled {
            await refreshNight()
            try? await Task.sleep(for: Self.nightPollInterval)
        }
    }

    private func refreshNight() async {
        switch await appState.nightLookup(forGroup: group.id) {
        case .found(let night):
            currentNight = night
        case .absent:
            currentNight = nil
        case .failed:
            break // keep the last known state to avoid flicker on a network blip
        }
        hasLoaded = true
    }
}

#Preview {
    NavigationStack {
        GroupView(group: Group(id: "g1", name: "Night Crew", active: nil, currNightId: nil, createdAt: nil, updatedAt: nil))
            .environmentObject(AppState())
    }
}
