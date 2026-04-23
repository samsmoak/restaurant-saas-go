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

### Public + customer — per tenant (`/api/r/:slug`)
| Method | Path | Auth | Notes |
|--------|------|------|-------|
| GET | `/menu` | — | Categories with embedded items |
| GET | `/restaurant` | — | Public view of the restaurant settings |
| GET | `/restaurant/status` | — | Opening hours + `manual_closed` |
| GET | `/orders/:order_number` | — | Order tracking by number |
| POST | `/checkout/create-intent` | JWT + customer profile | Build order + Stripe PaymentIntent |

### Customer-scoped (`/api/me`, JWT)
| Method | Path | Notes |
|--------|------|-------|
| GET / PUT | `/profile` | Read / update customer profile |
| GET | `/orders` | Latest 50 orders. `?restaurant_id=` optional filter |

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

## Notes

- Checkout recomputes every price from the DB. Client-submitted prices are ignored.
- Order numbers are `ORD-NNNNN` — globally unique, not per-tenant.
- Stripe webhook updates orders across all tenants by `payment_intent_id`. The hub then broadcasts to the right per-tenant admin channel and the per-order public channel.
- Admin finalize uses a Mongo transaction; Atlas / replica-set deployment is required.
