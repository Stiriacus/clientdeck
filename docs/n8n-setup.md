# n8n Integration

vitrine is populated exclusively via JSON over the API, the intended
way to do that is an n8n workflow that fires an `HTTP Request` node against
the webhook at the end.

## HTTP Request node

| Setting | Value |
|---|---|
| Method | `POST` |
| URL | `https://boards.example.com/api/v1/views` (or `.../webhook`, identical alias, if a shorter path is preferred) |
| Authentication | "None". The secret goes through a plain header, not through n8n's auth mechanism |
| Header `X-Webhook-Secret` | `{{ $credentials... }}` or an n8n environment variable. **Do not** hardcode into the node, see below |
| Header `Content-Type` | `application/json` |
| Body Content Type | JSON |
| Body | see mapping below |

**Secret handling:** the secret belongs in an n8n credential or an
environment variable that's referenced via an expression
(`{{ $env.VITRINE_SECRET }}`), not as a literal string in the node.
otherwise it ends up in the workflow export and in every backup of it.

## JSON body mapping

The body must match the contract in [`docs/api.md`](api.md). Minimal
example as an n8n "Set"/"Edit Fields" node before the HTTP Request node,
filling in the required fields plus a few optional ones:

```json
{
  "customer_id": "={{ $json.customerId }}",
  "client_name": "={{ $json.companyName }}",
  "intro": "={{ $json.introText }}",
  "products": "={{ $json.products }}"
}
```

`products` should already be assembled as an array in the target shape in
an upstream workflow step (`category`, `title`, optional `recommendation`,
`specs`, `rating`, `affiliate_link`, `image_url`, `price`, `badge`), e.g.
via a "Code" node that maps the raw data from your product source (sheet,
CRM, inventory system) into this schema.

**Category and product order:** vitrine takes over the order of the
`products` array 1:1 (category tabs in order of first appearance, products
within a category in array order). Sorting therefore happens in n8n, not
on the server. A "Sort" node before the request node controls the
resulting tab order.

## Reusing the response

The response contains the finished board URL:

```json
{
  "customer_id": "acme-corp",
  "slug": "acme-corp-a8f9b2",
  "url": "https://boards.example.com/c/acme-corp-a8f9b2",
  "created": true
}
```

Common next step in the workflow: a "Send Email" node that inserts
`{{ $json.url }}` into a template, e.g.:

> Hi {{ $json.client_name }}, here's your current product recommendation:
> {{ $json.url }}

`created` distinguishes a new creation from an update, useful for, e.g.,
sending a "new board" email only on `created: true` instead of an "updated
board" email.

## Handling errors

On an error, vitrine responds with a non-2xx status and
`{"error":"<code>","message":"..."}` (full table in
[`docs/api.md`](api.md)). Enable "Continue On Fail" on the HTTP Request node
and check `$json.error` in the following step to, e.g., deliberately abort
the workflow with a meaningful message on `invalid_payload` instead of
silently continuing.

**Recommendation:** if `ignored_fields` is non-empty in a successful
response, that points to a typo or a not-yet-supported field in the
upstream mapping. Deliberately failing the workflow at this point (an
`IF` node on `$json.ignored_fields.length > 0`) prevents a field from being
dropped permanently and unnoticed.
