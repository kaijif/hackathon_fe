//
//  SettingsView.swift
//  hackathon_fe
//
//  App settings. Currently only the backend base URL so the prototype can be
//  pointed at localhost or a deployed server without rebuilding.
//

import SwiftUI

struct SettingsView: View {
    @Environment(\.dismiss) private var dismiss

    @State private var baseURLString: String = AppConfig.baseURL.absoluteString

    var body: some View {
        Form {
            Section {
                TextField(AppConfig.defaultBaseURL.absoluteString, text: $baseURLString)
                    .keyboardType(.URL)
                    .textInputAutocapitalization(.never)
                    .autocorrectionDisabled()
                    .textContentType(.URL)
                    .submitLabel(.done)
                    .onSubmit(save)
            } header: {
                Text("Backend")
            } footer: {
                Text(footerText)
            }

            Section {
                Button("Reset to Default") {
                    baseURLString = AppConfig.defaultBaseURL.absoluteString
                }
                .disabled(baseURLString == AppConfig.defaultBaseURL.absoluteString)
            }
        }
        .navigationTitle("Settings")
        .navigationBarTitleDisplayMode(.inline)
        .toolbar {
            ToolbarItem(placement: .cancellationAction) {
                Button("Cancel") { dismiss() }
            }
            ToolbarItem(placement: .confirmationAction) {
                Button("Save", action: save)
                    .disabled(normalizedURL == nil)
            }
        }
    }

    private var footerText: String {
        if normalizedURL == nil {
            return "Enter a valid URL, e.g. \(AppConfig.defaultBaseURL.absoluteString)"
        }
        return "The base URL all API requests are sent to. Takes effect immediately."
    }

    /// A trimmed, validated URL: must have an http/https scheme and a host.
    private var normalizedURL: URL? {
        let trimmed = baseURLString.trimmingCharacters(in: .whitespacesAndNewlines)
        guard let url = URL(string: trimmed),
              let scheme = url.scheme?.lowercased(),
              scheme == "http" || scheme == "https",
              url.host?.isEmpty == false else {
            return nil
        }
        return url
    }

    private func save() {
        guard let url = normalizedURL else { return }
        AppConfig.baseURL = url
        dismiss()
    }
}

#Preview {
    NavigationStack {
        SettingsView()
    }
}
