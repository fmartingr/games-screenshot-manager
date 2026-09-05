---
id: TQ-0001
title: Normalize gallery names and links to Unicode NFC
status: done
priority: high
labels:
  - bug
  - component/backend
created: 2026-09-05T19:59:55+02:00
updated: 2026-09-05T20:12:30+02:00
---

## Problem

A game title with an accent produced a link that returned 404 on the static
host. The href and the directory were two different byte sequences. Both
render as "Pokémon".

| Thing | Bytes for the accent | Form |
| --- | --- | --- |
| Directory on the host | `C3 A9` | NFC, precomposed U+00E9 |
| href in the HTML | `65 CC 81` | NFD, `e` + U+0301 |

A web server matches the raw bytes of a URL path, so the 404 was correct.

Measured on the deployed site: 237 `index.html` files, 20 with an NFD href.
Two combining marks appeared, `%CC%81` (U+0301) and `%CC%83` (U+0303).

Percent-encoding is not part of the problem. `%20` and `%C3%A9` are both
correct.

## Design

NFC is the target form. The W3C names NFC as the form for the web. Git on
macOS converts to NFC for portability. Every filesystem in use stores NFC or
compares it correctly.

`normalize.go` holds the rule:

```go
func NormalizeName(name string) string { return norm.NFC.String(name) }
```

**The node carries the name that is on disk.** The gallery walk renames an
entry to NFC and then builds the node from the name the rename left behind. A
link is escaped from that name and is never normalized again. A rename that is
skipped or fails therefore yields a link that still matches its file.

## Implementation

**`gallery.go`**

- `renameToNFC` renames through a temporary name. A normalization-insensitive
  volume such as APFS resolves both forms to one file, and POSIX `rename` is a
  no-op in that case.
- `normalizeFolder` reads a folder once and returns a `folderEntry` per item,
  holding the name on disk after the rename attempt.
- The collision check uses a byte-exact set built from the entries already
  read. `os.Stat` cannot serve, because an insensitive volume reports the NFC
  name as present while the stored name is still NFD.
- `normalizeEntry` returns the original name on every path that does not
  rename: a collision, a sidecar whose media file kept its name, a dry run, or
  a failure. A failure is logged and the loop continues.
- A sidecar is `<media name>.<suffix>`, so it stays with its media file.

**`files.go`**

- `GetPathForGame` normalizes the platform, the game name and the game ID.
- `adoptExistingDir` walks each segment below the output path. Where a segment
  is missing but a sibling holds the same name in another form, that sibling is
  renamed to NFC and reused. Without it a provider run creates a second game
  directory beside the first and copies every screenshot again.
- `ProcessMedia` normalizes the media file name into `mediaName` and keeps
  `destMediaPath` in step with it.

**`gallery_node.go` and `templates/album.html`**

- `GetWebPath` and `GetWebCoverPath` escape the stored name as it is.
- `og:url` uses `GetWebPath`. It previously wrote the raw, unencoded
  `.Node.Path`.

**`.github/workflows/test.yml`**

- `gofmt`, `go vet` and `go test` on ubuntu-latest and macos-latest. macOS is
  in the matrix because APFS exercises a different path in this code.
- `video.go` had trailing whitespace and no final newline. It is formatted, so
  the new gate passes.

`go.mod` gains `golang.org/x/text`.

## Tests

`requireNormalizationSensitiveFS` probes the temporary directory and skips
when the filesystem cannot hold both forms as separate names.

- `gallery_test.go` — the rename, a collision, the loop continuing past a
  skipped entry, a sidecar staying with its media file, a dry run.
- `files_test.go` — `GetPathForGame`, directory adoption, adoption under a dry
  run, and `ProcessMedia` reusing an NFD directory rather than duplicating it.
- `gallery_node_test.go` — a stored name is escaped as it is, in either form.

## State

The change is in the working tree. It is not committed.

Verified on Linux against a fixture with 3 NFD names and ASCII controls:

- `grep -rl '%CC%' . --include=index.html` returns nothing.
- 0 NFD names remain on disk.
- 25 links across 6 pages resolve to a real path.
- `-dry-run` renames nothing and leaves all 25 links resolving.
- A second run performs 0 renames.
- `gofmt -l .` is empty. `go vet` and `go test ./...` pass.

The macOS path is covered by the CI matrix, not by a local run.

## Follow-up

The 20 `index.html` files already on the host must be re-uploaded. A
regenerate alone does not reach them.
