//
//  GuardianView.swift
//  hackathon_fe
//
//  Live monitoring screen for an active NightWatch night. The map fills the
//  screen; a Find My–style sheet holds the night controls and live member
//  status, sliding up and down over the map.
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

    /// Height of the Find My–style member sheet at its smallest (peek) detent —
    /// just tall enough to show the grabber, title bar and the I'm OK / End Night
    /// controls, keeping the members list hidden until you drag up.
    private let sheetPeek: CGFloat = 140

    init(nightId: String,
         currentUserId: String? = nil,
         navigationTitle: String = "Guardian",
         group: Group? = nil) {
        self.nightId = nightId
        self.navigationTitle = navigationTitle
        self.group = group
        _store = StateObject(wrappedValue: NightStore(nightId: nightId, currentUserId: currentUserId))
        _showGroupDetail = State(initialValue: group != nil)
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
        .sheet(isPresented: $showGroupDetail) { groupSheet }
    }

    // MARK: - Floating glass overlay

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

    // MARK: - Find My–style sheet (night controls, live members, invite & leave)

    @ViewBuilder
    private var groupSheet: some View {
        if let group {
            NavigationStack {
                GroupDetailView(
                    group: group,
                    onLeave: {
                        showGroupDetail = false
                        dismiss()
                    },
                    liveRows: store.rows,
                    onCheckIn: { await store.checkIn() },
                    onEndNight: {
                        if await store.endNight() != nil {
                            showGroupDetail = false
                            dismiss()
                        }
                    }
                )
            }
            .presentationDetents([.height(sheetPeek), .medium, .large])
            .presentationBackgroundInteraction(.enabled(upThrough: .medium))
            .presentationDragIndicator(.visible)
            .interactiveDismissDisabled()
        }
    }
}

#Preview {
    NavigationStack {
        GuardianView(nightId: "preview", currentUserId: nil)
    }
    .environmentObject(AppState())
}
