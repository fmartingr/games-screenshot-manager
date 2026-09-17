---
id: TQ-0012
title: 'Doctor: report the platforms each provider finds its own path on'
status: done
priority: normal
labels:
  - component/cli
created: 2026-09-17T09:03:57+02:00
updated: 2026-09-17T09:03:57+02:00
---

## Problem

`checkProvider` reported "enabled, and it finds its own path" for every
provider whose `path` is `auto`, whatever the host was. Most providers find a
path on some platforms only, so the report passed a provider that collects
nothing.

Measured on Linux, with `global.enabled = true` and no path set: guild_wars_2,
world_of_warcraft and diablo_4 all read `ok`, and all three fail at run time.

## Fix

`doctorProvider` carries `autoPlatforms`, the values of `runtime.GOOS` where the
provider finds its own path. The field is read only where `autoPath` is set, and
an empty list means every platform.

| Provider | Finds its own path on | Source |
| --- | --- | --- |
| steam | darwin, linux, windows | `getSteamBasePath` |
| guild_wars_2 | windows | `getScreenshotsPath` |
| world_of_warcraft | darwin, windows | `getScreenshotsPath` |
| minecraft | darwin, linux, windows | `getScreenshotsPaths` |
| diablo_4 | windows | `getScreenshotsPaths` |
| hytale | darwin, linux | `getScreenshotsPath` |
| nintendo_switch_2 | linux | `findUSBDevice`, which reads sysfs |
| xbox_game_bar | windows | the registry guard |
| playstation4, playstation5 | none, `autoPath` is false | |

A provider that finds no path on the host is a warning, not a failure: a path in
the config still works, so the run is not broken.

---

## Notes

- 2026-09-17T09:03:57+02:00 — Done together with the Diablo IV provider, which is what found the hole.

  Each list was read off the provider's own path function, not guessed. The Switch
  2 one is linux because findUSBDevice reads /sys/bus/usb/devices, which is Linux
  only. A path in the config reads an album folder on any platform, and that path
  skips the check.

  xbox_game_bar keeps its own guard, which says the registry drops it on any other
  platform. That guard runs after checkProvider, so it overrides the path line with
  the stronger message. Its autoPlatforms is set as well, so the declaration is
  true if that guard ever goes.

  The message is provider-neutral: 'enabled, but this host runs linux, where it
  finds no path of its own'.

  Verified on Linux with every provider enabled: steam, minecraft, hytale and
  nintendo_switch_2 read ok, guild_wars_2, world_of_warcraft and diablo_4 read
  warn, and xbox_game_bar keeps its own message.

  Three tests added in doctor_test.go: TestDoctorProvider_FindsPathHere,
  TestRunDoctor_WarnsWhereAProviderFindsNoPath and
  TestRunDoctor_TakesAPathWhereAProviderFindsNone. go vet, gofmt and go test are
  clean.
