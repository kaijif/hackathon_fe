//
//  GroupDetailView.swift
//  hackathon_fe
//
//  Detail screen for a safety group: members, invite, and leave actions.
//

import SwiftUI

struct GroupDetailView: View {
    let group: Group
    /// Called after the user successfully leaves, so the presenter can return
    /// to the groups list.
    var onLeave: () -> Void = {}

    @EnvironmentObject private var appState: AppState

    @State private var members: [Member] = []
    @State private var isLoading = false
    @State private var showInvite = false
    @State private var showLeaveConfirmation = false
    @State private var isLeaving = false

    var body: some View {
        List {
            membersSection
            inviteSection
            leaveSection
            errorSection
        }
        .navigationTitle(group.name)
        .task { await load() }
        .refreshable { await load() }
        .sheet(isPresented: $showInvite) {
            InviteView(group: group)
        }
        .confirmationDialog("Leave \u{201C}\(group.name)\u{201D}?",
                            isPresented: $showLeaveConfirmation,
                            titleVisibility: .visible) {
            Button("Leave Group", role: .destructive) {
                Task { await leave() }
            }
            Button("Cancel", role: .cancel) {}
        } message: {
            Text("You'll stop receiving this group's alerts and check-ins.")
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

    private var leaveSection: some View {
        Section {
            Button(role: .destructive) {
                showLeaveConfirmation = true
            } label: {
                HStack {
                    Label("Leave Group", systemImage: "rectangle.portrait.and.arrow.right")
                    if isLeaving {
                        Spacer()
                        ProgressView()
                    }
                }
            }
            .disabled(isLeaving)
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
    }

    private func leave() async {
        isLeaving = true
        defer { isLeaving = false }
        if await appState.leaveGroup(group.id) {
            onLeave()
        }
    }
}

#Preview {
    NavigationStack {
        GroupDetailView(group: Group(id: "g1", name: "Night Crew", active: true, currNightId: nil, createdAt: nil, updatedAt: nil))
            .environmentObject(AppState())
    }
}
