# API Contract

This is the reference documentation for the frozen contracts. Shapes are
stable from the first merge onward and only extensible additively (new
optional fields); existing field names are never renamed or repurposed.

## Authentication

All write routes (`POST /api/v1/views`, `POST /webhook`) require the header

```
X-Webhook-Secret: <VITRINE_WEBHOOK_SECRET>
```

If the header is missing or doesn't match, the service responds identically
in both cases with `401 unauthorized`. An attacker can't infer from this
whether a secret is even configured. Read access (`GET /c/{slug}`) requires
no secret; the slug itself is the capability.

## `POST /api/v1/views`

Alias: `POST /webhook`, identical behavior, shorter path for n8n
configurations.

### Request

```json
{
  "customer_id": "acme-corp",
  "client_name": "ACME Corp",
  "language": "fr",
  "intro": "Optional intro text above the tabs.",
  "products": [
    {
      "category": "Printers",
      "title": "Brother HL-L2375DW",
      "recommendation": "The best balance of price and toner cost for your volume.",
      "specs": { "Print engine": "Laser B/W", "Pages/min": "34", "Duplex": "yes" },
      "rating": 4.5,
      "affiliate_link": "https://example.com/p/123?tag=xyz",
      "image_url": "https://example.com/img/123.jpg",
      "price": "$179.00",
      "badge": "Best Value",
      "highlights": ["Laser B/W, 34 pages/min", "Automatic duplex printing"],
      "pros": ["Low toner cost", "Reliable for years in similar setups"],
      "cons": ["No colour printing", "No Wi-Fi on base model"]
    }
  ]
}
```

| Field | Required | Type | Note |
|---|---|---|---|
| `customer_id` | yes | string | Upsert key. Supplied by the client and stays stable across updates. |
| `client_name` | yes | string | ≤ 120 characters. |
| `intro` | – | string | Free text. |
| `language` | – | string, BCP 47 | e.g. `"en"`, `"fr"`, `"fr-CH"`. Defaults to `"en"` when empty. Controls the HTML `lang` attribute and all translated UI strings on the rendered board. |
| `products` | yes | array, ≥ 1, ≤ 100 | See below. |
| `products[].category` | yes | string | Determines tab membership and order (first appearance in the array). |
| `products[].title` | yes | string | ≤ 200 characters. |
| `products[].recommendation` | – | string | ≤ 2000 characters. |
| `products[].specs` | – | `map[string]string`, ≤ 20 entries | Rendered as a definition list inside the product detail accordion. Key order is not guaranteed by JSON; the renderer may sort keys for display. |
| `products[].rating` | – | number, 0.0–5.0 | Half steps are rendered; missing = no star block (not the same as `0`). |
| `products[].affiliate_link` | – | string, `http`/`https` | Other schemes (`javascript:`, `data:`, `file:`, …) → `400 invalid_url`. |
| `products[].image_url` | – | string, `http`/`https` | Same as `affiliate_link`. |
| `products[].price` | – | string | Freely formatted, e.g. `"$179.00"`. |
| `products[].badge` | – | string | e.g. `"Best Value"`. |
| `products[].highlights` | – | array, ≤ 5, each ≤ 160 characters | Key facts shown on the card surface for quick scanning. When omitted, the first two spec entries are auto-derived (alphabetically) as a fallback. |
| `products[].pros` | – | array, ≤ 10, each ≤ 300 characters | Advantages shown inside the product detail accordion. Intended for AI-generated content. |
| `products[].cons` | – | array, ≤ 10, each ≤ 300 characters | Disadvantages shown inside the product detail accordion. Intended for AI-generated content. |

**Unknown fields** are accepted, ignored, and reported back in the response
under `ignored_fields` as well as logged at `warn` level. An n8n workflow
that's ahead of an older binary doesn't break completely because of this,
but a typo in an optional field doesn't silently disappear either. The
operator can deliberately fail their n8n workflow on a non-empty
`ignored_fields`.

**Body limit:** 1 MiB. Larger bodies are rejected with `413` before JSON is
parsed.

### Response `200`

```json
{
  "customer_id": "acme-corp",
  "slug": "acme-corp-a8f9b2",
  "url": "https://boards.example.com/c/acme-corp-a8f9b2",
  "created": true,
  "ignored_fields": ["products[0].colour"]
}
```

- `created: false` when updating an existing customer (`customer_id`
  already present); the slug stays unchanged in this case.
- `ignored_fields` is omitted when empty.
- `url` is built from `VITRINE_BASE_URL` + `/c/` + `slug`. The operator
  gets the finished link straight back from n8n, e.g. for an email to the
  customer.

### Errors

Always in the format `{"error":"<code>","message":"<human readable>"}`.

| Status | `error` | Trigger |
|---|---|---|
| `400` | `invalid_payload` | Broken JSON, missing required field, wrong type, rating outside 0–5 |
| `400` | `invalid_url` | `affiliate_link`/`image_url` not `http`/`https` |
| `401` | `unauthorized` | Secret missing or wrong |
| `413` | `payload_too_large` | Body > 1 MiB |
| `500` | `slug_exhausted` | 5 consecutive slug collisions (practically never at board counts in the hundreds) |

### Examples

```sh
# Create/update a board
curl -X POST https://boards.example.com/api/v1/views \
  -H "X-Webhook-Secret: $VITRINE_WEBHOOK_SECRET" \
  -H "Content-Type: application/json" \
  --data @testdata/example_payload.json

# Missing secret -> 401
curl -i -X POST https://boards.example.com/api/v1/views \
  -H "Content-Type: application/json" \
  --data '{}'

# Invalid affiliate_link -> 400 invalid_url
curl -i -X POST https://boards.example.com/api/v1/views \
  -H "X-Webhook-Secret: $VITRINE_WEBHOOK_SECRET" \
  -H "Content-Type: application/json" \
  --data '{"customer_id":"x","client_name":"X","products":[{"category":"C","title":"T","affiliate_link":"javascript:alert(1)"}]}'
```

## Other routes

| Route | Response |
|---|---|
| `GET /c/{slug}` | `200 text/html` (board) or `404` (HTML 404 page in the theme, not JSON) |
| `GET /static/{path...}` | Theme assets from `embed.FS`, `Cache-Control: public, max-age=31536000, immutable` |
| `GET /healthz` | `200 {"status":"ok"}` |

## Theme view model

The shape that external themes render against is documented in
[`docs/theming.md`](theming.md).

## Rule for new fields

The payload schema is deliberately static and typed: there is no generic
`meta`/`extra` map passthrough. In short: pure display values belong in
`specs` (no code needed), anything with its own styling/position/behavior
becomes a new optional typed field.
