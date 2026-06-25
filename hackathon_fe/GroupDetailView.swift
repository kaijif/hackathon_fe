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
    /// Live participant rows from the active night's monitor; when non-empty the
    /// members list shows live status instead of the plain roster.
    var liveRows: [NightStore.Row] = []
    /// When provided, a night controls row (I'm OK / End Night) is shown at the
    /// top of the menu.
    var onCheckIn: (() async -> Void)? = nil
    var onEndNight: (() async -> Void)? = nil
    /// When provided, tapping a live member row selects that member's map pin.
    var selectedUserId: Binding<String?>? = nil

    @EnvironmentObject private var appState: AppState

    @State private var members: [Member] = []
    @State private var isLoading = false
    @State private var showInvite = false
    @State private var showLeaveConfirmation = false
    @State private var isLeaving = false

    var body: some View {
        SwiftUI.Group {
            if let selectedRow {
                memberDetail(selectedRow)
            } else {
                mainContent
            }
        }
        .navigationBarTitleDisplayMode(.inline)
        .task { await load() }
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

    /// Member roster + night controls — the default content of the sheet.
    private var mainContent: some View {
        VStack(spacing: 0) {
            if onCheckIn != nil || onEndNight != nil {
                nightControls
                    .padding(.horizontal)
                    .padding(.top, 16)
                    .padding(.bottom, 28)
            }

            List {
                membersSection
                leaveSection
                errorSection
            }
            .scrollContentBackground(.hidden)
            .refreshable { await load() }
        }
        .navigationTitle("Actions")
        .toolbar {
            ToolbarItem(placement: .topBarTrailing) {
                Button {
                    showInvite = true
                } label: {
                    Image(systemName: "plus")
                }
                .accessibilityLabel("Invite with QR code")
            }
        }
    }

    /// The currently selected member's live row, if any.
    private var selectedRow: NightStore.Row? {
        guard let id = selectedUserId?.wrappedValue else { return nil }
        return liveRows.first { $0.id == id }
    }

    /// Detail view for a single member, shown in place of the roster when a
    /// member is selected. The X button returns to the roster.
    private func memberDetail(_ row: NightStore.Row) -> some View {
        List {
            Section {
                VStack(alignment: .leading, spacing: 6) {
                    Text(row.name + (row.isCurrentUser ? " (You)" : ""))
                        .font(.title2.bold())
                    Text(lastSeenText(row.reportedAt))
                        .font(.subheadline.weight(.medium))
                        .foregroundStyle(lastSeenColor(row.reportedAt))
                }
                .padding(.vertical, 4)
            }

            Section("Status") {
                Label(row.status.label, systemImage: icon(for: row.status))
                    .foregroundStyle(color(for: row.status))
                if let detail = row.detail, !detail.isEmpty {
                    Text(detail)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
            }

            Section("Battery") {
                Label(row.batteryLevel.map { "\($0)%" } ?? "Unknown",
                      systemImage: batteryIcon(row.batteryLevel))
            }

            Section("Last check-in") {
                Label(checkInAbsolute(row.reportedAt) ?? "No check-in yet",
                      systemImage: "clock")
            }

            if let distance = row.distanceM {
                Section("Distance from start") {
                    Label(distanceText(distance), systemImage: "ruler")
                }
            }
        }
        .scrollContentBackground(.hidden)
        .navigationTitle(row.name)
        .toolbar {
            ToolbarItem(placement: .topBarLeading) {
                Button {
                    selectedUserId?.wrappedValue = nil
                } label: {
                    Image(systemName: "xmark")
                }
                .accessibilityLabel("Back to members")
            }
        }
    }

    private var nightControls: some View {
        HStack(spacing: 10) {
            Button {
                Task { await onCheckIn?() }
            } label: {
                Label("I'm OK", systemImage: "checkmark.circle.fill")
                    .font(.headline)
                    .frame(maxWidth: .infinity, minHeight: 50)
            }
            .buttonStyle(.glassProminent)
            .tint(.green)

            Button(role: .destructive) {
                Task { await onEndNight?() }
            } label: {
                Label("End Night", systemImage: "stop.circle.fill")
                    .font(.headline)
                    .frame(maxWidth: .infinity, minHeight: 50)
            }
            .buttonStyle(.glass)
            .tint(.red)
        }
    }

    private var membersSection: some View {
        Section("Members (\(memberCount))") {
            if !liveRows.isEmpty {
                ForEach(liveRows) { row in
                    if selectedUserId != nil {
                        Button {
                            toggleSelection(row.id)
                        } label: {
                            rowView(row)
                        }
                        .buttonStyle(.plain)
                        .listRowBackground(rowBackground(for: row.id))
                    } else {
                        rowView(row)
                    }
                }
            } else if members.isEmpty && isLoading {
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

    private func toggleSelection(_ userId: String) {
        guard let selectedUserId else { return }
        selectedUserId.wrappedValue = selectedUserId.wrappedValue == userId ? nil : userId
    }

    private func rowBackground(for userId: String) -> Color {
        selectedUserId?.wrappedValue == userId ? Color.accentColor.opacity(0.15) : .clear
    }

    private var memberCount: Int {
        liveRows.isEmpty ? members.count : liveRows.count
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

    private func batteryIcon(_ level: Int?) -> String {
        guard let level else { return "battery.0percent" }
        switch level {
        case ..<13: return "battery.0percent"
        case ..<38: return "battery.25percent"
        case ..<63: return "battery.50percent"
        case ..<88: return "battery.75percent"
        default: return "battery.100percent"
        }
    }

    /// Parses an ISO-8601 timestamp (with or without fractional seconds).
    private func isoDate(_ iso: String?) -> Date? {
        guard let iso else { return nil }
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        if let date = formatter.date(from: iso) { return date }
        formatter.formatOptions = [.withInternetDateTime]
        return formatter.date(from: iso)
    }

    /// "Last seen 2 minutes ago" (or a placeholder when we've never heard from
    /// the member during this night).
    private func lastSeenText(_ iso: String?) -> String {
        guard let date = isoDate(iso) else { return "No check-in yet" }
        let formatter = RelativeDateTimeFormatter()
        formatter.unitsStyle = .full
        return "Last seen " + formatter.localizedString(for: date, relativeTo: Date())
    }

    /// Green when last seen under 2 minutes ago, red beyond that, grey if unknown.
    private func lastSeenColor(_ iso: String?) -> Color {
        guard let date = isoDate(iso) else { return .gray }
        return Date().timeIntervalSince(date) < 120 ? .green : .red
    }

    /// Absolute formatted check-in time, e.g. "Jun 24, 2026 at 6:53 PM".
    private func checkInAbsolute(_ iso: String?) -> String? {
        guard let date = isoDate(iso) else { return nil }
        return date.formatted(date: .abbreviated, time: .shortened)
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
