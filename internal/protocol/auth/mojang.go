package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/Relixik/gomc/internal/protocol/codec"
)

// sessionHasJoinedURL is a package var so tests can point it at a stub server.
var sessionHasJoinedURL = "https://sessionserver.mojang.com/session/minecraft/hasJoined"

// ErrNotAuthenticated is returned when the session server does not recognise the
// join (HTTP 204) — i.e. the player is not who they claim or never authenticated.
var ErrNotAuthenticated = errors.New("auth: session server did not authenticate the player")

// Profile is the authenticated game profile returned by the session server.
type Profile struct {
	ID         string     `json:"id"` // undashed 32-hex UUID
	Name       string     `json:"name"`
	Properties []Property `json:"properties"`
}

// Property is a signed profile property (notably "textures" for skins/capes).
type Property struct {
	Name      string `json:"name"`
	Value     string `json:"value"`
	Signature string `json:"signature,omitempty"`
}

// UUID parses the profile's undashed ID into a codec.UUID.
func (p *Profile) UUID() (codec.UUID, error) {
	return codec.ParseUUID(p.ID)
}

// HasJoined asks the Mojang session server to confirm an online-mode player has
// joined this server, identified by the server hash from ServerHash.
func HasJoined(ctx context.Context, username, serverHash string) (*Profile, error) {
	q := url.Values{}
	q.Set("username", username)
	q.Set("serverId", serverHash)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sessionHasJoinedURL+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusOK:
		var p Profile
		if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
			return nil, fmt.Errorf("auth: decode hasJoined response: %w", err)
		}
		return &p, nil
	case http.StatusNoContent:
		return nil, ErrNotAuthenticated
	default:
		return nil, fmt.Errorf("auth: hasJoined unexpected status %d", resp.StatusCode)
	}
}
