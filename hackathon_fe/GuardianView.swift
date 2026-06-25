//
//  GuardianView.swift
//  hackathon_fe
//
//  Live monitoring screen for an active NightWatch night.
//

import SwiftUI

struct GuardianView: View {
    let nightId: String
    let navigationTitle: String
    let onShowGroup: (() -> Void)?

    @EnvironmentObject private var appState: AppState
    @Environment(\.dismiss) private var dismiss
    @StateObject private var store: NightStore

    init(nightId: String,
         currentUserId: String? = nil,
         navigationTitle: String = "Guardian",
         onShowGroup: (() -> Void)? = nil) {
        self.nightId = nightId
        self.navigationTitle = navigationTitle
        self.onShowGroup = onShowGroup
        _store = StateObject(wrappedValue: NightStore(nightId: nightId, currentUserId: currentUserId))
    }

    var body: some View {
        VStack(spacing: 0) {
            NightMapView(center: store.centerCoordinate, rangeMeters: store.maxRangeMeters, rows: store.rows)
                .frame(height: 300)

            if store.night?.status == .ended {
                Label("This night has ended", systemImage: "moon.zzz.fill")
                    .font(.subheadline.bold())
                    .foregroundStyle(.secondary)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .padding()
                    .background(.thinMaterial)
            }

            List {
                if store.rows.isEmpty {
                    ContentUnavailableView {
                        Label("No Locations Yet", systemImage: "location.slash")
                    } description: {
                        Text("Member status will appear here as the night updates.")
                    }
                } else {
                    Section("Members") {
                        ForEach(store.rows) { row in
                            rowView(row)
                        }
                    }
                }

                if let errorMessage = store.errorMessage {
                    Section {
                        Label(errorMessage, systemImage: "exclamationmark.triangle.fill")
                            .font(.caption)
                            .foregroundStyle(.orange)
                    }
                }
            }

            controls
                .padding()
                .background(.bar)
        }
        .navigationTitle(navigationTitle)
        .navigationBarTitleDisplayMode(.inline)
        .toolbar {
            if onShowGroup != nil {
                ToolbarItem(placement: .topBarTrailing) {
                    Button { onShowGroup?() } label: {
                        Image(systemName: "person.2")
                    }
                    .accessibilityLabel("Group details")
                }
            }
        }
        .task {
            appState.track(nightId: nightId)
            store.startPolling()
        }
        .onDisappear {
            store.stopPolling()
            appState.track(nightId: nil)
        }
    }

    private var controls: some View {
        VStack(spacing: 10) {
            Button {
                Task { await store.checkIn() }
            } label: {
                Label("I'm OK", systemImage: "checkmark.circle.fill")
                    .frame(maxWidth: .infinity)
            }
            .buttonStyle(.borderedProminent)

            Button(role: .destructive) {
                Task {
                    if await store.endNight() != nil {
                        dismiss()
                    }
                }
            } label: {
                Label("End Night", systemImage: "stop.circle.fill")
                    .frame(maxWidth: .infinity)
            }
            .buttonStyle(.bordered)

            Text(store.isCurrentUserAdmin ? "End the night when everyone is safe." : "Admins typically end the night.")
                .font(.caption2)
                .foregroundStyle(.secondary)
        }
    }

    private func rowView(_ row: NightStore.Row) -> some View {
        HStack(alignment: .top, spacing: 12) {
            Image(systemName: icon(for: row.status))
                .font(.title3)
                .foregroundStyle(color(for: row.status))
                .frame(width: 28)

            VStack(alignment: .leading, spacing: 6) {
                HStack(alignment: .firstTextBaseline) {
                    Text(row.name + (row.isCurrentUser ? " (You)" : ""))
                        .font(.headline)
                    Spacer()
                    Text(row.status.label)
                        .font(.caption.bold())
                        .padding(.horizontal, 8)
                        .padding(.vertical, 4)
                        .foregroundStyle(color(for: row.status))
                        .background(color(for: row.status).opacity(0.14))
                        .clipShape(Capsule())
                }

                HStack(spacing: 12) {
                    if let batteryLevel = row.batteryLevel {
                        Label("\(batteryLevel)%", systemImage: "battery.75percent")
                    }
                    if let distance = row.distanceM {
                        Label(distanceText(distance), systemImage: "ruler")
                    }
                }
                .font(.caption)
                .foregroundStyle(.secondary)

                if let detail = row.detail, !detail.isEmpty {
                    Text(detail)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
            }
        }
        .padding(.vertical, 4)
    }

    private func icon(for status: ParticipantStatusKind) -> String {
        switch status {
        case .ok:
            return "checkmark.circle.fill"
        case .lowBattery, .outOfRange, .missing:
            return "exclamationmark.triangle.fill"
        case .unknown:
            return "questionmark.circle"
        }
    }

    private func distanceText(_ meters: Double) -> String {
        if meters < 1_000 {
            return "\(Int(meters.rounded()))m"
        }
        return String(format: "%.1f km", meters / 1_000)
    }

    private func color(for status: ParticipantStatusKind) -> Color {
        switch status {
        case .ok:
            return .green
        case .lowBattery:
            return .orange
        case .outOfRange, .missing:
            return .red
        case .unknown:
            return .gray
        }
    }
}

#Preview {
    NavigationStack {
        GuardianView(nightId: "preview", currentUserId: nil)
    }
    .environmentObject(AppState())
}
