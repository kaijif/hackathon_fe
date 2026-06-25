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

    var body: some View {
        content
            .onAppear { Task { await loadNight() } }
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
                showsStartNight: true,
                glassBackground: true
            )
        }
    }

    private func isActive(_ night: Night) -> Bool {
        night.status == .active || night.status == .pending
    }

    private func loadNight() async {
        currentNight = await appState.currentNight(forGroup: group.id)
        hasLoaded = true
    }
}

#Preview {
    NavigationStack {
        GroupView(group: Group(id: "g1", name: "Night Crew", active: nil, currNightId: nil, createdAt: nil, updatedAt: nil))
            .environmentObject(AppState())
    }
}
