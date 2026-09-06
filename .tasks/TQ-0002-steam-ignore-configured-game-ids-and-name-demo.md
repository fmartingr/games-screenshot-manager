---
id: TQ-0002
title: 'Steam: ignore configured game IDs and name demo apps'
status: done
priority: normal
labels:
  - feature
  - component/config
  - provider/steam
  - component/backend
created: 2026-09-06T08:58:29+02:00
updated: 2026-09-06T09:39:27+02:00
---

## Problem

Two problems, one provider, one code path.

**1. Every app ID folder is taken.** The Steam provider takes every app ID
folder under `userdata/<user>/760/remote`. It also takes every published
screenshot the online gallery returns. Some of those app IDs are not wanted in
the output: a tool, a test build, a game the user does not want in the
gallery. There is no way to skip one today. `custom_games` only renames a
game. The game still gets a directory, a copy of every screenshot, and a
cover.

**2. A demo app gets no name.** `IStoreService/GetAppList/v1` does not return
an app of type `demo`. `resolveGameName` therefore returns an empty string,
and `GetScreenshots` falls back to the folder name. The gallery then shows a
raw app ID as the game title.

Measured on this machine, against the 9 app ID folders in `760/remote`:

| App ID | Store type | In `GetAppList` | Name |
| --- | --- | --- | --- |
| 4148250 | demo | no | Echoes of Aincrad DEMO Version |
| 4597250 | demo | no | Order of the Sinking Star Demo |
| the other 7 | game | yes | resolved today |

No query flag repairs this. `include_dlc`, `include_software`,
`include_videos`, `include_hardware` and a speculative `include_demos` were
each tried against both demo IDs. All returned nothing. The endpoint has no
switch for demos.

`https://store.steampowered.com/api/appdetails?appids=<id>&filters=basic`
does return both, with the name and `"type": "demo"`. It needs no API key.
`ISteamApps/GetAppList/v2`, the old public list, now answers 404.

## Design

### Ignore list

Add one key to `[providers.steam]`:

```toml
[providers.steam]
ignored_games = ["228980", "1234567890"]
```

The value is a list of Steam app IDs as strings. String is the type used
everywhere else for an app ID: the folder name on disk, the `custom_games`
keys, and `Game.ID`.

An ignored ID is dropped before the game reaches the `GameManager`. Nothing
downstream sees it. There is no output directory, no copy, no cover, and no
gallery entry.

### Name fallback

`GetGameName` gets a second source. The app list stays first. On a miss the
client asks the store `appdetails` endpoint for that one app ID, and caches
the answer.

The order of the two features matters. The ignore test runs before the name
lookup, so an ignored demo costs no request.

## Implementation

**config.go**

- `SteamConfig` gets `IgnoredGames []string` with the tag
  `toml:"ignored_games"`.

**steam.go**

- `SteamProvider` gets a field `ignoredGames map[string]struct{}`.
- `NewSteamProvider` builds that set from
  `config.Providers.Steam.IgnoredGames`. It trims each entry. It drops an
  empty entry. `p.steamConfig` is a copy of the config, so the set belongs on
  the provider and is built once.
- A method `isIgnored(gameID string) bool` reads the set.
- `GetScreenshots`: test `p.isIgnored(file.Name())` as the first statement of
  the folder loop. Log the skip. `continue` before `resolveGameName` and
  before `NewGame`.
- `GetPublishedScreenshots`: test the app ID after it is final. The `AppID ==
  0` branch resolves an ID from the shortcut name, so the test goes after that
  branch and before `resolveGameName`. `continue` on a match.
- `GetCovers` needs no test. It reads the games the `GameManager` holds, and
  an ignored game never gets there.

**steam_client.go**

- A new constant holds the store endpoint:
  `steamAppDetailsURL = "https://store.steampowered.com/api/appdetails"`.
  A field on `SteamClient` holds the base URL, so a test can point it at an
  `httptest` server.
- `GetGameName` keeps the app list as its first source. On an empty result it
  calls a new `lookupAppName(gameID string) string`.
- `lookupAppName` accepts a gameID of digits only. A folder name comes from
  disk, and the ID becomes part of a cache key, so anything else is refused
  without a request.
- The response is a map keyed by the app ID:
  `{"4148250": {"success": true, "data": {"type": "demo", "name": "..."}}}`.
  A `success` of false is a valid answer. It means the ID is not a store app.
- The result is cached under the key `steam-appname-<id>`. A name is held for
  30 days. An empty answer is held for 24 hours, so a folder that is not a
  store app does not hit the network on every run.
- A failed request never fails the run. Log it and return an empty string.
  The folder name is still the fallback, which is the behaviour of today.
- The endpoint is rate limited, near 200 requests per 5 minutes per IP. Only
  an ID the list does not name reaches it, and each answer is cached, so a
  normal run makes very few requests.

## Docs

- `config.example.toml`: put `ignored_games` in the `[providers.steam]` block,
  above the `[providers.steam.custom_games]` header. A bare key after a table
  header belongs to that table, so the order is part of the change.
- `README.md`: one line in the Steam section for the ignore list. One line for
  the demo names.

## Tests

- `config_test.go`: `ignored_games` parses into
  `config.Providers.Steam.IgnoredGames`.
- New `steam_test.go`: `isIgnored` returns true for a listed ID, false for an
  ID that is not listed, true for an entry with spaces around it, and false
  for both an empty list and a nil list.
- `steam_client_test.go`: build the client with `stubCache` and an `httptest`
  server, as the tests there already do.
  - A name in the app list returns without any request to the store.
  - An ID the list does not hold is resolved from the store response.
  - The resolved name is written to the cache.
  - A cached name answers without a request.
  - A response with `"success": false` returns an empty string.
  - A 500 or a broken body returns an empty string, and no error escapes.
  - A gameID that is not all digits makes no request.
- `GetScreenshots` gets no test. `getSteamBasePath` returns the platform path
  and ignores `userdata_path`, so a test cannot point the walk at a temporary
  tree. See the follow-up below.

## Acceptance

- `gofmt -l .` is empty.
- `go vet ./...` passes.
- `go test ./...` passes.
- A run with an ID in `ignored_games` creates no directory for that game.
- A run names 4148250 "Echoes of Aincrad DEMO Version" and 4597250 "Order of
  the Sinking Star Demo", rather than the app ID.

## Follow-up

`GetScreenshots` calls `getSteamBasePath` and never reads
`userdata_path`. The configured path has no effect on the disk walk. That is
a separate bug, and it is what blocks a test of the filter on the disk path.

---

## Notes

- 2026-09-06T09:08:29+02:00 — Probed the live Steam APIs from this machine to settle how a demo app is named.

  - The 9 folders in 760/remote hold 2 demo apps: 4148250 and 4597250.
  - IStoreService/GetAppList/v1 returns neither. Tried the default flags, then include_dlc, include_software, include_videos, include_hardware together, then include_demos. Each probe asked for last_appid=<id-1> and max_results=1, so the demo would have been the first result if the endpoint held it.
  - store.steampowered.com/api/appdetails?appids=<id>&filters=basic returns both, with type demo and the name. No API key needed.
  - ISteamApps/GetAppList/v2, the old public list, answers 404 now. It is not an option.

  So the fix is a per-ID fallback lookup, not a flag on the list request.
- 2026-09-06T09:39:27+02:00 — Implemented and verified.

  Code: config.go (IgnoredGames), steam.go (newIgnoredGamesSet, isIgnored, both call sites), steam_client.go (GetGameName falls back to lookupAppName/downloadAppName, cached under steam-appname-<id>).
  Docs: config.example.toml, README.md. Tests: steam_test.go (new), steam_client_test.go (7 new), config_test.go.

  gofmt -l is empty. go vet and go test ./... pass.

  Dry run against the real Steam data on this machine, with ignored_games = ["4597250", " 29720 "]:
  - 29720 skipped, which also proves the entry is trimmed.
  - 4597250 skipped.
  - 4148250 resolved from the store as "Echoes of Aincrad DEMO Version", type demo.
  - The second run read the name from the cache and made no request.
  - The dry run created no output directory.

  The change is in the working tree. It is not committed.
