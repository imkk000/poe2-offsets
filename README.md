# Go PoE2 Offsets

## Purpose

Just for fun. I am an active PoE2 player, and I wanted to build my own quality-of-life tools. This is my work with the Claude CLI: a work-in-progress effort to reverse engineer the game client in my free time. Because GGG ships new versions regularly, some offsets here will become obsolete after a patch, and a few may already be wrong or shifted — that is part of the discovery.

I play PoE2 on Linux through Proton (Wine). The game is still a Windows binary, so its struct layout is the Windows/MSVC one — but Wine runs it as an ordinary Linux process, so the memory is readable with native Linux APIs (`process_vm_readv`) rather than Win32.

## What this is

The deliverable is the reverse-engineered **offsets** of the live PoE2 client — vtables, struct field offsets, and pointer chains.

`gamestate` is the reference reader built on top of them: read-only walkers that turn those offsets into live game state — your character (life, mana, buffs, skills), nearby entities (monsters, ground items), inventory and stash, area and terrain, and the UI/HUD. It has no globals and no logging, and each reader resolves its component by name, so it survives vtable drift between patches — though individual field offsets still need re-anchoring after a patch.

## Usage

```go
import gamestate "github.com/imkk000/poe2-offsets"
```

## Related projects

- <https://github.com/imkk000/mh> — cross-platform process API
- <https://github.com/imkk000/go-protonhax> — communicate with Proton (Wine)
- <https://github.com/imkk000/go-offset-scanner> - simd memory pattern scanner

## License

Free to use however you like — no warranty. It's a casual hobby project.
