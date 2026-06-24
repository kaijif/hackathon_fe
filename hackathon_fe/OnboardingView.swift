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
}
