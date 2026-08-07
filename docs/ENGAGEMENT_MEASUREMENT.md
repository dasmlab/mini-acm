# Engagement time measurement

How “time on page” evolved from a misleading gap metric into foreground and
attention signals — and what mock-me records in the Activity log experiment.

## Why classic “time on page” lied

Universal Analytics (and many early analytics stacks) estimated time on a page
as the gap between consecutive hit timestamps:

- Page A at T0, Page B at T1 → time on A ≈ T1 − T0
- The **last page** in a session had **no next hit**, so dwell was often **0**
- A **bounce** (single pageview) also showed **0** time on page
- Background / idle tabs still counted if the next hit arrived later

So the metric answered “how long until the next pageview?” — not “how long was
someone actually looking at or using this page?”

## GA4: engagement time (foreground only)

Google Analytics 4 shifted to **engagement time** (`engagement_time_msec`):

- Time accumulates only while the page is in the **foreground**
- Uses the [Page Visibility API](https://developer.mozilla.org/en-US/docs/Web/API/Page_Visibility_API)
  (`document.visibilityState`) plus focus/blur style signals
- Idle background tabs stop accruing engagement time
- Still a **presence** model: visible ≠ actively interacting

GA4 also defines **engaged sessions** with heuristics such as:

- Session lasted ≥ ~10 seconds, **or**
- Had a conversion event, **or**
- Had ≥ 2 pageviews / screen views

Those rules filter bounce-like noise; they are not a full attention model.

## Presence vs attention

| Signal | What it measures | Weakness |
|--------|------------------|----------|
| Hit-gap “time on page” | Delay until next navigation | Last page / bounce = 0; idle tabs count |
| Visible / foreground ms | Tab open and visible | User may be looking away |
| Input-gated “engaged” ms | Recent mouse/scroll/key/touch | Misses pure reading without input |
| Heartbeat while visible | Continuous presence pings | Still not attention |

**Input-gated active time** (Salesforce-style “active attention”) only counts
intervals where there was recent input (e.g. within the last ~5 seconds). That
under-counts quiet reading but avoids counting a forgotten foreground tab as
“engaged.”

## What mock-me implements

Activity events live in `DATA_DIR/activity/events.jsonl`. Types:

- `login` — OIDC callback / first `/auth/me` per browser session
- `navigate` — client route change (or page hide), with dwell metrics
- `engaged` — reserved for explicit flush events (same schema)

On each **navigate-away**, the SPA posts:

| Field | Meaning |
|-------|---------|
| `dwellMs` | Wall-clock ms on the route (enter → leave) |
| `visibleMs` | Foreground/visible ms (Visibility API + window focus) |
| `engagedMs` | ms while recent input within ~5s **and** page was visible |

Identity is stamped server-side from the SSO session (`preferred_username` /
`sub` / `email`); clients cannot spoof another user. `GET /api/v1/activity` is
restricted to viewers in `ACTIVITY_VIEWERS` (default: `dasm`).

This is a **lab experiment**, not a full analytics product: no heatmaps, no
session replay, no third-party SDK, last-N JSONL only.

## Further reading

- [GA4: About analytics data and engagement](https://support.google.com/analytics/answer/11109416) (Google Help)
- [Page Visibility API](https://developer.mozilla.org/en-US/docs/Web/API/Page_Visibility_API) (MDN)
- Historical UA “Time on Page” / bounce interaction (many migration notes from UA → GA4)
