# HTML Product Spec Sync Evidence

Status: static sync passed; local Chromium visual evidence passed; in-app browser HTTP preview visual evidence passed; direct local-file navigation blocked by URL policy

## Changed File

- `codex-plus-product-spec.html`

## Static Checks

The HTML page was updated to avoid product-copy drift from the 07 release docs:

- Stale fixed model examples were removed from visible demo and script data.
- Model copy no longer hardcodes a stale `gpt-5-mini` demo path; `gpt-5` appears only as backend-configured model-family wording, not as a client-bundled entitlement promise.
- Fake user-managed API key examples were replaced with redacted token wording.
- Fixed quota and usage-counter examples in bootstrap/usage JSON were replaced with backend snapshot wording.
- Architecture copy now names Control Plane, Data Plane, Client Runtime and Platform Ops.
- Model and quota entitlement wording now points to backend configuration rather than client-bundled promises.
- Follow-up drift audit removed the hardcoded demo quota progress value.
- Demo quota display text is now separate from `quotaProgress`, so labels such as `后台快照` or `提醒` are not written into CSS width.

Static scan patterns checked:

- known fixed demo model names
- fake user-managed API key samples
- stale remaining-usage fields
- hardcoded usage percentages
- `fixed model`
- `固定模型`
- `固定额度`
- `固定价格`
- hardcoded quota CSS variables
- quota label values written directly into CSS width

Result: no matches except expected CSS/runtime-neutral values after the final scan.

## Browser Visual Evidence

Local Chromium visual evidence passed for the HTML product spec.

Generated and reviewed screenshots:

- `07-integration-release/docs/html-visual-evidence/product-spec-desktop.png`
- `07-integration-release/docs/html-visual-evidence/product-spec-mobile.png`

Review result:

- Desktop `1440x900`: nonblank first viewport; hero, navigation, product card and next-section hint render without obvious overlap.
- Mobile `390x844`: nonblank first viewport; navigation, hero headline, lead copy, buttons and signal cards fit without right-side text clipping after the responsive lead-copy fix.

In-app browser HTTP preview visual evidence passed for this local HTML target on this host.

Attempted in-app Browser verification for the HTML page after the browser runtime connected successfully. Direct navigation to the local `file://` HTML target was rejected by the in-app Browser URL policy, so the direct file target remains unproven. The same file was then served over local HTTP at `http://127.0.0.1:8099/codex-plus-product-spec.html`; in-app Browser navigation succeeded, desktop/mobile screenshots were captured, and visual review passed without obvious overlap, blank primary content or visible right-side clipping.

Boundary evidence:

- `07-integration-release/docs/html-visual-evidence/in-app-browser-policy-boundary.md`
- `07-integration-release/docs/html-visual-evidence/in-app-browser-http-desktop.png`
- `07-integration-release/docs/html-visual-evidence/in-app-browser-http-mobile.png`

## Release Boundary

This evidence only covers static HTML copy alignment. It does not prove:

- purchase/login/launch E2E;
- installer behavior;
- migration behavior;
- direct `file://` rendering of this local HTML file in the in-app browser;
- production release readiness.
