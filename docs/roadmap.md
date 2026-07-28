# Roadmap: API-first & maximale Flexibilisierung

Zielbild: **vitrine ist ein headless Board-Backend.** Die HTML-Ausgabe ist
*ein* Consumer der API, nicht ihr Kern. Alles was ein Theme anzeigt, ist über
die API abrufbar; alles was ein Kunde individualisiert, ist Payload — nicht
Code.

Dieser Fahrplan ist additiv gedacht: keine Phase bricht einen bestehenden
n8n-Workflow, ein bestehendes Theme oder eine bestehende Datenbank.

---

## Leitplanken (gelten für jede Phase)

1. **Der Wire-Contract ist nicht das Domain-Modell.** Öffentliche JSON-Antworten
   bekommen eigene DTOs (`internal/httpapi/wire`), nie direkt `board.BoardView`.
   Sonst wird jedes interne Refactoring zum Breaking Change.
2. **Additiv oder gar nicht.** Neue optionale Felder, nie Umbenennungen. Die
   Regel steht schon in `docs/api.md:148` — sie gilt ab jetzt auch für die
   Read-Seite.
3. **Themes rechnen nicht.** Flexibilität kommt als *Daten* in der `BoardView`
   an. Sobald Templates Logik enthalten, sind sie nicht mehr gefahrlos
   austauschbar.
4. **Validierung bleibt serverseitig.** Jedes neue freie Feld braucht Limits in
   `internal/board/validate.go`, sonst ist die 1-MiB-Body-Grenze die einzige
   Schranke.
5. **Kein Feature ohne Test und Doku-Zeile.** Golden-Tests für HTML,
   Contract-Tests für JSON.
6. **Infrastruktur gehört in den Reverse Proxy, nicht in die App.** vitrine
   terminiert kein TLS (`README.md:81`) — und genauso wenig macht es
   Zugangslisten, Rate-Limiting oder HTTP-Auth. Das kann nginx besser, in
   jeweils ein bis zwei Zeilen, ohne dass die App Zustand, IPs oder Cookies
   anfassen muss:

   ```nginx
   # Board nur für bestimmte Netze
   location /c/riverside-books-a8f9b2 { allow 203.0.113.0/24; deny all; }
   # Schreibroute nur intern
   location /api/    { allow 10.0.0.0/8; deny all; proxy_pass http://127.0.0.1:8080; }
   # Falls je nötig
   limit_req_zone $binary_remote_addr zone=boards:10m rate=10r/s;
   ```

   **Der inhaltliche Rahmen:** vitrine zeigt Produktempfehlungen und Showcases.
   Es ist kein Zahlungs-, Konto- oder Gesundheitsdaten-System. Maßnahmen, die
   nur mit "Betriebssicherheit" begründbar sind, gehören nicht in dieses
   Repository.

---

## Phase 1 — Read-API & Content-Negotiation

**Warum zuerst:** Ohne Read-Pfad ist "API first" eine Behauptung. Alles Weitere
(externes Frontend, Preview, Migration, Backup) hängt daran.

> **Kein `tenant_id` (Entscheidung 1 = Single-Operator).** Isolation zwischen
> Endkunden läuft über den Slug als Capability, nicht über Auth — Endkunden
> haben keinen API-Zugriff. Mehrere Betreiber isolieren sich über getrennte
> Deployments (eigenes Binary, eigene DB, eigenes Secret). `customer_id` bleibt
> Primary Key.

**Änderungen**

| Datei | Was |
|---|---|
| `internal/board/ports.go` | `Store` um `ByCustomerID`, `List`, `Delete` erweitern |
| `internal/store/sqlite.go` | Implementierung; `created_at`/`updated_at` mit selektieren (existieren, werden aktuell nie gelesen, `sqlite.go:171`) |
| `internal/httpapi/wire/` | **neu**: `BoardDocument`, `ProductDocument`, `ViewResponse` als stabile Wire-DTOs |
| `internal/httpapi/views.go` | **neu**: Handler für die Read-Routen |
| `internal/httpapi/board.go` | Content-Negotiation auf `GET /c/{slug}`, ETag |
| `internal/httpapi/router.go` | Routen + CORS-Middleware |

**Neue Routen**

```
GET /api/v1/views/{customer_id}   → 200 ViewResponse | 404 unknown_customer   (auth)
GET /api/v1/views?limit=&cursor=  → 200 {items:[…], next_cursor}              (auth)
GET /c/{slug}                     → HTML (default) | JSON bei Accept: application/json
GET /c/{slug}.json                → dasselbe explizit, für dumme Clients      (public)
```

**Details, die leicht übersehen werden**

- `board.BoardView` ist **nicht serialisierbar** — das Feld `T func(string) string`
  (`view.go:55`) muss im DTO durch eine `translations`-Map ersetzt werden.
- Die öffentliche `/c/{slug}`-JSON-Antwort enthält **kein** `customer_id`. Der
  Slug ist die Capability, die Kunden-ID ist Interna.
- JSON-Antworten: `X-Content-Type-Options: nosniff`, **kein** CSP-Header (der
  gehört nur an HTML, `board.go:17`).
- ETag = Hash über `updated_at` + Theme-Name + Theme-Version; `If-None-Match`
  → 304. Der Theme-Name muss rein, sonst liefert ein Theme-Wechsel einen
  Cache-Treffer mit altem Layout.
- CORS: konfigurierbare Origin-Allowlist (`VITRINE_CORS_ORIGINS`), nur für die
  öffentlichen Read-Routen, nie für `POST`.

**Fertig wenn:** ein Board vollständig per JSON abrufbar ist, `curl -H "Accept:
application/json" /c/{slug}` dieselben Daten liefert wie das HTML zeigt, und
Round-Trip-Tests (POST → GET → Vergleich) grün sind.

**Aufwand:** M (eine bis zwei Sitzungen)

---

## Phase 2 — Contract-First: OpenAPI 3.1

**Warum:** `docs/api.md` ist Prosa, die Limits stehen doppelt (dort und in
`validate.go:9-20`). Ein maschinenlesbarer Contract ist die Voraussetzung
dafür, dass "eingefroren" auch überprüfbar ist.

**Änderungen**

- `api/openapi.yaml` — Single Source of Truth für Request/Response/Fehler
- `internal/board/limits.go` — Limits als exportierte Konstanten, im Spec-Text
  referenziert; ein Test vergleicht Spec-Werte gegen Code-Konstanten
- `internal/httpapi/contract_test.go` — jede Handler-Antwort wird gegen das
  Schema validiert (`kin-openapi`), inklusive der Fehler-Bodies
- `make api-lint` + CI-Schritt
- `docs/api.md` wird zur *Erzählung*, das Spec zur *Wahrheit*

**Fertig wenn:** ein umbenanntes JSON-Feld einen roten Test erzeugt, und aus dem
Spec ein Client generierbar ist.

**Aufwand:** M

---

## Phase 3 — Theme-Manifest & `theme_options`

**Warum:** Das ist der größte Flexibilitätshebel pro Stunde. Heute heißt
"Kundenfarbe" = Theme-Verzeichnis kopieren. Danach: ein Theme, beliebig viele
Looks, ohne Rebuild.

**Änderungen**

| Datei | Was |
|---|---|
| `themes/<name>/theme.json` | **neu**: Manifest — `name`, `version`, `languages`, `options` (Typ, Default, enum/pattern), `sections` (für Phase 6) |
| `internal/render/manifest.go` | **neu**: Laden + Validieren des Manifests |
| `internal/render/theme.go` | Manifest an `Theme` hängen, bei `LoadTheme` mitladen |
| `internal/board/view.go` | `CustomerView.ThemeOptions map[string]string`, `BoardView.Options` |
| `internal/board/validate.go` | Optionen gegen das Manifest prüfen (Anzahl, Länge, enum) |
| `internal/store/migrate.go` | Schema **v4**: `ALTER TABLE customer_views ADD COLUMN theme_options TEXT NOT NULL DEFAULT '{}'` |
| `internal/httpapi/theme_css.go` | **neu**: `GET /c/{slug}/theme.css` |

**Der CSP-Punkt:** Kundenfarben brauchen CSS Custom Properties. Inline-`<style>`
ist durch `default-src 'self'; style-src 'self'` (`board.go:17`) verboten — und
das soll so bleiben. Lösung: eine **generierte, ETag-bare CSS-Route**, die nur
`:root { --accent: …; }` ausgibt, mit strikter Wert-Validierung (Farb-Pattern,
keine freien Strings). Die CSP wird *nicht* aufgeweicht.

**Neue Route**

```
GET /api/v1/themes → [{name, version, languages, options[], sections[]}]  (auth)
```

**Fertig wenn:** zwei Kunden mit demselben Theme sichtbar unterschiedlich
aussehen, ohne dass eine Datei kopiert wurde.

**Aufwand:** L

---

## Phase 4 — `extra`-Passthrough (schemafreie Inhalte)

**Warum:** Heute kostet ein neues Inhaltsfeld sechs Stellen im Code
(`view.go` → `validate.go` → `webhook.go:37` → `viewmodel.go` → Template →
Doku) und einen Release. Danach: null.

**Änderungen**

- `board.CustomerView.Extra map[string]string` und `board.Product.Extra` —
  validiert (≤ 20 Einträge, Key ≤ 40, Wert ≤ 500 Zeichen, Key-Pattern
  `^[a-z][a-z0-9_]*$`), aber **nicht interpretiert**
- Keys mit Suffix `_url` laufen durch `validateURL` (`validate.go:112`) — sonst
  landet `javascript:` in einem `href`, wenn ein Theme das Feld verlinkt
- `webhook.go:33-41`: `extra` in die Known-Fields aufnehmen, sonst meldet
  `ignoredFields` es fälschlich als ignoriert
- `viewmodel.go`: 1:1 durchreichen nach `ProductView.Extra` / `BoardView.Extra`
- Themes lesen `{{index .Extra "lieferzeit"}}` — `html/template` escaped das
  weiterhin automatisch

**Fertig wenn:** ein neues Inhaltsfeld nur noch Payload + Template kostet, kein
Go-Release.

**Aufwand:** S

---

## Phase 5 — Lifecycle & Link-Hygiene

**Warum:** Aktuell kommt nichts wieder aus dem System raus — kein DELETE, kein
Export. Genau die Punkte, die `README.md:160` als "deliberately deferred past
v1" führt. Sobald echte Kundendaten drinstehen, ist das auch DSGVO-relevant.

**5a — Lifecycle**

```
DELETE /api/v1/views/{customer_id}   → 204 | 404          (auth)
PATCH  /api/v1/views/{customer_id}   → status: draft|published, expires_at
GET    /api/v1/export                → NDJSON aller Views  (auth, Backup)
POST   /api/v1/import                → NDJSON, idempotent
```

- Schema **v5**: `status`, `published_at`, `expires_at`, `deleted_at` (Soft
  Delete, damit ein versehentliches DELETE nicht sofort Datenverlust ist)
- `GET /c/{slug}` auf `draft`/abgelaufen/gelöscht → 404-Seite (nicht 410:
  ein 410 bestätigt, dass der Slug mal existierte)

**5b — Link-Hygiene** *(drei Einzeiler, keine neue Mechanik)*

Zugangsschutz macht nginx (Leitplanke 6). Hier stehen nur die Dinge, die *im
Code* falsch sind und deren Fix nichts kompliziert macht:

- **Referrer-Leak schließen:** `<meta name="referrer" content="no-referrer">` im
  Layout jedes Themes. Heute wandert die Board-URL im `Referer`-Header zum
  Händler, sobald ein Kunde auf "Jetzt ansehen" klickt
  (`themes/plain/board.html:50` hat kein `referrerpolicy`, die `<img>`-Tags eine
  Zeile weiter oben schon). Das ist kein Sicherheits-, sondern ein
  Diskretionsthema: der Händler erfährt sonst, für welchen Kunden du was
  empfiehlst.
- **Suffix-Entropie** von 6 auf 12 base62-Zeichen (`slug.go:12`): eine Konstante,
  35,7 → 71 Bit. Bestehende Slugs bleiben gültig.
- **Modulo-Bias** in `slug.go:104` beheben — die ersten acht Alphabet-Zeichen
  sind aktuell minimal häufiger. Kosmetisch, aber drei Zeilen.

Kein Rate-Limiting, kein Board-Passwort, keine Enumerations-Erkennung in der
App. Wenn ein Board wirklich vertraulich ist, ist eine nginx-Accesslist die
richtige Antwort — nicht eine Mechanik, die alle anderen Boards mitverkompliziert.

**Fertig wenn:** ein Kunde vollständig löschbar und exportierbar ist, und die
Board-URL nicht mehr an Dritte durchsickert.

**Aufwand:** 5a = M, 5b = XS

---

## Phase 6 — Section-Modell (maximale inhaltliche Flexibilität)

**Warum jetzt und nicht früher:** Das ist der Umbau mit dem größten Blast
Radius — Validierung, View-Model und *alle* Themes. Phase 3+4 decken vorher
schon den Großteil des Bedarfs. Diese Phase lohnt erst, wenn Kunden *andere
Seitentypen* wollen, nicht nur andere Felder.

**Modell**

```json
{
  "sections": [
    { "type": "products", "title": "Drucker", "items": [ … ] },
    { "type": "richtext", "title": "Hinweis", "body": "…" },
    { "type": "comparison", "columns": [ … ] }
  ]
}
```

- `products` auf Top-Level bleibt gültig und wird intern zu genau einer
  `products`-Section pro Kategorie gemappt → **kein** Bruch für bestehende
  Workflows
- Jedes Theme deklariert in `theme.json`, welche `type`s es rendert; unbekannte
  Sections werden übersprungen und in der Webhook-Antwort gemeldet (analog zu
  `ignored_fields`) — ein Theme wird dadurch nie kaputt, nur unvollständig
- `BoardView.Sections` kommt additiv neben `Categories`; `Categories` wird erst
  entfernt, wenn kein Theme es mehr nutzt

**Fertig wenn:** ein Board aus gemischten Sektionstypen gerendert wird und die
alten Payloads unverändert weiterlaufen.

**Aufwand:** XL

---

## Phase 7 — Betrieb

Bewusst kurz. Alles was der Reverse Proxy kann, macht der Reverse Proxy.

- `Cache-Control` für HTML-Boards (aktuell nur für `/static/*`, `router.go:52`)
- Backup-Strategie für die SQLite-Datei (WAL-Checkpoint + Kopie, oder Litestream)
- Doku-Abschnitt: nginx-Snippets für Accesslists pro Board und optionales
  `limit_req` — als Rezept in `docs/startup.md`, nicht als Code

**Aufwand:** S

---

## Ausdrücklich nicht geplant

Ergänzt die Non-Goals in `README.md:156`. Diese Punkte sind geprüft und
verworfen, nicht vergessen:

- **Rate-Limiting, Accesslists, HTTP-Auth in der App** → nginx (Leitplanke 6)
- **API-Keys pro Mandant, `tenant_id`** → getrennte Deployments pro Betreiber
- **Login/Konten für Endkunden** → der Slug ist die Capability
- **Metrics-Endpoint, Audit-Log, Klick-Tracking** → Overhead ohne Nutzen bei
  fünf Kunden; das Zugriffs-Log von nginx beantwortet dieselben Fragen
- **Admin-UI/CMS** → Inhalte kommen ausschließlich über die API

---

## Abhängigkeiten

```
Phase 1 (Read-API)
   ├── Phase 2 (OpenAPI)  ← braucht die fertigen Response-Shapes
   ├── Phase 3 (theme_options)
   │      └── Phase 6 (Sections)  ← braucht das Manifest
   ├── Phase 4 (extra)            ← unabhängig, jederzeit einschiebbar
   └── Phase 5 (Lifecycle/Link-Sicherheit)
Phase 7 durchgehend
```

Phase 4 ist die billigste echte Flexibilisierung und kann jederzeit
vorgezogen werden, wenn ein konkreter Kundenwunsch drückt.

---

## Offene Entscheidungen

**1. Single-Operator oder Multi-Tenant? — ENTSCHIEDEN: Single-Operator.**
Ein Betreiber bedient seine Endkunden; jeder Kunde bekommt ein eigenes Theme und
einen eigenen Link, aber keinen API-Zugriff. Andere Betreiber hosten ihre eigene
Instanz. Konsequenzen: kein `tenant_id`, keine API-Keys pro Mandant, kein
Schema-Umbau am Primary Key. Die Isolation, die zählt, ist die des Slugs — die
Härtung dafür steht in Phase 5b.

Was das explizit **nicht** heißt: Sollte später doch eine geteilte Instanz für
fremde Betreiber kommen, ist der Nachbau teuer (Primary-Key-Wechsel auf
`(tenant_id, customer_id)` bedeutet in SQLite Tabelle neu bauen und kopieren).
Das ist bewusst in Kauf genommen, weil das Produkt laut Zuschnitt self-hosted
ausgeliefert wird.

**2. Bleibt Server-Side-Rendering die primäre UI?**
Wenn ja: Themes bleiben der Hauptweg, JSON ist Integrationsschnittstelle.
Wenn nein (SPA/Next.js als Haupt-Frontend): CORS und die öffentliche
JSON-Antwort werden Produktoberfläche und brauchen in Phase 1 deutlich mehr
Sorgfalt (Feld-Stabilität, Pagination, Bild-Proxy).

**3. Soll `products` langfristig zugunsten `sections` verschwinden?**
Wenn ja, braucht Phase 6 einen Deprecation-Zeitplan. Wenn nein, bleibt
`products` dauerhaft als bequemer Shortcut — meine Empfehlung, weil 90 % der
Boards genau das sind.

---

## Reihenfolge-Empfehlung

**1 → 4 → 3 → 2 → 5a → (5b) → 6 → 7**

Begründung für das Vorziehen von Phase 4 vor 2 und 3: `extra` ist S-Aufwand,
sofort spürbar und macht dich unabhängig von Release-Zyklen, während OpenAPI
(2) erst dann maximalen Wert hat, wenn die Read-Shapes aus Phase 1 *und* die
flexiblen Felder aus 3/4 im Spec stehen — sonst schreibst du das Spec zweimal.
