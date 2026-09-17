---
id: TQ-0013
title: World of Warcraft provider collects nothing
status: done
priority: high
labels:
  - bug
  - component/backend
created: 2026-09-17T09:06:56+02:00
updated: 2026-09-17T09:07:47+02:00
---

## Problem

`WorldOfWarcraftProvider.GetScreenshots` builds a `Game`, reads the folder,
fills `game.Screenshots`, and returns without ever calling
`p.gameManager.AddGame(game)`. The game is dropped on the floor.

`worldofwarcraft.go:98`

Nothing downstream ever sees it, so an enabled provider copies no file and
writes no folder. The run reports no error, so the failure is silent.

Every other provider adds the game. Guild Wars 2 and Hytale guard the call with
`len(game.Screenshots) > 0`, so a folder that holds nothing makes no empty game
folder in the output.

## Found by

A code review of the Diablo IV provider, which was modeled on this one. The new
provider calls `AddGame` and is not affected.

## Fix

Add the call, with the same guard the other providers use.

---

## Notes

- 2026-09-17T09:07:47+02:00 — Fixed. The AddGame call was added, guarded by len(game.Screenshots) > 0, which is what Guild Wars 2 and Hytale do.

  The guard matters: without it an enabled provider whose folder holds nothing
  would write an empty 'PC/World of Warcraft' folder into the output.

  Confirmed the bug was real before the fix. TestWorldOfWarcraftAddsTheGame and
  TestWorldOfWarcraftSkipsANameItCannotRead both fail against the old code with
  'GetScreenshots() added no game'.

  Verified end to end against a fixture of two screenshots:
    PC/World of Warcraft/2026-03-14_09-05-01.jpg
    PC/World of Warcraft/2026-04-01_11-22-33.jpg

  worldofwarcraft_test.go is a new file with three tests: the game reaches the
  GameManager, an empty folder adds no game, and a name the provider cannot read
  is skipped rather than fatal.

  Not touched, and still true of this provider:
  - The file test is strings.Contains(name, '.jpg'), so a name that merely holds
    '.jpg' matches. A directory is not skipped either.
  - A missing folder returns an error rather than being skipped, which is not what
    Hytale and Diablo IV do.
  Neither is the reported bug, so both are left as they are.

  go vet, gofmt and go test are clean.
