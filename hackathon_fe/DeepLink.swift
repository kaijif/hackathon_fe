//
//  DeepLink.swift
//  hackathon_fe
//
//  Builds and parses the custom-scheme links encoded in invite QR codes.
//  A join link looks like: hackathon_be://join?groupId=<groupId>
//

import Foundation

nonisolated enum DeepLink {
    /// Custom URL scheme registered in Info.plist. Matches the app name.
    static let scheme = "hackathon_be"
    /// Host that identifies a "join a group" action.
    static let joinHost = "join"

    private static let groupIdAllowed: CharacterSet =
        CharacterSet.alphanumerics.union(CharacterSet(charactersIn: "-_"))

    /// The string encoded into an invite QR code.
    static func joinURLString(groupId: String) -> String {
        let encoded = groupId.addingPercentEncoding(withAllowedCharacters: groupIdAllowed) ?? groupId
        return "\(scheme)://\(joinHost)?groupId=\(encoded)"
    }

    /// Extracts the `groupId` from an incoming join link, tolerating the
    /// non-RFC underscore in the scheme by falling back to manual parsing.
    static func groupId(fromJoinURL url: URL) -> String? {
        if let components = URLComponents(url: url, resolvingAgainstBaseURL: false),
           let value = components.queryItems?.first(where: { $0.name == "groupId" })?.value,
           !value.isEmpty {
            return value
        }

        let raw = url.absoluteString
        guard let range = raw.range(of: "groupId=") else { return nil }
        let tail = raw[range.upperBound...]
        let value = String(tail.prefix(while: { $0 != "&" }))
        guard !value.isEmpty else { return nil }
        return value.removingPercentEncoding ?? value
    }
}
