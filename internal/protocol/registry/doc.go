// Package registry is the Configuration-phase data provider — the biggest delta
// versus the archived old/ server, which bundled dimension data into Join Game.
// From 1.20.2 onward the Configuration state is mandatory: no modern client can
// reach Play without it.
//
// It embeds the vanilla 26.1.2 datapack JSON (via go:embed), converts it to
// network NBT, and serves:
//
//   - Registry Data — one packet per registry, in deterministic order, because
//     entry order defines the numeric IDs referenced by later Play packets.
//     Must include dimension_type, the 24 damage_type entries, a biome named
//     exactly minecraft:plains, chat_type, painting_variant, wolf_variant, etc.
//   - Update Tags
//   - Known Packs negotiation (the server blocks until the client's Known Packs
//     arrive before it may omit per-entry NBT).
//
// Stdlib only: embed, encoding/json, package nbt.
package registry
