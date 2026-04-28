# restaurant-saas-go

Go HTTP backend for a multi-tenant restaurant ordering SaaS. Replaces the old
Next.js + Supabase stack. Fiber + MongoDB + Redis, Stripe for payments, S3 for
uploads, and a per-tenant WebSocket hub for realtime updates.

## Stack

- Go 1.25, Fiber v2, go-mongo-driver v1.16
- MongoDB (single-node replica set for transactions; Atlas in production)
- Redis (rate limiting, optional)
- Stripe v76
- AWS S3 (SDK v2)
- Google OAuth (id-token)
- HS256 JWTs, 7 day TTL. Admin tokens carry `restaurant_id` + `role`.

## Multi-tenancy model

- `restaurants` collection. Each row has a unique `slug` and an `owner_id`.
- Tenant-scoped collections (`menu_items`, `categories`, `orders`, `admin_users`, `admin_invites`) all carry a `restaurant_id` and are indexed on it.
- `users` + `customer_profiles` are global — a customer can order from any restaurant.
- `admin_users` is keyed by `(user_id, restaurant_id)`, so one user can admin multiple restaurants with different roles (`owner` | `admin` | `staff`).
- Tenant resolution:
  - **Public + customer routes** live under `/api/r/:slug/**`. The `TenantResolveFromPath` middleware looks up the slug and sets the tenant context.
  - **Admin routes** live under `/api/admin/**`. The admin JWT carries `restaurant_id`; `TenantResolveFromToken` reads it, and `RequireAdminForTenant` verifies `admin_users`.

## Quick start (local)

```bash
cp .env.example .env
# edit .env: fill JWT_SECRET, and optionally Stripe/Google/S3
docker compose up -d
curl http://localhost:8080/health
```

Mongo boots as a single-node replica set `rs0`. The compose healthcheck runs `rs.initiate(...)` the first time, so transactions work immediately.

## Bootstrapping the first restaurant + admin

1. **Sign up a customer user** via `POST /api/auth/signup/customer`.
2. **Create a restaurant** with that user as owner:
   ```bash
   curl -sX POST http://localhost:8080/api/restaurants \
     -H "Authorization: Bearer $TOKEN" \
     -H 'content-type: application/json' \
     -d '{"slug":"ember-forge","name":"Ember Forge"}'
   ```
   The creator is inserted into `admin_users` with role `owner`.
3. **Activate** the admin token (scope it to the restaurant):
   ```bash
   curl -sX POST http://localhost:8080/api/auth/admin/activate \
     -H "Authorization: Bearer $TOKEN" \
     -d '{"restaurant_id":"<from step 2>"}'
   ```
   Use the returned token for every `/api/admin/**` call.

### Legacy invite bootstrap

Set `ADMIN_LEGACY_INVITE_CODE` and `ADMIN_LEGACY_RESTAURANT_SLUG` in the env. `POST /api/auth/admin/finalize {"invite_code":"..."}` then promotes the caller to `admin` of that restaurant. Unset both env vars once the first real invite is issued.

## REST surface

### Auth (`/api/auth`)
| Method | Path | Notes |
|--------|------|-------|
| POST | `/signup/customer` | Create customer identity |
| POST | `/login` | Email + password |
| POST | `/google` | Google id-token sign-in |
| GET  | `/memberships` | Restaurants the user admins (requires JWT) |
| POST | `/admin/finalize` | Claim a tenant-scoped invite; returns `{token, restaurant_id, restaurant_slug, role}` |
| POST | `/admin/activate` | Re-issue a JWT scoped to a restaurant you already admin |
| POST | `/signout` | No-op; drop the token client-side |

### Restaurants (`/api/restaurants`)
| Method | Path | Notes |
|--------|------|-------|
| POST | `/` | JWT. Create a restaurant; caller becomes `owner` |
| GET  | `/mine` | JWT. List restaurants the caller is an admin of |
| GET  | `/lookup?slug=…` | Public. Minimum info to render a tenant picker |

### Discovery (`/api/restaurants`, public)
| Method | Path | Notes |
|--------|------|-------|
| GET | `/` | Composite-ranked list. Query: `lat`, `lng`, `cuisine`, `limit` (max 50), `offset`. Returns `{restaurants, total, limit, offset}` with optional `distance_km` per row. |
| GET | `/search?q=…` | Mongo `$text` search blended with the composite ranker. Same query params as `/`. |
| GET | `/search/suggest?q=<prefix>` | `{suggestions: [string]}`. Top 10 matching restaurant names + cuisine tags. |

Ranking is a server-side composite of price (0.4) + quality (0.3) + speed (0.2) + reliability (0.1). Restaurants with `rating_count < 20` get a neutral 0.5 quality score; restaurants with `completion_rate < 0.80` are penalised ×0.7; restaurants with `rating_count ≥ 20` and `average_rating < 3.0` are excluded. The price band is **never** exposed to the client — there is no price filter.

### Public + customer — per tenant (`/api/r/:slug`)
| Method | Path | Auth | Notes |
|--------|------|------|-------|
| GET | `/menu` | — | Categories with embedded items |
| GET | `/restaurant` | — | Public view of the restaurant settings |
| GET | `/restaurant/status` | — | Opening hours + `manual_closed` |
| GET | `/orders/:order_number` | — | Order tracking by number |
| GET | `/reviews?limit=&offset=` | — | List reviews for the restaurant (newest first) |
| POST | `/checkout/create-intent` | JWT + customer profile | Build order + Stripe PaymentIntent |

### Customer-scoped (`/api/me`, JWT)
| Method | Path | Notes |
|--------|------|-------|
| GET / PUT | `/profile` | Read / update customer profile. `PUT` accepts an optional `photo_url`; `GET` returns it when set |
| GET | `/orders` | Latest 50 orders. `?restaurant_id=` optional filter |
| GET | `/favorites` | List the caller's favourite restaurants (joined to public restaurant view) |
| POST | `/favorites` | `{restaurant_id}` — idempotent upsert. 404 if restaurant unknown |
| DELETE | `/favorites/:restaurant_id` | Remove a favourite |
| POST | `/reviews` | `{order_id, rating, comment?}`. Order must belong to caller and be in `completed` or `delivered`. Triggers a recompute of the restaurant's `average_rating` + `rating_count`. 409 on duplicate (one review per order) |
| POST | `/uploads/presign` | `{filename, content_type, size}`. Customer profile photos. Limited to `image/*` and 4 MB; uploads land in the `customer-avatars/` S3 prefix. Returns `{upload_url, public_url, key}` |

### AI (`/api/ai`)
| Method | Path | Auth | Notes |
|--------|------|------|-------|
| POST | `/search` | OptionalJWT | `{query, lat?, lng?}` → `{intent, restaurants}`. Always returns 200; rule-based intent parser runs first, then the LLM refines if `LLM_API_KEY` is set |
| POST | `/chat` | JWT | `{messages, context?}` → `{reply, actions?}`. Returns HTTP 200 with a fallback reply when `LLM_API_KEY` is unset or the LLM call fails — never a 5xx |

### Admin (`/api/admin`, JWT scoped to a restaurant)
| Method | Path | Notes |
|--------|------|-------|
| GET | `/restaurant` | Full settings row |
| PUT | `/restaurant` | Update settings |
| POST | `/restaurant/manual-closed` | Toggle the open/closed override |
| GET/POST/PUT/DELETE | `/categories[/:id]` | Manage categories |
| GET/POST/PUT/DELETE | `/menu-items[/:id]` | Manage menu items |
| GET | `/orders[?status=]` | List orders; statuses = `new/preparing/ready/completed/cancelled/all` |
| GET | `/orders/analytics?from=<ms>&to=<ms>` | Range list for dashboards |
| PUT / DELETE | `/orders/:id` | Update status / delete |
| GET/POST/PATCH/DELETE | `/invites[/:id[/revoke]]` | Issue tenant invites |
| GET/DELETE | `/users[/:id]` | List/remove admins of this tenant |
| POST | `/uploads/presign` · `/uploads/direct` | S3 image uploads |
| GET | `/billing/subscription` | Subscription status |
| POST | `/billing/checkout/setup` · `/billing/checkout/subscription` · `/billing/portal` | Stripe checkout / portal flows |
| GET | `/billing/usage` | Current month: `{period_start, period_end, order_count, per_order_fee_total, tier, base_price, tier_thresholds, projected_total, currency}` |
| GET | `/billing/tier` | `{current_tier, base_price, includes_orders, next_tier_at, order_count}` — tier 1 ≤ 250 ($49), tier 2 ≤ 750 ($99), tier 3 > 750 ($149) |

### WebSocket
| Path | Auth | Broadcasts |
|------|------|------------|
| `/ws/admin/orders` | JWT scoped to a restaurant | `order.created / updated / deleted` for the admin's tenant |
| `/ws/orders/:order_number` | public | updates for that one order |

## Dev

```bash
go build ./...
go vet ./...
go run ./cmd/api
```

Hot reload is provided by the `api` compose service via `air`.

## Cloud Run deploy

See [deploy/README.md](deploy/README.md). Three scripts, `bootstrap.sh` → `set-secrets.sh` → `deploy.sh`.

## AI degradation contract

The `/api/ai/*` endpoints are designed never to return a 5xx for missing or flaky LLM access:

- `LLM_API_KEY` unset → `client.FromEnv()` returns nil. `POST /api/ai/search` runs only the rule-based intent parser (no LLM call). `POST /api/ai/chat` returns `{"reply": "AI is unavailable right now — please try again later."}` with HTTP 200.
- LLM HTTP error or invalid response → search falls back to the rule-based intent; chat returns the same fallback reply with HTTP 200 and logs the error.
- Provider selection: `LLM_PROVIDER` ∈ `{anthropic, openai}` (default `anthropic`). The OpenAI adapter speaks the `chat/completions` shape, so it works against OpenAI directly or against compatible providers via `LLM_BASE_URL` (e.g. Groq, Together AI).

## Notes

- Checkout recomputes every price from the DB. Client-submitted prices are ignored.
- Order numbers are `ORD-NNNNN` — globally unique, not per-tenant.
- Stripe webhook updates orders across all tenants by `payment_intent_id`. The hub then broadcasts to the right per-tenant admin channel and the per-order public channel.
- Admin finalize uses a Mongo transaction; Atlas / replica-set deployment is required.

## Discovery + ranking collections

- `restaurants` carries the discovery / ranking inputs: `cuisine_tags`, `location` (GeoJSON Point, populated whenever lat/lng is updated), `average_rating`, `rating_count`, `completion_rate`, `average_prep_minutes`, `price_band`. A text index on `(name, description, cuisine_tags)` and a 2dsphere index on `location` power the discovery endpoints.
- `favorites` — `{customer_id, restaurant_id, created_at}`; unique on `(customer_id, restaurant_id)`.
- `reviews` — `{restaurant_id, order_id, customer_id, rating, comment?, created_at}`; unique on `order_id` so a single order produces at most one review. Inserts trigger `RecomputeRatingAggregates` on the restaurant.
- `billing_usage` — `{restaurant_id, period_start, period_end, order_count, per_order_fee_total, currency, ...}`; unique on `(restaurant_id, period_start)`. The `payment_intent.succeeded` webhook fires `RecordOrder` (a $0.99 per-paid-order increment) into the current period's row.
- `orderService.UpdateStatus` triggers `RecomputeOperationalMetrics` (computes `completion_rate` + `average_prep_minutes` over the last 90 days).
- The `GET /api/restaurants/:id/reviews` route from the Savorar prompt is exposed at **`GET /api/r/:restaurant_id/reviews`** to match the existing `/api/r/:restaurant_id/*` tenant-routing convention.
