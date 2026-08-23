# Feature: Secure Idle Session & Password Change

Branch recommendation: `feature/secure-session-password`

## Scope

Security/authentication hardening only. No Kubernetes, Vault, Docker, database schema, Next.js config, or environment config changes.

## Changes

- JWT absolute lifetime is 30 minutes in `auth-service`.
- Browser token/user state remains in `sessionStorage`, never persistent `localStorage`.
- Legacy Tropical auth keys in `localStorage` are removed automatically.
- Frontend signs users out after 15 minutes of inactivity. Click, keyboard, scroll, touch, and route navigation reset the idle timer.
- Frontend also schedules logout at the JWT `exp` timestamp, so active sessions still require re-authentication after 30 minutes.
- Login form no longer ships with a prefilled email/password.
- Added authenticated password-change endpoint with current-password verification.
- New password minimum: 12 characters.
- Admin Users page includes a Change My Password security panel.
- Successful password changes clear the session and require a fresh login.
- Users navigation is hidden for non-admin roles; backend authorization remains the authority.

## Existing RBAC Preserved

The application remains multi-role:

- Admin: full access.
- Auditor: read operational data and mutate audit/issue workflows.
- Staff: read operational data and mutate permitted sales workflows.
- Authenticated roles: General Live Chat.
- User lifecycle API remains admin-only.

## Important Vault Note

`BOOTSTRAP_ADMIN_PASSWORD` is only used when the bootstrap admin does not already exist. Updating that Vault value alone does not reset the password of an existing database user. Use the in-app password change flow while authenticated. Emergency forgotten-password recovery should be handled as a separate controlled admin operation.
