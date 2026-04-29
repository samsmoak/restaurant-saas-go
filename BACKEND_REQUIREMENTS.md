# Savorar — Backend Requirements

This document is the contract between the Savorar Flutter client and the
backend. **Every endpoint listed here is called by the client today**;
when an endpoint is unimplemented the client surfaces an error state to
the user.

The Flutter app at `/Users/samuelenamzih/Desktop/savorarclient` ships
**no mock fallbacks** — repositories speak HTTP/SSE/WebSocket directly.
If you want to bring a screen up locally, you must implement the
endpoint(s) it depends on.

- **Base URL:** `--dart-define=SAVORAR_API_BASE_URL=…`
  (see [`lib/core/config/app_config.dart`](lib/core/config/app_config.dart))
- **WS base URL:** `--dart-define=SAVORAR_WS_BASE_URL=…`
- **Auth header:** `Authorization: Bearer <accessToken>` on every request
  except the auth endpoints below.
- **Content type:** `application/json; charset=utf-8` for all non-SSE
  endpoints.
- **Money:** integer cents (`price_cents: 1250` = `$12.50`).
- **Time:** ISO-8601 UTC strings (`2026-04-29T14:32:00Z`).
- **IDs:** opaque server-issued strings; do not assume UUIDs.
- **Errors:** non-2xx responses must return JSON
  `{"error": "human message"}` (the client maps this into the
  user-visible error state).

---

## 1. Auth

### `POST /api/auth/google`

Exchange a Google ID token for a Savorar session.

**Request**
```json
{
  "id_token": "<Google id_token>"
}
```

**Response 200**
```json
{
  "token": "<Savorar bearer token>",
  "user": {
    "id": "u_…",
    "email": "alex@example.com",
    "name": "Alex K.",
    "photo_url": "https://…",
    "phone": "+1…"
  },
  "profile": { "...": "see /api/me/profile shape" },
  "is_admin": false,
  "memberships": [
    { "restaurant_id": "r_…", "role": "manager" }
  ],
  "is_new_user": true
}
```

**Critical fields**
- `token` — stored in iOS Keychain / Android Keystore via
  `SecureStorageService` and replayed in the `Authorization` header.
- `is_new_user` — when `true`, the client routes the user into Taste
  Profile setup before Home (see Welcome flow). Must default to `false`.

### `POST /api/auth/signout`

Server invalidates the session. Client clears local credentials
regardless of the response.

### `GET /api/auth/memberships`

Returns the membership list from the session payload above. Used to
gate restaurant-admin tooling (out of scope for v1 customer app).

---

## 2. Profile, addresses, payment methods

### `GET /api/me/profile` / `PUT /api/me/profile`

```json
{
  "id": "u_…",
  "user_id": "u_…",
  "email": "alex@example.com",
  "full_name": "Alex K.",
  "phone": "+1…",
  "default_address": "123 Hume Ave",
  "addresses": [ /* see /api/me/addresses */ ],
  "photo_url": "https://…"
}
```

`PUT` takes a partial — only `full_name`, `phone`, `default_address`,
`photo_url` are writable. Server ignores other fields.

### `GET /api/me/addresses`, `POST /api/me/addresses`, `DELETE /api/me/addresses/{id}`

Address shape:
```json
{
  "id": "addr_…",
  "label": "Home",
  "address": "123 Hume Ave",
  "city": "Alexandria",
  "state": "VA",
  "zip": "22301",
  "lat": 38.81,
  "lng": -77.05,
  "floor": "1st",
  "landmark": "near the laundromat"
}
```

### `GET /api/me/payment-methods`, `POST /api/me/payment-methods/setup-intent`, `DELETE /api/me/payment-methods/{id}`

`POST .../setup-intent` returns `{ "client_secret": "seti_…_secret_…" }`
which the client hands to Stripe to attach the card.

### `GET /api/me/uploads/presign` *(query: `content_type`, `filename`)*

Returns a presigned PUT URL for avatar / review-photo uploads:
```json
{
  "upload_url": "https://…",
  "public_url": "https://…",
  "fields": { "...": "..." }
}
```

### `GET /api/me/stats` *(NEW — used by Profile screen)*

Lifetime totals shown on `04 · Profile`. The client renders 0 until
this endpoint exists.
```json
{
  "orders": 47,
  "saved": 12,
  "reviews": 4
}
```

---

## 3. Discovery

### `GET /api/restaurants` *(query: `cuisine`, `lat`, `lng`, `near`, `tag`)*

Paginated list for the Home feed. Response:
```json
{
  "restaurants": [
    {
      "id": "r_…",
      "name": "Briyani Bar",
      "description": "Hyderabadi · Halal",
      "logo_url": "https://…/hero.jpg",
      "cuisine_tags": ["Indian", "Hyderabadi"],
      "average_rating": 4.5,
      "rating_count": 6234,
      "estimated_delivery_time": 18,
      "estimated_pickup_time": 15,
      "distance_km": 2.3,
      "delivery_fee_cents": 0,
      "min_order_cents": 700,
      "currency": "usd",
      "featured": true,
      "manual_closed": false,
      "opening_hours": {
        "mon": { "open": "11:00", "close": "22:00", "closed": false }
      }
    }
  ],
  "next_cursor": null
}
```

### `GET /api/restaurants/search?q=…`

Same response shape. **Important:** match restaurants AND dishes; the
client renders the result in the Restaurants tab — Dishes results come
from `/api/ai/search` so the AI ranks them.

### `GET /api/restaurants/search/suggest?q=…`

Returns inline suggestions for the search field:
```json
{
  "suggestions": [
    { "label": "Tom Yum Goong", "type": "dish" },
    { "label": "Bangkok Soul",  "type": "restaurant" }
  ]
}
```

### `GET /api/restaurants/{id}` and `GET /api/r/{id}/restaurant`

Single-restaurant payload (same fields as the list item, plus
`phone`, `formatted_address`, `latitude`, `longitude`, `timezone`).

### `GET /api/r/{id}/menu`

```json
{
  "categories": [
    {
      "id": "c_…",
      "name": "Popular",
      "items": [
        {
          "id": "mi_…",
          "name": "Mutton Biryani",
          "description": "Slow-cooked basmati with bone-in mutton…",
          "image_url": "https://…",
          "base_price_cents": 1500,
          "is_featured": true,
          "is_available": true,
          "tags": ["Spicy", "Halal"],
          "sizes": [
            { "id": "s_half",   "name": "Half plate",  "price_modifier_cents": 0,    "is_default": true },
            { "id": "s_full",   "name": "Full plate",  "price_modifier_cents": 400 },
            { "id": "s_family", "name": "Family",      "price_modifier_cents": 2200 }
          ],
          "extras": [
            { "id": "e_raita", "name": "Raita", "price_cents": 150 }
          ]
        }
      ]
    }
  ]
}
```

### `GET /api/r/{id}/restaurant/status`

```json
{ "is_open": true, "next_open_at": null, "manual_closed": false }
```

---

## 4. Reviews

### `GET /api/r/{restaurantId}/reviews`

```json
{
  "reviews": [
    {
      "id": "rv_…",
      "user_name": "Alex K.",
      "rating": 5,
      "tags": ["Tasty", "Hot", "Big portion"],
      "comment": "…",
      "photos": ["https://…"],
      "created_at": "2026-04-12T14:32:00Z"
    }
  ]
}
```

### `POST /api/me/reviews`

```json
{
  "order_id": "o_…",
  "restaurant_id": "r_…",
  "rating": 4,
  "tags": ["Tasty", "Hot"],
  "comment": "Worth it.",
  "photos": ["https://…"],
  "courier_thumbs": "up"
}
```

Rejects with 409 if the order has already been reviewed.

---

## 5. Cart, checkout, orders

### `POST /api/r/{restaurantId}/checkout/create-intent`

Server is the source of truth for prices. Client sends the cart shape;
server returns the breakdown plus a Stripe payment intent.

**Request**
```json
{
  "lines": [
    {
      "menu_item_id": "mi_…",
      "size_id": "s_full",
      "extra_ids": ["e_raita"],
      "quantity": 2,
      "special_instructions": "extra spicy"
    }
  ],
  "delivery_address_id": "addr_…",
  "delivery_mode": "standard",
  "promo_code": "WELCOME10",
  "tip_percent": 15,
  "group_note": "Ring twice."
}
```

**Response 200**
```json
{
  "order_draft_id": "od_…",
  "payment_intent_client_secret": "pi_…_secret_…",
  "summary": {
    "subtotal_cents":     3450,
    "delivery_fee_cents":  199,
    "service_fee_cents":   250,
    "tip_cents":           517,
    "tax_cents":           276,
    "discount_cents":     -345,
    "total_cents":        4347,
    "currency": "usd"
  }
}
```

### `GET /api/me/orders`

```json
{
  "orders": [
    {
      "id": "o_…",
      "order_number": "M-2024-…",
      "restaurant_id": "r_…",
      "restaurant_name": "Briyani Bar",
      "restaurant_logo_url": "https://…",
      "items_summary": "Mutton biryani, Raita, Papadum",
      "status": "in_progress",
      "placed_at": "2026-04-29T18:42:00Z",
      "estimated_delivery_at": "2026-04-29T19:15:00Z",
      "total_cents": 3250,
      "currency": "usd"
    }
  ]
}
```

`status` is one of `confirmed | preparing | en_route | delivered | cancelled`.

### `GET /api/r/{restaurantId}/orders/{orderNumber}`

Full order detail. Same shape as list + `lines`, `courier`,
`delivery_address`, `payment_method_summary`.

### WebSocket `wss://…/ws/orders/{orderNumber}`

Client subscribes for live tracking. Server pushes JSON frames:
```json
{
  "type": "status",
  "status": "preparing",
  "estimated_minutes_remaining": 14
}
```
```json
{
  "type": "courier",
  "name": "Marcus",
  "vehicle": "Bicycle",
  "rating": 4.9,
  "lat": 38.811,
  "lng": -77.05
}
```
```json
{
  "type": "delivered",
  "delivered_at": "2026-04-29T19:14:00Z"
}
```

Connection lifecycle: client reconnects on close (30 s back-off);
server should accept duplicate subscriptions per token without complaint.

---

## 6. Favorites

### `GET /api/me/favorites`

```json
{
  "restaurants": [ /* same restaurant shape as discovery */ ],
  "dishes": [ /* see /api/ai/dishes/{id} */ ]
}
```

### `PUT /api/me/favorites/{restaurantId}` / `DELETE /api/me/favorites/{restaurantId}`

Toggles save state. Returns `204 No Content`.

---

## 7. Savor-AI (RAG taste assistant) — **NEW endpoints**

This is the headline feature. The client is fully built and waiting on
these endpoints. Until they exist, every Savor-AI screen surfaces an
error state.

### `POST /api/ai/chat` *(Server-Sent Events)*

The single entry point for the chat experience. Body:
```json
{
  "conversation_id": null,
  "message": "Something warm and spicy for tonight, not too heavy.",
  "voice": false,
  "location": { "lat": 38.81, "lng": -77.05 }
}
```

Response is `Content-Type: text/event-stream`. **Each event is one
`data:` JSON object** (no `event:` line needed). The client consumes
events in order and rebuilds the AI bubble after each. The client
expects four event types:

#### `clarify` — taste-bar interjection (optional)

Emit before any retrieval steps when the prompt is ambiguous (no
spice/richness/dietary signal). The Flutter chat renders the sliders
inline so the user can adjust before search runs.

```json
{
  "type": "clarify",
  "taste": { "spice": 7, "richness": 4, "acidity": 3, "carbs": 2 },
  "suggestions": ["Looks right", "Less spicy", "More citrus"]
}
```

#### `step` — retrieval progress

Emit one event per pipeline phase. The client looks up `step` by id
and replaces the matching row, so a `pending → active → done`
progression mutates a single line in the bubble.

```json
{
  "type": "step",
  "step": "parse",
  "status": "active",
  "meta": { "count": 14 }
}
```

Step ids the client recognises: `parse`, `filter`, `search`, `rerank`,
`done`. Status: `pending | active | done`. `meta.count` is rendered
after the label (e.g. "Filtered restaurants · 14 results").

#### `answer` — final grounded response

Always emit exactly one `answer` event. The `text` is the AI
explanation. `sources` becomes the chip strip below the bubble.
`dishes` becomes the embedded dish previews.

```json
{
  "type": "answer",
  "text": "Found 5 dishes that fit a warm, lightly spicy profile…",
  "sources": [
    { "kind": "menu",    "name": "Bangkok Soul · Menu" },
    { "kind": "review",  "name": "14 reviews"           },
    { "kind": "taste",   "name": "Your taste profile"   }
  ],
  "dishes": [
    {
      "id": "d_tom_yum",
      "name": "Tom Yum Goong",
      "restaurant_id": "r_bangkok_soul",
      "restaurant_name": "Bangkok Soul",
      "match": 94,
      "price_cents": 1250,
      "image_url": "https://…",
      "emoji": "🍜",
      "tags": ["Spicy", "Citrus"],
      "spice": 7,
      "prep_minutes": 12,
      "dietary": "Pescatarian",
      "description": "Lemongrass, lime, prawns, mild chili",
      "flavor": {
        "umami": 8, "sweet": 4, "sour": 6,
        "salty": 4, "spicy": 7, "rich": 5
      }
    }
  ]
}
```

`source.kind` ∈ `{menu, review, restaurant, taste}` (drives the
chip's leading glyph). `match` is 0–100.

#### `followups` — suggested next prompts

```json
{
  "type": "followups",
  "chips": ["Cheaper options", "Vegetarian instead", "Anything with rice?"]
}
```

Client renders these as outlined suggestion chips below the answer.

---

### `POST /api/ai/recommend`

Used by `A6 · Recommend Sheet`. Returns a ranked list given a parsed
taste fingerprint.

**Request**
```json
{
  "taste": { "spice": 7, "citrus": 6, "richness": 4 },
  "location": { "lat": 38.81, "lng": -77.05 }
}
```

**Response**
```json
{
  "dishes": [ /* same shape as the answer.dishes[] above */ ]
}
```

### `GET /api/ai/search?q=…`

Used by Search → Dishes tab and by Vector Search Results (`A5`).
Optional query params from the chips: `spicy`, `citrus`, `under_15`,
`under_30_min`, `rating_4_5_plus`. Same `dishes[]` shape.

### `GET /api/ai/dishes/{id}`

Single dish detail (`A7`). Same shape as the dish array element above.
Including `flavor` is required so the Flavor Radar renders.

### Cravings history (`A9`)

- `GET /api/ai/cravings`
- `PUT /api/ai/cravings/{id}/pin` — pin
- `DELETE /api/ai/cravings/{id}/pin` — unpin
- `DELETE /api/ai/cravings/{id}` — delete

```json
{
  "cravings": [
    {
      "id": "cv_…",
      "title": "Warm + spicy, not too heavy",
      "summary": "Found Tom Yum at Bangkok Soul",
      "emoji": "🍜",
      "date_label": "Today, 3:42 PM",
      "match": 94,
      "pinned": false
    }
  ]
}
```

### Taste profile

- `GET /api/me/taste-profile`
- `PUT /api/me/taste-profile`

```json
{
  "spice": 7,
  "dietary": ["Halal"],
  "cuisines": ["Italian", "Thai", "Indian"],
  "allergens": ["Peanuts"]
}
```

The Flutter client persists this client-side too, but the server is
authoritative — return the canonical version on PUT.

---

## 8. Notifications & FCM (`20 · Notifications`)

### `GET /api/me/notifications`

```json
{
  "notifications": [
    {
      "id": "n_…",
      "kind": "courier",
      "title": "Marcus is on the way",
      "body": "Briyani Bar · arriving in 12 min",
      "icon_emoji": "🛵",
      "color": "primary",
      "created_at": "2026-04-29T18:48:00Z",
      "read": false,
      "deep_link": "/orders/M-2024-…"
    }
  ]
}
```

`kind` ∈ `{courier, promo, review, ai_tip, order_status}`. `color` ∈
`{primary, blue, green}` and drives the icon-disc tint in the UI.

### `POST /api/me/notifications/read`

Body: `{"ids": ["n_…", "n_…"]}` to mark read in bulk; the "Mark all
read" CTA sends `{"all": true}`.

### `POST /api/me/devices`

```json
{ "fcm_token": "…", "platform": "ios" }
```

Used at sign-in. Server attaches the token to the user for push
fan-out. `DELETE /api/me/devices/{token}` on sign-out.

---

## 9. Group order (`14 · Group Order`)

### `POST /api/groups`

```json
{ "restaurant_id": "r_…" }
```

→
```json
{
  "id": "g_…",
  "share_code": "3FQ2",
  "share_url": "savorar.app/g/3FQ2",
  "host_user_id": "u_…",
  "min_for_free_delivery_cents": 4000,
  "lock_expires_at": null
}
```

### `GET /api/groups/{shareCode}`

Returns members + their sub-carts. Members poll this every 5 s today;
upgrading to a WebSocket (`/ws/groups/{shareCode}`) is desirable.

```json
{
  "id": "g_…",
  "host_user_id": "u_…",
  "members": [
    {
      "user_id": "u_…",
      "name": "Aisha",
      "avatar_url": "https://…",
      "lines": [ /* cart line shape */ ],
      "subtotal_cents": 1100,
      "status": "ready"
    }
  ],
  "subtotal_cents": 3450,
  "lock_expires_at": null
}
```

### `POST /api/groups/{shareCode}/join`

Body: `{}`. Adds the caller as a member.

### `POST /api/groups/{shareCode}/lock`

Hosts only. Body:
```json
{ "lock_minutes": 5 }
```

### `POST /api/groups/{shareCode}/checkout`

Hosts only, after lock. Returns the same `payment_intent_client_secret`
flow as `/api/r/{id}/checkout/create-intent` but settles the entire
group as a single order.

---

## 10. Conventions

### Pagination
- Cursor-based. Response shape: `{ "items": [...], "next_cursor": "..." }`.
- Pass cursor on subsequent requests as `?cursor=…`.
- Page size is server-decided; client never sends `limit`.

### Money
- All prices/fees/tips/totals are integer cents.
- Field names end with `_cents`.
- Currency is a sibling field on the relevant payload (`"currency": "usd"`).

### Timestamps
- ISO-8601 with timezone `Z` for UTC; the client formats locally.
- `*_label` fields (e.g. `date_label`) are server-rendered display
  strings — used where the client cannot easily compute the right
  format (e.g. "Today, 3:42 PM" vs "Apr 24").

### Idempotency
- All `POST` endpoints that mutate cart/payment state accept an
  `Idempotency-Key` header. The client retries safe-to-retry calls.

### Error model
- 4xx/5xx responses must be JSON `{"error": "human readable message"}`.
  The client surfaces this directly in its error states.
- 401 — client clears credentials and routes to Welcome.
- 402 — Stripe payment failed; the message is shown verbatim.
- 429 — client backs off + shows "Try again later".

### CORS / origins
- N/A; no web client today.

### Locale
- All user-visible strings are English-only for v1. The Voice Mode
  language picker is a stub.

---

## 11. Removed / out of scope

The following surfaces in earlier specs are intentionally not
implemented and the backend should not expose them:

- `15 · Schedule for Later`
- `16 · Promos & Aura Rewards` browse screen *(promo code entry is
  still supported via `promo_code` on `create-intent` — there's no
  endpoint that returns "available promos" today)*
- Restaurant-admin and courier flows (separate apps).

---

## 12. Implementation roadmap (suggested)

A reasonable ordering for new backend work:

1. `GET /api/me/stats` — unblocks the Profile screen.
2. `POST /api/ai/chat` SSE — unblocks the entire Savor-AI flow.
3. `GET /api/ai/dishes/{id}` + `GET /api/ai/search` —
   `A5 · Vector Search Results` and `A7 · Dish Detail`.
4. `POST /api/ai/recommend` — `A6 · Recommend Sheet`.
5. `GET /api/me/taste-profile` + `PUT` — Taste Profile setup wizard
   (`A2`) and the chat's clarify event consumes it for context.
6. Cravings endpoints — `A9 · Your cravings`.
7. `GET /api/me/notifications` + FCM token registration — `20 ·
   Notifications` and push.
8. Group order endpoints — `14 · Group Order`.

Anything not listed here is already wired up and exercised by the
existing customer flows.

---

## Contact

Open issues against this repo with the `backend` label. Field-level
questions go in the PR that adds the endpoint; protocol-level
questions update this doc.
