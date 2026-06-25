//
//  GroupView.swift
//  hackathon_fe
//
//  A group's home screen. Opening a group lands here: it shows the live Guardian
//  monitor when a night is active, otherwise a prompt to start one. The people
//  button in the top-right opens the full group detail (members & invite).
//

import SwiftUI

struct GroupView: View {
    let group: Group

    @EnvironmentObject private var appState: AppState
    @Environment(\.dismiss) private var dismiss

    @State private var currentNight: Night?
    @State private var hasLoaded = false
    @State private var showGroupDetail = false

    var body: some View {
        content
            .sheet(isPresented: $showGroupDetail) {
                NavigationStack {
                    GroupDetailView(group: group, onLeave: {
                        showGroupDetail = false
                        dismiss()
                    })
                        .toolbar {
                            ToolbarItem(placement: .topBarTrailing) {
                                Button {
                                    showGroupDetail = false
                                } label: {
                                    Image(systemName: "xmark")
                                }
                                .accessibilityLabel("Close")
                            }
                        }
                }
            }
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
                group: group
            )
        } else if !hasLoaded {
            ProgressView("Loading\u{2026}")
                .frame(maxWidth: .infinity, maxHeight: .infinity)
                .navigationTitle(group.name)
                .navigationBarTitleDisplayMode(.inline)
                .toolbar { groupToolbar }
        } else {
            noNightView
        }
    }

    // No active night → prompt to start one, with the action in the same bottom
    // bar position the End Night button occupies during an active night.
    private var noNightView: some View {
        VStack(spacing: 0) {
            ContentUnavailableView {
                Label("No Active Night", systemImage: "moon.zzz")
            } description: {
                Text("Start a night to begin watching over the group.")
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity)

            VStack(spacing: 10) {
                NavigationLink {
                    StartNightView(group: group)
                } label: {
                    Label("Start Night", systemImage: "moon.stars.fill")
                        .frame(maxWidth: .infinity, minHeight: 50)
                }
                .buttonStyle(.borderedProminent)
            }
            .padding()
            .background(.bar)
        }
        .navigationTitle(group.name)
        .navigationBarTitleDisplayMode(.inline)
        .toolbar { groupToolbar }
    }

    @ToolbarContentBuilder
    private var groupToolbar: some ToolbarContent {
        ToolbarItem(placement: .topBarTrailing) {
            Button { showGroupDetail = true } label: {
                Image(systemName: "person.2")
            }
            .accessibilityLabel("Group details")
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
