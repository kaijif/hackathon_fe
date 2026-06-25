//
//  GuardianView.swift
//  hackathon_fe
//
//  Live monitoring screen for an active NightWatch night. The map fills the
//  screen with floating Liquid Glass controls; live member status lives in the
//  group detail sheet, opened with the people button.
//

import SwiftUI

struct GuardianView: View {
    let nightId: String
    let navigationTitle: String
    let group: Group?

    @EnvironmentObject private var appState: AppState
    @Environment(\.dismiss) private var dismiss
    @StateObject private var store: NightStore

    @State private var showGroupDetail = false

    init(nightId: String,
         currentUserId: String? = nil,
         navigationTitle: String = "Guardian",
         group: Group? = nil) {
        self.nightId = nightId
        self.navigationTitle = navigationTitle
        self.group = group
        _store = StateObject(wrappedValue: NightStore(nightId: nightId, currentUserId: currentUserId))
    }

    var body: some View {
        ZStack {
            NightMapView(center: store.centerCoordinate,
                         rangeMeters: store.maxRangeMeters,
                         rows: store.rows)
                .ignoresSafeArea()

            VStack(spacing: 0) {
                topBar
                Spacer()
                bottomControls
            }
            .padding()
        }
        .toolbar(.hidden, for: .navigationBar)
        .task {
            appState.track(nightId: nightId)
            store.startPolling()
        }
        .onDisappear {
            store.stopPolling()
            appState.track(nightId: nil)
        }
        .sheet(isPresented: $showGroupDetail) { groupDetailSheet }
    }

    // MARK: - Floating glass overlays

    private var topBar: some View {
        HStack(spacing: 10) {
            Button { dismiss() } label: {
                Image(systemName: "chevron.left")
                    .font(.headline)
                    .frame(width: 24, height: 24)
            }
            .buttonStyle(.glass)
            .accessibilityLabel("Back")

            statusChip

            Spacer()

            if group != nil {
                Button { showGroupDetail = true } label: {
                    Image(systemName: "person.2")
                        .font(.headline)
                        .frame(width: 24, height: 24)
                }
                .buttonStyle(.glass)
                .accessibilityLabel("Group details")
            }
        }
    }

    private var statusChip: some View {
        SwiftUI.Group {
            if store.night?.status == .ended {
                Label("Night ended", systemImage: "moon.zzz.fill")
            } else {
                Text(navigationTitle)
            }
        }
        .font(.headline)
        .padding(.horizontal, 16)
        .padding(.vertical, 10)
        .glassEffect()
    }

    private var bottomControls: some View {
        VStack(spacing: 10) {
            if let errorMessage = store.errorMessage {
                Label(errorMessage, systemImage: "exclamationmark.triangle.fill")
                    .font(.caption)
                    .foregroundStyle(.orange)
                    .padding(.horizontal, 14)
                    .padding(.vertical, 8)
                    .glassEffect()
            }

            Button {
                Task { await store.checkIn() }
            } label: {
                Label("I'm OK", systemImage: "checkmark.circle.fill")
                    .font(.headline)
                    .frame(maxWidth: .infinity, minHeight: 50)
            }
            .buttonStyle(.glassProminent)
            .tint(.green)

            Button(role: .destructive) {
                Task {
                    if await store.endNight() != nil {
                        dismiss()
                    }
                }
            } label: {
                Label("End Night", systemImage: "stop.circle.fill")
                    .font(.headline)
                    .frame(maxWidth: .infinity, minHeight: 50)
            }
            .buttonStyle(.glass)
            .tint(.red)
        }
    }

    // MARK: - Group detail sheet (live members, invite & leave)

    @ViewBuilder
    private var groupDetailSheet: some View {
        if let group {
            NavigationStack {
                GroupDetailView(group: group, onLeave: {
                    showGroupDetail = false
                    dismiss()
                }, liveRows: store.rows)
                .toolbar {
                    ToolbarItem(placement: .topBarTrailing) {
                        Button { showGroupDetail = false } label: {
                            Image(systemName: "xmark")
                        }
                        .accessibilityLabel("Close")
                    }
                }
            }
        }
    }
}

#Preview {
    NavigationStack {
        GuardianView(nightId: "preview", currentUserId: nil)
    }
    .environmentObject(AppState())
}
