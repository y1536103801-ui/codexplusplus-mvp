# 04 HTML Sync Evidence

Run folder: 20260618-1403-docs
Status: final

Result: pass

## HTML Evidence Result

- Static sync passed for `codex-plus-product-spec.html`.
- Local Chromium visual evidence passed using the stored desktop and mobile screenshots in `05-html-visual-evidence/`.
- Approved browser preview passed through `http://127.0.0.1:8092/codex-plus-product-spec.html`; the page loaded with title text, visible H1 text, nonempty body text and no horizontal overflow in the browser DOM check.
- Direct `file://` in-app Browser navigation is a URL-policy limitation on this host. The approved in-app Browser evidence path is local HTTP preview, recorded in `05-html-visual-evidence/in-app-browser-policy-boundary.md`.
- Desktop 1440x900 screenshot and Mobile 390x844 screenshot were reviewed without obvious overlap and without right-side text clipping.
- Fixed-value residue scan result: pass; no matches except expected CSS/runtime-neutral values.
