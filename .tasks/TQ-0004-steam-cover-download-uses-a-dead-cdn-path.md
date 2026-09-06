---
id: TQ-0004
title: Steam cover download uses a dead CDN path
status: done
priority: high
labels:
  - bug
  - provider/steam
  - component/backend
created: 2026-09-06T10:58:49+02:00
updated: 2026-09-06T11:35:07+02:00
---

## Problem

`SteamProvider.GetCovers` fails with a 404 for a growing set of app IDs.

```
ERR error downloading cover provider=steam game_id=4597250 game_name="Order of the Sinking Star Demo" error="error downloading cover: error downloading URL: 404 Not Found"
ERR error downloading cover provider=steam game_id=4148250 game_name="Echoes of Aincrad DEMO Version" error="error downloading cover: error downloading URL: 404 Not Found"
```

The cause is the hard-coded path in `steam_client.go:21`:

```go
steamGameHeaderURL = "https://cdn.cloudflare.steamstatic.com/steam/apps/%s/header.jpg"
```

Steam moved store assets to a per-revision path with a content hash in it. The
flat path returns 404 for every app published under the new scheme.

## Evidence

The store API gives the real URL for the two failing IDs. Both answer 200 with
`image/jpeg`:

| App ID | Real header URL |
| --- | --- |
| 4597250 | `https://shared.akamai.steamstatic.com/store_item_assets/steam/apps/4597250/c72ca605ac231c2dce09226e8494105f6d84a366/header.jpg?t=1786437981` |
| 4148250 | `https://shared.akamai.steamstatic.com/store_item_assets/steam/apps/4148250/afd5a96ebc1247efc4e3bf864bf3ea219b9a4e7e/header.jpg?t=1781625833` |

The `?t=` query is not necessary. The image answers 200 without it.

## The problem is not limited to demos

A probe of the flat path against the `header_image` the store API reports:

| App ID | Name | Flat path | Hash segment |
| --- | --- | --- | --- |
| 620 | Portal 2 | 200 | no |
| 730 | Counter-Strike 2 | 200 | yes |
| 1091500 | Cyberpunk 2077 | 200 | yes |
| 499170 | Order of the Sinking Star (full game) | 404 | yes |
| 2244210 | Echoes of Aincrad (full game) | 404 | yes |
| 3527290 | recent app | 404 | yes, file is `header_alt_assets_3.jpg` |

Two facts follow:

1. Full games fail as well. Every cover download is at risk.
2. The filename is not always `header.jpg`. App 3527290 uses
   `header_alt_assets_3.jpg`, so a rebuilt path is wrong even with the hash.

## Fix

Take the URL from the store API. Never rebuild it from the app ID.

`downloadAppName` (`steam_client.go:475`) already requests
`https://store.steampowered.com/api/appdetails` with `filters=basic`. That
response already holds `header_image` and `capsule_image`. The
`steamAppDetails` struct (`steam_client.go:103`) does not read those fields
yet.

Steps:

1. Add `HeaderImage` to `steamAppDetails.Data`, read from `header_image`.
2. Make the name lookup and the cover lookup share one request and one cache
   entry. The current key `steam-appname-<id>` holds a bare name, so it needs a
   new shape, or a second key holds the cover URL.
3. Pass the stored URL to `DownloadGameCover`. Use it verbatim.
4. Keep a fallback for an app the store API does not answer for. A non-Steam
   shortcut has no store app, and `success: false` is a valid answer.
5. Do not lose the miss cache. A failed request says nothing about the app ID
   and must not be stored as a miss. `steamAppNameMissTTL` covers this today.

## Notes on scope

- `library_600x900.jpg`, `capsule_616x353.jpg` and `logo.png` on the flat path
  are 404 for the affected apps too. Only `header.jpg` is downloaded today, so
  this task covers the header alone.
- `steam_client_test.go` has a fake `appDetailsURL`. The new field can be
  tested through that same hook.

---

## Notes

- 2026-09-06T11:17:32+02:00 — Implemented. The store URL is now read from header_image and used verbatim.

  Measured facts that shaped the design:
  - header_image is present whenever success is true. Checked 620, 400, 220, 570, 105600, 2246340, 3527290, 4148250, 4597250.
  - appdetails takes one app ID only. appids=620,570,4148250 answers null, so a batch is not possible.
  - appdetails refuses with 429 after about 200 requests in 5 minutes. Measured: 149 of 200 requests answered, then 51 refusals, inside 76 seconds.

  The legacy CDN path is therefore kept and tried first. It is not needed for
  correctness, but it costs nothing, and without it every game would need one
  store request. A library over about 200 games would be cut off part way
  through its first run. The legacy path also serves a cover for an app the
  store refuses: app 12345 answers success=false, and the legacy path returns a
  real 460x215 header.

  Changes:
  - steamAppDetails.Data gains HeaderImage, read from header_image.
  - steamAppInfo{Name, CoverURL} is one cache entry under steam-appinfo-<id>.
    The name lookup and the cover lookup share one request.
  - lookupAppName becomes lookupApp, downloadAppName becomes downloadAppInfo.
  - steamAppNameTTL and steamAppNameMissTTL become steamAppInfoTTL and
    steamAppInfoMissTTL. The miss TTL still guards a failed request from being
    cached as a miss.
  - A cached entry that will not parse is treated as a miss, so an entry written
    by an older version repairs itself.
  - SteamClient gains gameHeaderURL, so a test can point the legacy path at a
    local server.

  Verified against the live Steam service: 4148250, 4597250, 620, 3527290 and
  499170 all download a cover. Five new tests cover the ladder, the shared
  request and the old cache format. go vet and gofmt are clean.

  golangci-lint could not run. It panics inside staticcheck while analysing the
  Go standard library, which is a tool and toolchain mismatch rather than
  anything in this change.
- 2026-09-06T11:35:07+02:00 — Code review found two problems in this change. Both are fixed.

  1. The dead legacy path was probed on every run. A 404 was never remembered,
     so an app on the per-revision scheme paid one wasted request per run, even
     with the store URL already cached. DownloadGameCover now tries a cached
     cover URL first, then the legacy path, then a fresh store lookup. Verified
     live: a second run downloads all six covers and asks the store zero times.

  2. The 'older cache format' comment claimed an upgrade path the new cache key
     never reaches. A steam-appname-<id> entry is never read, so it just expires.
     The comment now says what is true: a damaged entry is a miss. The test is
     renamed to TestSteamClient_ADamagedCacheEntryIsTreatedAsAMiss and feeds it
     an unparsable entry.

  lookupApp is split into cachedApp (cache only) and refreshApp (request, then
  write). That split is what lets DownloadGameCover read the cache without
  triggering a request, and ask the store without reading a stale answer back.

  A cached record naming no cover URL is a known miss, so the store is not asked
  again inside its TTL. A cached URL that answers 404 does trigger a refresh, so
  replaced art repairs itself rather than failing for 30 days.

  Three tests added: DownloadGameCoverPrefersACachedURL,
  DownloadGameCoverDoesNotReAskAboutAKnownMiss and
  DownloadGameCoverRefreshesAStaleCachedURL.
