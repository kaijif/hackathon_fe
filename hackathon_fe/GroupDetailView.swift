//
//  GroupDetailView.swift
//  hackathon_fe
//
//  Detail screen for a safety group: tonight's monitoring state, members,
//  and invite actions.
//

import SwiftUI

struct GroupDetailView: View {
    let group: Group

    @EnvironmentObject private var appState: AppState

    @State private var members: [Member] = []
    @State private var currentNight: Night?
    @State private var isLoading = false
    @State private var showInvite = false

    var body: some View {
        List {
            tonightSection
            membersSection
            inviteSection
            errorSection
        }
        .navigationTitle(group.name)
        .task { await load() }
        .refreshable { await load() }
        .sheet(isPresented: $showInvite) {
            InviteView(group: group)
        }
    }

    private var tonightSection: some View {
        Section("Tonight") {
            if isLoading && currentNight == nil {
                ProgressView("Loading tonight\u{2026}")
            } else if let night = currentNight, night.status == .active || night.status == .pending {
                NavigationLink {
                    GuardianView(nightId: night.id, currentUserId: appState.currentUser?.id)
                        .onAppear { appState.track(nightId: night.id) }
                } label: {
                    Label("Open Guardian", systemImage: "shield.lefthalf.filled")
                        .font(.headline)
                }
            } else {
                NavigationLink {
                    StartNightView(group: group)
                } label: {
                    Label("Start Night", systemImage: "moon.stars.fill")
                        .font(.headline)
                }
            }
        }
    }

    private var membersSection: some View {
        Section("Members (\(members.count))") {
            if members.isEmpty && isLoading {
                ProgressView("Loading members\u{2026}")
            } else if members.isEmpty {
                Text("No members yet")
                    .foregroundStyle(.secondary)
            } else {
                ForEach(members) { member in
                    HStack {
                        Text(memberName(member))
                        Spacer()
                        if member.isAdmin == true {
                            Text("Admin")
                                .font(.caption)
                                .foregroundStyle(.secondary)
                        }
                    }
                }
            }
        }
    }

    private var inviteSection: some View {
        Section("Invite") {
            Button {
                showInvite = true
            } label: {
                Label("Invite with QR code", systemImage: "qrcode")
            }
        }
    }

    @ViewBuilder
    private var errorSection: some View {
        if let error = appState.errorMessage {
            Section {
                Text(error)
                    .foregroundStyle(.red)
            }
        }
    }

    private func memberName(_ member: Member) -> String {
        let name = member.name ?? "Member"
        return member.userId == appState.currentUser?.id ? "\(name) (You)" : name
    }

    private func load() async {
        isLoading = true
        defer { isLoading = false }

        members = await appState.members(ofGroup: group.id)
        currentNight = await appState.currentNight(forGroup: group.id)
    }
}

#Preview {
    NavigationStack {
        GroupDetailView(group: Group(id: "g1", name: "Night Crew", active: true, currNightId: nil, createdAt: nil, updatedAt: nil))
            .environmentObject(AppState())
    }
}
