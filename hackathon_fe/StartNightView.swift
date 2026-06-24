//
//  StartNightView.swift
//  hackathon_fe
//
//  Safety configuration shown before starting an active night.
//

import SwiftUI
internal import CoreLocation

struct StartNightView: View {
    let group: Group

    @EnvironmentObject private var appState: AppState
    @Environment(\.dismiss) private var dismiss

    @State private var maxRangeM: Int = 1000
    @State private var timeLimitHours: Int = 8
    @State private var checkInEveryMin: Int = 15
    @State private var checkInLimitMin: Int = 60
    @State private var lowBatteryThreshold: Int = 20
    @State private var isSubmitting: Bool = false

    var body: some View {
        Form {
            Section {
                Stepper(value: $maxRangeM, in: 100...10000, step: 100) {
                    Text("Max range: \(formattedDistance(maxRangeM))")
                }
            } header: {
                Text("Safe zone")
            } footer: {
                Text("Members beyond this distance from the start location trigger an alert.")
            }

            Section {
                Picker("Night duration", selection: $timeLimitHours) {
                    ForEach(1...12, id: \.self) { hours in
                        Text("\(hours) h").tag(hours)
                    }
                }
            } header: {
                Text("Duration")
            } footer: {
                Text("The night automatically ends after this time.")
            }

            Section {
                Picker("Ask for check-in", selection: $checkInEveryMin) {
                    ForEach([5, 10, 15, 30, 60], id: \.self) { minutes in
                        Text("Every \(minutes) min").tag(minutes)
                    }
                }

                Picker("No response", selection: $checkInLimitMin) {
                    ForEach([15, 30, 45, 60, 90, 120], id: \.self) { minutes in
                        Text("Mark missing after \(minutes) min").tag(minutes)
                    }
                }
            } header: {
                Text("Check-ins")
            } footer: {
                Text("Controls how often each member is asked 'are you OK?' and when no-response escalates.")
            }

            Section {
                Stepper(value: $lowBatteryThreshold, in: 5...50, step: 5) {
                    Text("Low battery below: \(lowBatteryThreshold)%")
                }
            } header: {
                Text("Battery")
            } footer: {
                Text("Alerts the group when a member drops below this level.")
            }

            if appState.locationManager.lastCoordinate == nil {
                Section {
                    Text("Waiting for your location… the night will center on where you are now.")
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
        .navigationTitle("Start Night")
        .navigationBarTitleDisplayMode(.inline)
        .toolbar {
            ToolbarItem(placement: .cancellationAction) {
                Button("Cancel") { dismiss() }
                    .disabled(isSubmitting)
            }

            ToolbarItem(placement: .confirmationAction) {
                Button {
                    Task { await startNight() }
                } label: {
                    if isSubmitting {
                        ProgressView()
                    } else {
                        Text("Start")
                    }
                }
                .disabled(isSubmitting)
            }
        }
        .task {
            appState.locationManager.requestAuthorization()
            appState.locationManager.startTracking()
        }
    }

    private func formattedDistance(_ meters: Int) -> String {
        if meters >= 1000 {
            return String(format: "%.1f km", Double(meters) / 1000)
        }
        return "\(meters) m"
    }

    private func startNight() async {
        isSubmitting = true
        defer { isSubmitting = false }

        let coord = appState.locationManager.lastCoordinate
        let config = CreateNightRequest(
            agentId: nil,
            center: coord.map { Coords(lat: $0.latitude, lng: $0.longitude) },
            timeLimitMin: timeLimitHours * 60,
            checkInLimitMin: checkInLimitMin,
            checkInEveryMin: checkInEveryMin,
            maxRangeM: maxRangeM,
            lowBatteryThreshold: lowBatteryThreshold
        )

        if await appState.startNight(forGroup: group.id, config: config) != nil {
            dismiss()
        }
    }
}

#Preview {
    NavigationStack {
        StartNightView(group: Group(id: "g1", name: "Night Crew", active: nil, currNightId: nil, createdAt: nil, updatedAt: nil))
            .environmentObject(AppState())
    }
}
