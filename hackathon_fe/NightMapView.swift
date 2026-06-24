//
//  NightMapView.swift
//  hackathon_fe
//
//  MapKit view showing the safe zone and live participant locations.
//

import SwiftUI
import MapKit
import CoreLocation

struct NightMapView: View {
    let center: CLLocationCoordinate2D?
    let rangeMeters: Double?
    let rows: [NightStore.Row]

    @State private var position: MapCameraPosition

    init(center: CLLocationCoordinate2D?, rangeMeters: Double?, rows: [NightStore.Row]) {
        self.center = center
        self.rangeMeters = rangeMeters
        self.rows = rows
        _position = State(initialValue: Self.initialPosition(center: center, rangeMeters: rangeMeters, rows: rows))
    }

    var body: some View {
        Map(position: $position) {
            if let center {
                Marker("Start", systemImage: "house.fill", coordinate: center)
                    .tint(.blue)

                if let rangeMeters, rangeMeters > 0 {
                    MapCircle(center: center, radius: rangeMeters)
                        .foregroundStyle(.blue.opacity(0.12))
                        .stroke(.blue.opacity(0.6), lineWidth: 1)
                }
            }

            ForEach(rows) { row in
                if let coordinate = row.coordinate {
                    Marker(row.name, coordinate: coordinate)
                        .tint(markerColor(row.status))
                }
            }
        }
        .mapStyle(.standard)
        .clipShape(RoundedRectangle(cornerRadius: 16))
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

    private func markerColor(_ status: ParticipantStatusKind) -> Color {
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
    NightMapView(
        center: CLLocationCoordinate2D(latitude: 37.33, longitude: -122.03),
        rangeMeters: 800,
        rows: []
    )
}
