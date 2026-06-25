//
//  NewGroupView.swift
//  hackathon_fe
//
//  Create-a-group form opened by the "+" button on the Groups screen. Once the
//  group is created the user lands back on the list and can open it to share its
//  invite QR code (see InviteView).
//

import SwiftUI

struct NewGroupView: View {
    @EnvironmentObject private var appState: AppState
    @Environment(\.dismiss) private var dismiss

    @State private var name = ""
    @State private var isCreating = false

    var body: some View {
        NavigationStack {
            Form {
                Section("New group") {
                    TextField("Group name", text: $name)
                        .textInputAutocapitalization(.words)
                        .submitLabel(.done)
                        .onSubmit { Task { await create() } }
                }

                if let error = appState.errorMessage {
                    Section {
                        Text(error).foregroundStyle(.red)
                    }
                }
            }
            .navigationTitle("New Group")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") { dismiss() }
                        .disabled(isCreating)
                }
                ToolbarItem(placement: .confirmationAction) {
                    Button {
                        Task { await create() }
                    } label: {
                        if isCreating {
                            ProgressView()
                        } else {
                            Text("Create")
                        }
                    }
                    .disabled(trimmedName.isEmpty || isCreating)
                }
            }
        }
    }

    private var trimmedName: String {
        name.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    private func create() async {
        guard !trimmedName.isEmpty else { return }
        isCreating = true
        defer { isCreating = false }
        if await appState.createGroup(name: trimmedName) != nil {
            dismiss()
        }
    }
}

#Preview {
    NewGroupView()
        .environmentObject(AppState())
}
