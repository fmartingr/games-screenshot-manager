---
id: TQ-0008
title: The doctor creates the cache directory, contrary to its own contract
status: done
priority: low
labels:
  - bug
  - component/cli
created: 2026-09-06T11:39:59+02:00
updated: 2026-09-06T11:46:39+02:00
---

## Problem

`RunDoctor`'s doc comment says "It writes no file, it creates no directory."
It does create one.

`providerRequirements` (`doctor.go:326`) builds the registry to ask each
provider what it needs. That builds `SteamProvider`, which calls
`NewSteamClient`, which calls `newFileCache`, which runs `os.MkdirAll` on
`<user cache>/games-screenshot-manager` (`cache.go:48`).

## Fix

Either make the cache directory lazy, so it is created on the first write
rather than in the constructor, or correct the contract in the comment. The
lazy option keeps the promise the doctor makes.

---

## Notes

- 2026-09-06T11:46:39+02:00 — Fixed. The cache directory is made lazily.

  newFileCache now only names the path. fileCache.Set runs os.MkdirAll before it
  writes, which is where the directory is needed: writeFileAtomic puts its
  temporary file in that same directory. Get needs no directory, because a
  missing metadata file is already a miss.

  RunDoctor therefore keeps its contract. The Steam client it builds to ask for
  requirements no longer creates anything.

  Three cache tests added: NewFileCache_CreatesNoDirectory,
  FileCache_GetIsAMissWithoutTheDirectory and FileCache_SetCreatesTheDirectory.
  One doctor test added: RunDoctor_CreatesNoDirectory.

  The doctor test needed a Steam API key in its config. Without one the provider
  refuses to start, so NewSteamClient is never reached and the test passed even
  with the bug in place. Confirmed both tests fail with the MkdirAll put back in
  the constructor.

  Verified with the real binary: a doctor run against XDG_CACHE_HOME pointing at
  a path that does not exist leaves that path absent.
