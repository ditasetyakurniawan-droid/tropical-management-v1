# Feature: Modern Enterprise Loading UI

**Branch:** `feature/modern-enterprise-loading-ui`

## Scope
Frontend/view-only refinement for Tropical Management. No backend service, database, API contract, environment file, build configuration, or package dependency was changed.

## What changed
- Added a reusable enterprise loading experience with Tropical green/gold branding.
- Added a global internal-route transition overlay for navigation between application pages.
- Added a slim animated route progress indicator.
- Added Next.js App Router `loading.js` fallback UI.
- Replaced the basic session bootstrap spinner with the enterprise loader.
- Added subtle page-entry, card, navigation, chart, progress, hero-glow, and button micro-interactions.
- Improved login loading feedback.
- Added responsive behavior and `prefers-reduced-motion` support.

## Changed UI files
- `web/app/globals.css`
- `web/app/layout.js`
- `web/app/loading.js` (new)
- `web/app/login/page.js`
- `web/components/EnterpriseLoader.js` (new)
- `web/components/RouteTransition.js` (new)
- `web/components/SessionGate.js`

## Validation
- JavaScript/JSX syntax check: passed.
- CSS structural brace check: passed.
- Protected backend/config comparison against provided main ZIP: unchanged.
- Full `npm run build` was not completed in the sandbox because the environment cannot resolve `registry.npmjs.org`, so dependencies cannot be fetched here. Run `npm ci && npm run build` locally before merging the PR.
