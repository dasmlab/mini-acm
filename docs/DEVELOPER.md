# Developer contribution guide

mock-me is a **shared production tool**. `main` is always what is live.
This doc is the etiquette and flow for multi-developer work without stepping on prod.

## Mental model

| Track | What it is | URL |
|-------|------------|-----|
| **Production** | `main` → GHCR → GitOps (`dasmlab-live-cicd`) → Argo → `mock-me-system` | https://mock-me.apps.2026-prod-1.ocp.dasmlab.org |
| **Your preview** | Any other branch you push → image build → GitOps `previews/{you}.yaml` → Argo → **your** NS | https://dev-**{you}**-mock-me.apps.2026-prod-1.ocp.dasmlab.org |

You develop on a branch preview. When it is good, open a **PR to `main`**.
Merging to `main` is the only path that updates production.

### Why `dev-{you}-…` and not `devbuild-{sha}-…`?

Edge TLS is terminated on the HAProxy box (`10.20.1.10`) with **one Let's Encrypt
cert per hostname**. There is no wildcard (no DNS TXT automation).

**One hostname per GitHub user** gives you:

- A stable URL while you iterate on any branch
- A natural **one active preview per developer** limit
- Cert provisioning only the **first** time that developer appears (`ensure-preview-cert.sh`)

The **image/version** (including short SHA) is still shown in the app chip and
in the GitHub Actions job summary.

---

## Happy path

1. `git checkout -b yourname-short-topic`
2. Code locally; keep commits focused
3. `git push -u origin HEAD`
4. Wait for **mock-me CX Pipeline** on that branch
5. Open the **Preview URL** from the Actions summary
6. Sign in with Keycloak (`mock-me` / `admin` role) — same IdP as prod
7. Iterate: push more commits → same URL updates (in-progress runs for you cancel)
8. Open a **PR → `main`**; review; merge → production deploy

### Fresh vs persistent preview data

By default your preview **keeps its data PVC** across pushes.

To wipe preview data on the next deploy, either:

- Include `[fresh-preview]` in the commit message, or
- Run **workflow_dispatch** with `fresh_preview=true`

### Cleanup

- Workflow **mock-me preview cleanup** (PR closed, or manual with actor)
- Or: `PREVIEW_ACTOR=you ./scripts/ci/cleanup-preview.sh` on a runner with `oc`

---

## CI / tokens / certs

Self-hosted runners resolve credentials in order:

1. `/home/dasm/gh_token`
2. `secrets.DASMLAB_GHCR_PAT`
3. `/home/dasm/gh-pat` → `DASMLAB_GHCR_PAT` / `GH_TOKEN`

Used for GHCR push and (prod) GitOps push to `lmcdasm/dasmlab-live-cicd`.

Preview deploy:

- `scripts/ci/ensure-preview-cert.sh` → SSH to `10.20.1.10`, add `CERTn=FQDN` if missing, `./runme.sh`
- `scripts/ci/deploy-preview.sh` → copy OIDC/GHCR secrets from `mock-me-system`, `oc apply` preview envelope, rollout

Concurrency: `mock-me-preview-{github.actor}` — only **one** preview pipeline per developer at a time.

---

## Keycloak

Valid redirect / web origin patterns must include preview hosts — see
[KEYCLOAK_SETUP.md](./KEYCLOAK_SETUP.md).

**Branch → push → use your personal preview → PR to `main` → prod.**
