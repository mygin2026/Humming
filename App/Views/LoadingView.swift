import UIKit

final class LoadingView: UIView {
    private let indicator = UIActivityIndicatorView(style: .large)
    private let progressView = UIProgressView(progressViewStyle: .default)

    override init(frame: CGRect) {
        super.init(frame: frame)
        setup()
    }

    required init?(coder: NSCoder) {
        super.init(coder: coder)
        setup()
    }

    private func setup() {
        backgroundColor = AppConfig.backgroundColor
        isUserInteractionEnabled = false

        indicator.translatesAutoresizingMaskIntoConstraints = false
        progressView.translatesAutoresizingMaskIntoConstraints = false
        progressView.progressTintColor = UIColor(hex: "#1A73E8")
        progressView.trackTintColor = UIColor(white: 0.9, alpha: 1)

        addSubview(indicator)
        addSubview(progressView)

        NSLayoutConstraint.activate([
            indicator.centerXAnchor.constraint(equalTo: centerXAnchor),
            indicator.centerYAnchor.constraint(equalTo: centerYAnchor),

            progressView.leadingAnchor.constraint(equalTo: leadingAnchor),
            progressView.trailingAnchor.constraint(equalTo: trailingAnchor),
            progressView.topAnchor.constraint(equalTo: safeAreaLayoutGuide.topAnchor),
            progressView.heightAnchor.constraint(equalToConstant: 2)
        ])
    }

    func start() {
        isHidden = false
        indicator.startAnimating()
        progressView.progress = 0
    }

    func setProgress(_ value: Double) {
        progressView.setProgress(Float(value), animated: true)
    }

    func stop() {
        indicator.stopAnimating()
        isHidden = true
        progressView.progress = 0
    }
}
