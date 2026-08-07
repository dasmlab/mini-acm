# Keycloak SSO setup for mock-me

Same dasmlab realm pattern as interview-me.

## Cluster IdP

| Item | Value |
|------|--------|
| Keycloak URL | `https://keycloak.apps.2026-prod-1.ocp.dasmlab.org` |
| Realm | `dasmlab` |
| Issuer | `https://keycloak.apps.2026-prod-1.ocp.dasmlab.org/realms/dasmlab` |
| App URL | `https://mock-me.apps.2026-prod-1.ocp.dasmlab.org` |
| Client ID | `mock-me` |

## Create client

1. Client type: OpenID Connect · Client ID: `mock-me`
2. Client authentication: **ON** · Standard flow: ON
3. Redirect URIs:
   - `https://mock-me.apps.2026-prod-1.ocp.dasmlab.org/api/v1/auth/callback`
   - `https://dev-*-mock-me.apps.2026-prod-1.ocp.dasmlab.org/api/v1/auth/callback`
   - `https://*.apps.2026-prod-1.ocp.dasmlab.org/*`
   - `http://localhost:8080/api/v1/auth/callback`
4. Web origins (as needed):
   - `https://mock-me.apps.2026-prod-1.ocp.dasmlab.org`
   - `https://dev-*-mock-me.apps.2026-prod-1.ocp.dasmlab.org`
5. Client role: `admin` — assign to staff users (Filter by clients → mock-me)
6. Ensure `roles` client scope / User Client Role mapper puts `resource_access["mock-me"].roles` in the access token

## Env (serve / Deployment)

```bash
KEYCLOAK_URL=https://keycloak.apps.2026-prod-1.ocp.dasmlab.org
KEYCLOAK_REALM=dasmlab
OIDC_CLIENT_ID=mock-me
OIDC_CLIENT_SECRET=<from Keycloak Credentials tab>
APP_PUBLIC_URL=https://mock-me.apps.2026-prod-1.ocp.dasmlab.org
OIDC_REDIRECT_URI=https://mock-me.apps.2026-prod-1.ocp.dasmlab.org/api/v1/auth/callback
# optional lab CA
# OIDC_CA_FILE=/etc/oidc/ca.crt
```

When these are unset, serve runs **open local/dev** (no login). With them set, APIs require the `admin` client role.

Store the secret in K8s as `mock-me-oidc` / key `client-secret` (do not commit).


## Prod wiring (2026-prod-1)

- Namespace: `mock-me-system`
- Secret: `mock-me-oidc` (key `client-secret`)
- ConfigMap: `mock-me-oidc-ca` (lab CA for Keycloak TLS)
- Route: https://mock-me.apps.2026-prod-1.ocp.dasmlab.org (HAProxy **CERT55**)
- Client role may be named `Admin` or `admin` (match is case-insensitive)
- Without a session, `GET /api/v1/mockups` returns **401** when OIDC is enabled

## Developer previews

Per-developer sites (any non-`main` branch push):

| Item | Value |
|------|--------|
| Host | `https://dev-{github-user}-mock-me.apps.2026-prod-1.ocp.dasmlab.org` |
| Namespace | `mock-me-dev-{user}` |
| Cert | `scripts/ci/ensure-preview-cert.sh` on HAProxy `10.20.1.10` (first preview only) |
| Secrets | Copied from `mock-me-system` (`dasmlab-ghcr-pull`, `mock-me-oidc`, optional CA/ssh/pull) |

See [DEVELOPER.md](./DEVELOPER.md).
