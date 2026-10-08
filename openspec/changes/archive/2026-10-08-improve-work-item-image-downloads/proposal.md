## Why

Work-item fetch downloads attachment relations but misses screenshots embedded in fields and comments. Agents also lack a source-to-file mapping, and colliding attachment names can overwrite downloaded evidence.

## What Changes

- Discover HTML inline images in work-item string fields and HTML/Markdown images in comments included with `--include-comments`.
- Download supported Azure DevOps work-item attachment image references alongside ordinary attachments, deduplicated per exported item.
- Export an asset manifest recording original references, field/comment provenance, local paths, and explicit reasons for unsupported references.
- Preserve readable image extensions and allocate collision-free filenames, including on case-insensitive filesystems.
- Preserve raw item/comment payloads, escaped HTML, existing output folders and stdout JSON keys. The attachment count describes downloaded files, including inline images.
- Keep failed supported downloads fatal; unsupported inline references are recorded without network access.

## Capabilities

### New Capabilities

- `ado-work-item-assets`: Discover, download, and map inline images and ordinary attachments into local work-item evidence.

### Modified Capabilities

- `cli-command-surface`: Extend work-item attachment export and help to cover inline assets and their manifest.
- `ado-work-item-comments`: Include image assets from opt-in comments while preserving raw discussion and stdout compatibility.
- `agent-skill-creation`: Teach generated agent guidance to consult the asset manifest and inspect downloaded images.

## Impact

Affected areas are `internal/ado` work-item export, attachment naming and URL resolution; `internal/cli` fetch wiring/help/generated guidance; their existing Go tests; and README documentation. HTML and Markdown parsing may introduce focused parser dependencies. PR and wiki exports, browser rendering, arbitrary external downloads, caching, concurrency, and resumable transfers are outside scope.

Risk is high because content-derived URLs reach authenticated retrieval and exported manifest behavior becomes a consumer contract. The user approved this work-item scope in the exploration conversation; these artifacts capture that approval for implementation.
