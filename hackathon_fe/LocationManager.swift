//
//  LocationManager.swift
//  hackathon_fe
//
//  Captures the device's location (foreground + background) and battery level
//  and reports them to the server frequently so the group's active nights stay
//  up to date. Background updates require the "location" UIBackgroundMode and
//  "Always" authorization (see Info.plist).
//

import Foundation
import CoreLocation
import UIKit

@MainActor
final class LocationManager: NSObject, ObservableObject {
    @Published private(set) var authorizationStatus: CLAuthorizationStatus
    @Published private(set) var lastCoordinate: CLLocationCoordinate2D?
    @Published private(set) var isTracking = false

    /// Supplies the signed-in user id; set by AppState once a user exists.
    var userIdProvider: () -> String? = { nil }
    /// Supplies the active night id (if any) so fixes are also reported into the
    /// night immediately, in addition to the server-side propagation the backend
    /// does for `PUT /users/{id}/location`.
    var activeNightIdProvider: () -> String? = { nil }

    private let manager = CLLocationManager()
    private let api: APIClient
    /// Minimum seconds between server-side location pushes (throttle).
    private let minUploadInterval: TimeInterval = 15
    private var lastUploadAt: Date?
    private var uploadTask: Task<Void, Never>?

    init(api: APIClient = APIClient()) {
        self.api = api
        self.authorizationStatus = .notDetermined
        super.init()
        authorizationStatus = manager.authorizationStatus
        manager.delegate = self
        manager.desiredAccuracy = kCLLocationAccuracyBest
        manager.distanceFilter = 10
        UIDevice.current.isBatteryMonitoringEnabled = true
    }

    // MARK: - Authorization & tracking

    /// Requests When-In-Use first; once granted we escalate to Always so the app
    /// can keep reporting in the background during a night.
    func requestAuthorization() {
        switch manager.authorizationStatus {
        case .notDetermined:
            manager.requestWhenInUseAuthorization()
        case .authorizedWhenInUse:
            manager.requestAlwaysAuthorization()
        default:
            break
        }
    }

    func startTracking() {
        isTracking = true
        manager.startUpdatingLocation()
        manager.startMonitoringSignificantLocationChanges()
        if manager.authorizationStatus == .authorizedAlways {
            manager.allowsBackgroundLocationUpdates = true
            manager.pausesLocationUpdatesAutomatically = false
        }
    }

    func stopTracking() {
        isTracking = false
        manager.allowsBackgroundLocationUpdates = false
        manager.stopUpdatingLocation()
        manager.stopMonitoringSignificantLocationChanges()
    }

    /// Current battery as an integer percent (0-100), or nil if unknown.
    var batteryPercent: Int? {
        let level = UIDevice.current.batteryLevel
        guard level >= 0 else { return nil }
        return Int((level * 100).rounded())
    }

    /// Forces an immediate upload of the most recent coordinate (e.g. a manual
    /// check-in), bypassing the throttle.
    func uploadNow() {
        guard let coord = lastCoordinate else { return }
        lastUploadAt = Date()
        upload(lat: coord.latitude, lng: coord.longitude, battery: batteryPercent)
    }

    // MARK: - Internal

    private func handleNewCoordinate(lat: Double, lng: Double) {
        lastCoordinate = CLLocationCoordinate2D(latitude: lat, longitude: lng)
        let now = Date()
        if let last = lastUploadAt, now.timeIntervalSince(last) < minUploadInterval {
            return
        }
        lastUploadAt = now
        upload(lat: lat, lng: lng, battery: batteryPercent)
    }

    private func upload(lat: Double, lng: Double, battery: Int?) {
        guard let userId = userIdProvider() else { return }
        let nightId = activeNightIdProvider()
        uploadTask?.cancel()
        uploadTask = Task { [api] in
            try? await api.updateLocation(userId: userId, lat: lat, lng: lng, batteryLevel: battery)
            if let nightId {
                try? await api.reportNightLocation(nightId: nightId, userId: userId, lat: lat, lng: lng, batteryLevel: battery)
            }
        }
    }

    private func applyAuthorizationChange(_ status: CLAuthorizationStatus) {
        authorizationStatus = status
        switch status {
        case .authorizedWhenInUse:
            manager.requestAlwaysAuthorization()
            startTracking()
        case .authorizedAlways:
            startTracking()
        default:
            break
        }
    }
}

// CoreLocation delivers these callbacks on the main thread for a manager created
// on the main actor, so we hop back to the main actor with only Sendable values.
extension LocationManager: CLLocationManagerDelegate {
    nonisolated func locationManager(_ manager: CLLocationManager, didUpdateLocations locations: [CLLocation]) {
        guard let coord = locations.last?.coordinate else { return }
        let lat = coord.latitude
        let lng = coord.longitude
        Task { @MainActor [weak self] in self?.handleNewCoordinate(lat: lat, lng: lng) }
    }

    nonisolated func locationManagerDidChangeAuthorization(_ manager: CLLocationManager) {
        let status = manager.authorizationStatus
        Task { @MainActor [weak self] in self?.applyAuthorizationChange(status) }
    }

    nonisolated func locationManager(_ manager: CLLocationManager, didFailWithError error: Error) {
        // Transient failures are common; updates resume on the next fix.
    }
}
