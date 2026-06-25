//
//  QRScannerView.swift
//  hackathon_fe
//
//  Camera-based scanner for join codes. Invite QR codes now encode a bare group
//  id (no deep link), so a successful scan yields the group id directly.
//

import SwiftUI
import AVFoundation

/// Sheet that shows the camera and reports the first decoded QR string.
struct QRScannerSheet: View {
    /// Called with the decoded QR string (a group id).
    var onCode: (String) -> Void

    @Environment(\.dismiss) private var dismiss
    @State private var cameraDenied = false

    var body: some View {
        NavigationStack {
            ZStack {
                if cameraDenied {
                    deniedView
                } else {
                    QRScannerRepresentable(onCode: onCode, onDenied: { cameraDenied = true })
                        .ignoresSafeArea()
                    reticle
                }
            }
            .navigationTitle("Scan Join Code")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") { dismiss() }
                }
            }
        }
    }

    private var reticle: some View {
        VStack(spacing: 16) {
            RoundedRectangle(cornerRadius: 24)
                .stroke(.white, lineWidth: 3)
                .frame(width: 240, height: 240)
                .opacity(0.9)
            Text("Point at a group's QR code")
                .font(.subheadline)
                .foregroundStyle(.white)
                .shadow(radius: 4)
        }
    }

    private var deniedView: some View {
        ContentUnavailableView {
            Label("Camera Access Needed", systemImage: "camera.fill")
        } description: {
            Text("Enable camera access in Settings to scan a group's join code.")
        } actions: {
            Button("Open Settings") {
                if let url = URL(string: UIApplication.openSettingsURLString) {
                    UIApplication.shared.open(url)
                }
            }
            .buttonStyle(.borderedProminent)
        }
    }
}

private struct QRScannerRepresentable: UIViewControllerRepresentable {
    var onCode: (String) -> Void
    var onDenied: () -> Void

    func makeCoordinator() -> Coordinator {
        Coordinator(onCode: onCode)
    }

    func makeUIViewController(context: Context) -> ScannerViewController {
        let controller = ScannerViewController()
        controller.coordinator = context.coordinator
        controller.onDenied = onDenied
        return controller
    }

    func updateUIViewController(_ uiViewController: ScannerViewController, context: Context) {}

    final class Coordinator: NSObject, AVCaptureMetadataOutputObjectsDelegate {
        private let onCode: (String) -> Void
        private var didScan = false

        init(onCode: @escaping (String) -> Void) {
            self.onCode = onCode
        }

        func metadataOutput(_ output: AVCaptureMetadataOutput,
                            didOutput metadataObjects: [AVMetadataObject],
                            from connection: AVCaptureConnection) {
            guard !didScan,
                  let object = metadataObjects.first as? AVMetadataMachineReadableCodeObject,
                  let value = object.stringValue?.trimmingCharacters(in: .whitespacesAndNewlines),
                  !value.isEmpty else { return }
            didScan = true
            onCode(value)
        }
    }
}

/// Hosts the AVCaptureSession and renders its preview layer.
final class ScannerViewController: UIViewController {
    weak var coordinator: QRScannerRepresentable.Coordinator?
    var onDenied: (() -> Void)?

    private let session = AVCaptureSession()
    private let sessionQueue = DispatchQueue(label: "qr.scanner.session")
    private var previewLayer: AVCaptureVideoPreviewLayer?

    override func viewDidLoad() {
        super.viewDidLoad()
        view.backgroundColor = .black
        previewLayer = AVCaptureVideoPreviewLayer(session: session)
        previewLayer?.videoGravity = .resizeAspectFill
        if let previewLayer { view.layer.addSublayer(previewLayer) }
        requestAccessAndConfigure()
    }

    override func viewDidLayoutSubviews() {
        super.viewDidLayoutSubviews()
        previewLayer?.frame = view.bounds
    }

    override func viewWillDisappear(_ animated: Bool) {
        super.viewWillDisappear(animated)
        sessionQueue.async { [session] in
            if session.isRunning { session.stopRunning() }
        }
    }

    private func requestAccessAndConfigure() {
        switch AVCaptureDevice.authorizationStatus(for: .video) {
        case .authorized:
            configureSession()
        case .notDetermined:
            AVCaptureDevice.requestAccess(for: .video) { [weak self] granted in
                DispatchQueue.main.async {
                    if granted { self?.configureSession() } else { self?.onDenied?() }
                }
            }
        default:
            onDenied?()
        }
    }

    private func configureSession() {
        sessionQueue.async { [weak self] in
            guard let self else { return }
            self.session.beginConfiguration()

            guard let device = AVCaptureDevice.default(for: .video),
                  let input = try? AVCaptureDeviceInput(device: device),
                  self.session.canAddInput(input) else {
                self.session.commitConfiguration()
                return
            }
            self.session.addInput(input)

            let output = AVCaptureMetadataOutput()
            guard self.session.canAddOutput(output) else {
                self.session.commitConfiguration()
                return
            }
            self.session.addOutput(output)
            output.setMetadataObjectsDelegate(self.coordinator, queue: .main)
            output.metadataObjectTypes = [.qr]

            self.session.commitConfiguration()
            self.session.startRunning()
        }
    }
}
