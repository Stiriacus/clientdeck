# Fonts

`roboto-400.woff2`, `roboto-500.woff2`, `roboto-700.woff2` are the **latin**
subset of [Roboto](https://fonts.google.com/specimen/Roboto) (Google Fonts,
Apache License 2.0), downloaded from `fonts.gstatic.com`.

They are self-hosted so a board page makes **no external requests** — the files
are embedded into the binary along with the rest of the `plain` theme.

Glyphs outside the latin subset (U+0000–U+00FF plus a few punctuation marks)
fall back to the system sans-serif stack declared in `--font`.
