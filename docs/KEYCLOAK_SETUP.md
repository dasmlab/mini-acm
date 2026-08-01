# Keycloak SSO setup for mini-mock

Same dasmlab realm pattern as interview-me.

## Cluster IdP

| Item | Value |
|------|--------|
| Keycloak URL | `https://keycloak.apps.2026-prod-1.ocp.dasmlab.org` |
| Realm | `dasmlab` |
| Issuer | `https://keycloak.apps.2026-prod-1.ocp.dasmlab.org/realms/dasmlab` |
| App URL | `https://mini-mock.apps.2026-prod-1.ocp.dasmlab.org` |
| Client ID | `mini-mock` |

## Create client

1. Client type: OpenID Connect · Client ID: `mini-mock`
2. Client authentication: **ON** · Standard flow: ON
3. Redirect URIs:
   - `https://mini-mock.apps.2026-prod-1.ocp.dasmlab.org/api/v1/auth/callback`
   - `https://*.apps.2026-prod-1.ocp.dasmlab.org/*`
   - `http://localhost:8080/api/v1/auth/callback`
4. Client role: `admin` — assign to staff users (Filter by clients → mini-mock)
5. Ensure `roles` client scope / User Client Role mapper puts `resource_access["mini-mock"].roles` in the access token

## Env (serve / Deployment)

```bash
KEYCLOAK_URL=https://keycloak.apps.2026-prod-1.ocp.dasmlab.org
KEYCLOAK_REALM=dasmlab
OIDC_CLIENT_ID=mini-mock
OIDC_CLIENT_SECRET=<from Keycloak Credentials tab>
APP_PUBLIC_URL=https://mini-mock.apps.2026-prod-1.ocp.dasmlab.org
OIDC_REDIRECT_URI=https://mini-mock.apps.2026-prod-1.ocp.dasmlab.org/api/v1/auth/callback
# optional lab CA
# OIDC_CA_FILE=/etc/oidc/ca.crt
```

When these are unset, serve runs **open local/dev** (no login). With them set, APIs require the `admin` client role.

Store the secret in K8s as `mini-mock-oidc` / key `client-secret` (do not commit).
