package registry

import (
	_ "embed"
	"encoding/json"

	"github.com/Relixik/gomc/internal/protocol/packet"
)

// registriesJSON is the exact set, order, and entry identifiers of the
// synchronised registries a vanilla 26.1.2 server sends during the
// Configuration phase — captured from the reference server (see .mcref/probe.go
// and WORKFLOW.md §3). All entries are sent with has_data=false, relying on the
// shared "minecraft:core" known pack, which is exactly what vanilla does.
//
//go:embed registries.json
var registriesJSON []byte

// tagsData is the captured vanilla 26.1.2 Update Tags packet payload — the tag
// definitions for the synced registries (enchantment exclusive_set, dialog,
// timeline, item/block tags, etc.). The client REQUIRES these or registry
// loading fails ("Unbound tags"). The tags reference registry entries by
// numeric id, which are valid here because our registry order matches vanilla's.
//
//go:embed tags.bin
var tagsData []byte

// Tags returns the Update Tags packet payload to send during Configuration.
func Tags() []byte { return tagsData }

type rawRegistry struct {
	Registry string   `json:"registry"`
	Entries  []string `json:"entries"`
}

var synced []*packet.RegistryData

func init() {
	var raw []rawRegistry
	if err := json.Unmarshal(registriesJSON, &raw); err != nil {
		// Compile-time embedded data; a parse failure is a build/programmer bug.
		panic("registry: invalid embedded registries.json: " + err.Error())
	}
	synced = make([]*packet.RegistryData, 0, len(raw))
	for _, r := range raw {
		entries := make([]packet.RegistryEntry, len(r.Entries))
		for i, id := range r.Entries {
			entries[i] = packet.RegistryEntry{ID: id} // Data nil => has_data=false
		}
		synced = append(synced, &packet.RegistryData{Registry: r.Registry, Entries: entries})
	}
}

// SyncedData returns the Registry Data packets to send during Configuration, in
// vanilla order, each entry with has_data=false (the client fills the content
// from its matching core pack).
func SyncedData() []*packet.RegistryData { return synced }

// Count reports how many registries are synchronised.
func Count() int { return len(synced) }
