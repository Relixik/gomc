// Package text builds Minecraft text components ("chat components").
//
// The primary on-wire form is NBT (a String tag for plain text, a Compound for
// styled text), built on package nbt. A legacy length-prefixed JSON form is
// also provided for the few places that still use it (e.g. the Login-state
// Disconnect packet uses a JSON string, whereas Configuration/Play Disconnect
// use NBT). The Status (server list) Response JSON struct also lives here.
//
// Stdlib only: encoding/json (JSON form + status) and package nbt (NBT form).
package text
