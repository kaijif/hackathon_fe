//
//  InviteView.swift
//  hackathon_fe
//
//  The QR-code modal opened by the "+" button. Lets the user pick (or create)
//  a group and shows a scannable invite link so others can join.
//

import SwiftUI

struct InviteView: View {
    /// When provided, this group is pre-selected (e.g. tapping a group row).
    var preselectedGroupId: String?

    @EnvironmentObject private var appState: AppState
    @Environment(\.dismiss) private var dismiss

    @State private var selectedGroupId: String?
    @State private var newGroupName = ""
    @State private var isCreating = false

    private var selectedGroup: Group? {
        appState.groups.first { $0.id == selectedGroupId }
    }

    var body: some View {
        NavigationStack {
            Form {
                if !appState.groups.isEmpty {
                    Section("Invite to group") {
                        Picker("Group", selection: $selectedGroupId) {
                            ForEach(appState.groups) { group in
                                Text(group.name).tag(group.id as String?)
                            }
                        }
                    }
                }

                Section("Create a new group") {
                    TextField("Group name", text: $newGroupName)
                    Button {
                        Task { await createGroup() }
                    } label: {
                        HStack {
                            Text("Create & show QR")
                            if isCreating {
                                Spacer()
                                ProgressView()
                            }
                        }
                    }
                    .disabled(trimmedName.isEmpty || isCreating)
                }

                if let group = selectedGroup {
                    Section("Scan to join \u{201C}\(group.name)\u{201D}") {
                        VStack(spacing: 12) {
                            QRCodeView(content: DeepLink.joinURLString(groupId: group.id))
                                .frame(maxWidth: .infinity)

                            Text(DeepLink.joinURLString(groupId: group.id))
                                .font(.caption)
                                .foregroundStyle(.secondary)
                                .multilineTextAlignment(.center)
                                .textSelection(.enabled)

                            ShareLink(item: DeepLink.joinURLString(groupId: group.id)) {
                                Label("Share invite link", systemImage: "square.and.arrow.up")
                            }
                        }
                        .padding(.vertical, 8)
                    }
                } else {
                    Section {
                        Text("Create or select a group to generate its invite QR code.")
                            .font(.footnote)
                            .foregroundStyle(.secondary)
                    }
                }

                if let error = appState.errorMessage {
                    Section {
                        Text(error).foregroundStyle(.red)
                    }
                }
            }
            .navigationTitle("Invite")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .confirmationAction) {
                    Button("Done") { dismiss() }
                }
            }
            .onAppear {
                if selectedGroupId == nil {
                    selectedGroupId = preselectedGroupId ?? appState.groups.first?.id
                }
            }
        }
    }

    private var trimmedName: String {
        newGroupName.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    private func createGroup() async {
        isCreating = true
        defer { isCreating = false }
        if let group = await appState.createGroup(name: trimmedName) {
            selectedGroupId = group.id
            newGroupName = ""
        }
    }
}
