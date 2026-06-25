//
//  NightMapView.swift
//  hackathon_fe
//
//  MapKit view showing the safe zone and live participant locations.
//

import SwiftUI
import MapKit
internal import CoreLocation

struct NightMapView: View {
    let center: CLLocationCoordinate2D?
    let rangeMeters: Double?
    let rows: [NightStore.Row]
    let cornerRadius: CGFloat
    @Binding var selectedUserId: String?

    @State private var position: MapCameraPosition

    init(center: CLLocationCoordinate2D?,
         rangeMeters: Double?,
         rows: [NightStore.Row],
         cornerRadius: CGFloat = 16,
         selectedUserId: Binding<String?> = .constant(nil)) {
        self.center = center
        self.rangeMeters = rangeMeters
        self.rows = rows
        self.cornerRadius = cornerRadius
        _selectedUserId = selectedUserId
        _position = State(initialValue: Self.initialPosition(center: center, rangeMeters: rangeMeters, rows: rows))
    }

    var body: some View {
        Map(position: $position, selection: $selectedUserId) {
            if let circleCenter, let rangeMeters, rangeMeters > 0 {
                MapCircle(center: circleCenter, radius: rangeMeters)
                    .foregroundStyle(.blue.opacity(0.12))
                    .stroke(.blue.opacity(0.6), lineWidth: 1)
            }

            ForEach(rows) { row in
                if let coordinate = row.coordinate {
                    Marker(row.name, coordinate: coordinate)
                        .tint(row.isAdmin ? .green : .blue)
                        .tag(row.id)
                }
            }
        }
        .mapStyle(.standard)
        .clipShape(RoundedRectangle(cornerRadius: cornerRadius))
        .onChange(of: selectedUserId) { _, newValue in
            focus(on: newValue)
        }
    }

    /// The safe-zone circle follows the group admin's live location, falling back
    /// to the night's configured center until the admin reports a location.
    private var circleCenter: CLLocationCoordinate2D? {
        rows.first(where: { $0.isAdmin })?.coordinate ?? center
    }

    /// Recenters the camera on the selected member's pin so it comes into view.
    private func focus(on userId: String?) {
        guard let userId,
              let coordinate = rows.first(where: { $0.id == userId })?.coordinate else {
            return
        }
        let meters = max(rangeMeters ?? 400, 300)
        withAnimation {
            position = .region(MKCoordinateRegion(
                center: coordinate,
                latitudinalMeters: meters,
                longitudinalMeters: meters
            ))
        }
    }

    private static func initialPosition(
        center: CLLocationCoordinate2D?,
        rangeMeters: Double?,
        rows: [NightStore.Row]
    ) -> MapCameraPosition {
        if let center {
            let meters = max((rangeMeters ?? 500) * 3, 500)
            return .region(MKCoordinateRegion(
                center: center,
                latitudinalMeters: meters,
                longitudinalMeters: meters
            ))
        }

        if let coordinate = rows.compactMap(\.coordinate).first {
            return .region(MKCoordinateRegion(
                center: coordinate,
                latitudinalMeters: 1_500,
                longitudinalMeters: 1_500
            ))
        }

        return .automatic
    }
}

#Preview {
    NightMapView(
        center: CLLocationCoordinate2D(latitude: 37.33, longitude: -122.03),
        rangeMeters: 800,
        rows: []
    )
}
