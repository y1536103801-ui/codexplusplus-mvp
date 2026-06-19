# In-App Browser HTTP Preview Evidence and File URL Boundary

Date: 2026-06-18
Status: in-app browser HTTP visual evidence passed; direct local file URL remains blocked by URL policy

## Scope

This records the coordinator verification of `codex-plus-product-spec.html` in the Codex in-app Browser after local Chromium desktop/mobile screenshots had already passed.

## Local File Attempt

- Browser runtime connection: succeeded.
- Target: local `codex-plus-product-spec.html` through direct `file://` navigation.
- Intended viewports: desktop `1440x900`, mobile `390x844`.
- Result: the in-app Browser rejected direct navigation to the local `file://` URL under its URL policy.

## Local HTTP Preview Attempt

- Preview URL: `http://127.0.0.1:8099/codex-plus-product-spec.html`.
- HTTP check: returned `200` for the HTML target.
- In-app Browser navigation: succeeded for the local HTTP preview.
- Desktop viewport requested: `1440x900`.
- Mobile viewport requested: `390x844`.
- DOM signals observed: title `Codex++ 成品方案`; H1 `把 Codex 变成买完即用的客户端。`; `Control Plane`, `Data Plane`, `Client Runtime`, `Platform Ops`, and `quotaProgress` all present.
- Desktop review: nonblank first viewport; hero, navigation, product card and next-section hint render without obvious overlap.
- Mobile review: nonblank first viewport; navigation, hero headline, lead copy, buttons and signal cards fit without visible right-side clipping.

## Evidence Files

- `07-integration-release/docs/html-visual-evidence/in-app-browser-http-desktop.png` (`1424x900`)
- `07-integration-release/docs/html-visual-evidence/in-app-browser-http-mobile.png` (`374x828`)

## Evidence Boundary

- Local Chromium screenshots remain the final docs/product-copy evidence package screenshots because that verifier expects exact desktop/mobile PNG dimensions for the copied evidence bundle.
- In-app Browser rendering of the local HTML through HTTP preview is now proven on this host.
- Direct `file://` rendering of the local HTML file remains blocked by URL policy and is not claimed as passed.
- This does not affect E2E, package or compatibility release evidence, which still require production-equivalent external environments.
