package packet

import "github.com/Relixik/gomc/internal/protocol/codec"

const (
	idLoginStart         = 0x00 // serverbound
	idEncryptionResponse = 0x01 // serverbound
	idLoginAcknowledged  = 0x03 // serverbound

	idEncryptionRequest = 0x01 // clientbound
	idLoginSuccess      = 0x02 // clientbound
	idSetCompression    = 0x03 // clientbound
)

func init() {
	registerServerbound(StateLogin, idLoginStart, func() Decoder { return &LoginStart{} })
	registerServerbound(StateLogin, idEncryptionResponse, func() Decoder { return &EncryptionResponse{} })
	registerServerbound(StateLogin, idLoginAcknowledged, func() Decoder { return &LoginAcknowledged{} })
}

// LoginStart begins login: the desired name and the client's UUID (which the
// server replaces with the offline UUID in offline mode). (Login, sb, 0x00.)
type LoginStart struct {
	Name string
	UUID codec.UUID
}

func (p *LoginStart) Decode(r *codec.Reader) {
	p.Name = r.String(16)
	p.UUID = r.UUID()
}

// LoginAcknowledged is sent by the client after LoginSuccess; it moves the
// connection into the Configuration state. (Login, sb, 0x03.)
type LoginAcknowledged struct{}

func (p *LoginAcknowledged) Decode(*codec.Reader) {}

// EncryptionRequest asks the client to start AES encryption. It carries the
// server's RSA public key (X.509/PKIX DER, byte-for-byte Java's
// PublicKey.getEncoded()) and a verify token the client must echo back
// (RSA-encrypted) to prove it controls the session. ServerID is empty since 1.7
// but still hashed into the server id. ShouldAuthenticate (1.20.5+) tells the
// client whether the server will verify the join with Mojang. (Login, cb, 0x01.)
type EncryptionRequest struct {
	ServerID           string
	PublicKey          []byte // PKIX DER
	VerifyToken        []byte
	ShouldAuthenticate bool
}

func (p *EncryptionRequest) ID() int32 { return idEncryptionRequest }

func (p *EncryptionRequest) Encode(w *codec.Writer) {
	w.String(p.ServerID)
	w.ByteArray(p.PublicKey)
	w.ByteArray(p.VerifyToken)
	w.Bool(p.ShouldAuthenticate)
}

// EncryptionResponse carries the client's RSA/PKCS#1v1.5-encrypted shared secret
// (the 16-byte AES key) and the RSA-encrypted verify token echoed back from
// EncryptionRequest. (Login, sb, 0x01.)
type EncryptionResponse struct {
	SharedSecret []byte
	VerifyToken  []byte
}

func (p *EncryptionResponse) Decode(r *codec.Reader) {
	p.SharedSecret = r.ByteArray()
	p.VerifyToken = r.ByteArray()
}

// SetCompression enables zlib packet compression in both directions for all
// subsequent packets: frames whose body is at least Threshold bytes are
// compressed (a negative threshold disables it). It must be sent before
// LoginSuccess, and LoginSuccess is the first packet under the new framing.
// (Login, cb, 0x03.)
type SetCompression struct {
	Threshold int32
}

func (p *SetCompression) ID() int32 { return idSetCompression }

func (p *SetCompression) Encode(w *codec.Writer) {
	w.VarInt(p.Threshold)
}

// LoginProperty is a signed profile property (e.g. "textures").
type LoginProperty struct {
	Name      string
	Value     string
	Signature string // empty => unsigned
}

// LoginSuccess confirms login and carries the authoritative profile. (Login,
// cb, 0x02.)
type LoginSuccess struct {
	UUID       codec.UUID
	Name       string
	Properties []LoginProperty
}

func (p *LoginSuccess) ID() int32 { return idLoginSuccess }

func (p *LoginSuccess) Encode(w *codec.Writer) {
	w.UUID(p.UUID)
	w.String(p.Name)
	w.VarInt(int32(len(p.Properties)))
	for _, prop := range p.Properties {
		w.String(prop.Name)
		w.String(prop.Value)
		if prop.Signature != "" {
			w.Bool(true)
			w.String(prop.Signature)
		} else {
			w.Bool(false)
		}
	}
}
