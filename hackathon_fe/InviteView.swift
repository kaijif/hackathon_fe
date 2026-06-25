//
//  InviteView.swift
//  hackathon_fe
//
//  Invite sheet opened from a group's "Invite" button. Shows the scannable
//  join QR code (and shareable link) for that one group. Creating a group lives
//  in NewGroupView; this view is purely "scan to join".
//

import SwiftUI

struct InviteView: View {
    let group: Group

    @Environment(\.dismiss) private var dismiss

    private var joinURLString: String {
        DeepLink.joinURLString(groupId: group.id)
    }

    var body: some View {
        NavigationStack {
            Form {
                Section("Scan to join \u{201C}\(group.name)\u{201D}") {
                    VStack(spacing: 12) {
                        QRCodeView(content: joinURLString)
                            .frame(maxWidth: .infinity)

                        Text(joinURLString)
                            .font(.caption)
                            .foregroundStyle(.secondary)
                            .multilineTextAlignment(.center)
                            .textSelection(.enabled)

                        ShareLink(item: joinURLString) {
                            Label("Share invite link", systemImage: "square.and.arrow.up")
                        }
                    }
                    .padding(.vertical, 8)
                }
            }
            .navigationTitle("Invite")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .confirmationAction) {
                    Button("Done") { dismiss() }
                }
            }
        }
    }
}

#Preview {
    InviteView(group: Group(id: "g1", name: "Night Crew", active: nil, currNightId: nil, createdAt: nil, updatedAt: nil))
}
