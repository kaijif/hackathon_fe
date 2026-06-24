//
//  JoinGroupView.swift
//  hackathon_fe
//
//  Confirmation sheet shown after scanning an invite QR code. Loads the group
//  details, then joins it via the API.
//

import SwiftUI

struct JoinGroupView: View {
    let groupId: String

    @EnvironmentObject private var appState: AppState
    @Environment(\.dismiss) private var dismiss

    @State private var group: Group?
    @State private var phase: Phase = .loading
    @State private var errorText: String?

    private enum Phase { case loading, ready, joining, success, error }

    var body: some View {
        NavigationStack {
            VStack(spacing: 20) {
                switch phase {
                case .loading:
                    ProgressView("Loading group\u{2026}")

                case .ready, .joining:
                    Image(systemName: "person.3.fill")
                        .font(.system(size: 52))
                        .foregroundStyle(.tint)
                    Text(group?.name ?? "Group")
                        .font(.title2).bold()
                        .multilineTextAlignment(.center)
                    Text("Do you want to join this group?")
                        .foregroundStyle(.secondary)
                    joinButton

                case .success:
                    Image(systemName: "checkmark.circle.fill")
                        .font(.system(size: 52))
                        .foregroundStyle(.green)
                    Text("You joined \(group?.name ?? "the group")!")
                        .font(.title3).bold()
                        .multilineTextAlignment(.center)

                case .error:
                    Image(systemName: "exclamationmark.triangle.fill")
                        .font(.system(size: 52))
                        .foregroundStyle(.orange)
                    Text(errorText ?? "Something went wrong.")
                        .multilineTextAlignment(.center)
                        .foregroundStyle(.secondary)
                    Button("Try Again") { Task { await load() } }
                        .buttonStyle(.bordered)
                }
            }
            .padding()
            .frame(maxWidth: .infinity, maxHeight: .infinity)
            .navigationTitle("Join Group")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button(phase == .success ? "Done" : "Cancel") { dismiss() }
                }
            }
            .task { await load() }
        }
    }

    private var joinButton: some View {
        Button {
            Task { await join() }
        } label: {
            HStack {
                Spacer()
                if phase == .joining {
                    ProgressView().tint(.white)
                } else {
                    Text("Join Group").bold()
                }
                Spacer()
            }
            .padding()
            .background(Color.accentColor)
            .foregroundStyle(.white)
            .clipShape(RoundedRectangle(cornerRadius: 12))
        }
        .disabled(phase == .joining)
        .padding(.top, 8)
    }

    private func load() async {
        phase = .loading
        errorText = nil
        do {
            group = try await appState.fetchGroup(id: groupId)
            phase = .ready
        } catch {
            errorText = "Couldn't find that group. It may have been deleted.\n\n\(error.localizedDescription)"
            phase = .error
        }
    }

    private func join() async {
        phase = .joining
        do {
            try await appState.join(groupId: groupId)
            phase = .success
        } catch {
            errorText = error.localizedDescription
            phase = .error
        }
    }
}
