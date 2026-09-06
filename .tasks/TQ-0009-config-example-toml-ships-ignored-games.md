---
id: TQ-0009
title: config.example.toml ships ignored_games uncommented
status: done
priority: normal
labels:
  - bug
  - component/config
created: 2026-09-06T11:39:59+02:00
updated: 2026-09-06T11:40:10+02:00
---

## Problem

`config.example.toml:130` ships this line live:

    ignored_games = ["4148250", "4597250"]

Anyone who copies the example silently skips two real Steam app IDs. Those
games get no folder, no screenshots and no cover, and nothing says why.

## Fix

Comment the line out, the way the neighbouring `custom_games` block shows a
sample without applying it.

---

## Notes

- 2026-09-06T11:40:10+02:00 — The line is commented out. The two app IDs were a sample, not a default, and the neighbouring custom_games block shows a sample the same way.
