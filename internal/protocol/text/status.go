package text

// StatusResponse is the JSON document a server returns to a Status-state
// Request (the multiplayer server-list ping).
type StatusResponse struct {
	Version            StatusVersion `json:"version"`
	Players            StatusPlayers `json:"players"`
	Description        Component     `json:"description"`
	Favicon            string        `json:"favicon,omitempty"` // "data:image/png;base64,..."
	EnforcesSecureChat bool          `json:"enforcesSecureChat"`
}

// StatusVersion identifies the protocol the server speaks. If Protocol does not
// match the client's, the client shows an incompatible-version notice.
type StatusVersion struct {
	Name     string `json:"name"`
	Protocol int    `json:"protocol"`
}

// StatusPlayers carries the online/max counts and an optional hover sample.
type StatusPlayers struct {
	Max    int                  `json:"max"`
	Online int                  `json:"online"`
	Sample []StatusPlayerSample `json:"sample,omitempty"`
}

// StatusPlayerSample is one entry of the hover-over player list.
type StatusPlayerSample struct {
	Name string `json:"name"`
	ID   string `json:"id"` // UUID string
}
