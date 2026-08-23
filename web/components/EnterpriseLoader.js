export default function EnterpriseLoader({
  message = "Preparing your operations workspace",
  detail = "Synchronizing secure modules and live restaurant data",
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
              <small>Management Console</small>
            </span>
            <span className="enterprise-loader-secure"><i /> Secure session</span>
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
            <span><i /> Interface</span>
            <span><i /> Operations</span>
            <span><i /> Insights</span>
          </div>
        </div>
      </div>
    </div>
  );
}
