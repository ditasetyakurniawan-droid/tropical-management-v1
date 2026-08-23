export default function EnterpriseLoader({
  message = "Menyiapkan ruang kerja operasional",
  detail = "Menyinkronkan modul aman dan data restoran terbaru",
  embedded = false,
}) {
  return (
    <div className={`enterprise-loader ${embedded ? "enterprise-loader-embedded" : ""}`} role="status" aria-live="polite" aria-label={message}>
      <div className="enterprise-loader-panel">
        <div className="enterprise-loader-grid" aria-hidden="true" />
        <div className="enterprise-loader-glow enterprise-loader-glow-a" aria-hidden="true" />
        <div className="enterprise-loader-glow enterprise-loader-glow-b" aria-hidden="true" />

        <div className="enterprise-loader-content">
          <div className="enterprise-loader-brand">
            <span className="enterprise-loader-brand-mark" aria-hidden="true"><span /></span>
            <span className="enterprise-loader-brand-copy">
              <strong>TROPICAL<span>.</span></strong>
              <small>Konsol Manajemen</small>
            </span>
            <span className="enterprise-loader-secure"><i /> Sesi aman</span>
          </div>

          <div className="enterprise-loader-stage" aria-hidden="true">
            <span className="enterprise-orbit enterprise-orbit-outer" />
            <span className="enterprise-orbit enterprise-orbit-middle" />
            <span className="enterprise-orbit enterprise-orbit-inner" />
            <span className="enterprise-loader-core"><span /></span>
          </div>

          <div className="enterprise-loader-copy">
            <p>{message}<span className="enterprise-loading-dots"><i /><i /><i /></span></p>
            <span>{detail}</span>
          </div>

          <div className="enterprise-loader-progress" aria-hidden="true">
            <span />
          </div>

          <div className="enterprise-loader-meta" aria-hidden="true">
            <span><i /> Antarmuka</span>
            <span><i /> Operasional</span>
            <span><i /> Wawasan</span>
          </div>
        </div>
      </div>
    </div>
  );
}
