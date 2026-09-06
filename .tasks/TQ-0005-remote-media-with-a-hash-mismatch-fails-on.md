---
id: TQ-0005
title: Remote media with a hash mismatch fails on every run after the first
status: done
priority: high
labels:
  - bug
  - component/backend
created: 2026-09-06T11:39:59+02:00
updated: 2026-09-06T11:42:59+02:00
---

## Problem

`FileManager.ProcessMedia` renames a media file whose destination exists with
a different hash. The new name carries the source hash. For remote media the
hash is empty, because `Media.GetSourceHash` opens `SourcePath`, and
`SourcePath` is only set after the download at `files.go:208`.

So the destination becomes `name_.jpg`, with a trailing underscore and no
hash.

## The failure on run 2

1. `name.jpg` exists and `steamGalleryComparisonFunc` reports a mismatch.
2. The target becomes `name_.jpg`, and that file already exists from run 1.
3. The `sourceHash != ""` guard skips the "already exists under its hash name"
   check, because the hash is empty.
4. `copyFile` returns `ErrCopyFileDestinationExists`.
5. `ProcessMedia` returns an error, and `ProcessGame` aborts the rest of that
   game's media.

Steam online-gallery screenshots reach this path. `steam.go:178` sets
`steamGalleryComparisonFunc` on media that carries a URL and no path.

## Fix

Download remote media before the rename decision, so the hash names real
content. The download already happens in the same function, only later.

---

## Notes

- 2026-09-06T11:42:59+02:00 — Fixed. ProcessMedia now downloads remote media before it reads the hash.

  The download moved into a new helper, ensureLocalSource, which is called in
  two places: in the rename branch before the hash is read, and in the copy
  branch as before. The helper is idempotent, so the second call downloads
  nothing.

  The helper tests GetSourcePath rather than IsLocal. IsLocal stays false after
  a download, because SourceURL is kept, so IsLocal would have downloaded twice.

  A dry run downloads nothing, so its hash stays empty and the name it reports
  is not the final one. That is stated in the comment.

  Regression test added: ProcessMediaHashesRemoteMediaBeforeRenaming. It runs
  ProcessMedia twice against a local HTTP server with
  steamGalleryComparisonFunc and an existing file of the same name. Confirmed
  that it fails without the fix, with the exact reported failure:

    run 2: ProcessMedia() returned an error: error copying media
    /tmp/download_3742584927.jpg -> .../2024-01-01_12-00-00_.jpg:
    copy destination exists
