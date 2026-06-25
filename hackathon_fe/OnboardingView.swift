//
//  OnboardingView.swift
//  hackathon_fe
//
//  First-launch profile creation. The prototype backend has no auth, so we
//  simply create a User and remember its id locally.
//

import SwiftUI

struct OnboardingView: View {
    @EnvironmentObject private var appState: AppState

    @State private var name = ""
    @State private var trustedContact = ""
    @State private var baseURLString: String = AppConfig.baseURL.absoluteString
    @State private var isSubmitting = false

    var body: some View {
        NavigationStack {
            Form {
                Section {
                    TextField("Your name", text: $name)
                        .textContentType(.name)
                    TextField("Trusted contact (optional)", text: $trustedContact)
                        .keyboardType(.phonePad)
                        .textContentType(.telephoneNumber)
                } header: {
                    Text("Welcome to NightWatch")
                } footer: {
                    Text("This creates your profile so you can form groups and join others by scanning a QR code.")
                }

                Section {
                    TextField(AppConfig.defaultBaseURL.absoluteString, text: $baseURLString)
                        .keyboardType(.URL)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                        .textContentType(.URL)
                        .submitLabel(.done)
                        .onSubmit(saveBackend)
                } header: {
                    Text("Backend")
                } footer: {
                    Text(backendFooter)
                }

                if let error = appState.errorMessage {
                    Section {
                        Text(error).foregroundStyle(.red)
                    }
                }
            }
            .navigationTitle("Get Started")
            .toolbar {
                ToolbarItem(placement: .confirmationAction) {
                    Button("Continue") {
                        Task {
                            isSubmitting = true
                            saveBackend()
                            await appState.completeOnboarding(
                                name: trimmedName,
                                trustedContact: trimmedContact.isEmpty ? nil : trimmedContact
                            )
                            isSubmitting = false
                        }
                    }
                    .disabled(trimmedName.isEmpty || isSubmitting)
                }
            }
            .interactiveDismissDisabled(true)
        }
    }

    private var trimmedName: String {
        name.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    private var trimmedContact: String {
        trustedContact.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    private var backendFooter: String {
        if normalizedURL == nil {
            return "Enter the server address, e.g. \(AppConfig.defaultBaseURL.absoluteString)"
        }
        return "The address of the NightWatch server your app talks to. You can change it later in Settings."
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

    private func saveBackend() {
        if let url = normalizedURL { AppConfig.baseURL = url }
    }
}
