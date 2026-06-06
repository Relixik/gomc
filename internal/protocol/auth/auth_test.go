package auth

import (
	"context"
	crand "crypto/rand"
	"crypto/rsa"
	"crypto/sha1" //nolint:gosec // test vectors hash literal strings
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServerHashVectors(t *testing.T) {
	// The canonical wiki.vg server-hash vectors: Notch is positive, jeb_ is
	// negative (leading '-'), simon exercises leading-zero stripping.
	cases := map[string]string{
		"Notch": "4ed1f46bbe04bc756bcb17c0c7ce3e4632f06a48",
		"jeb_":  "-7c9d5b0044c130109a5d7b5fb5c317c02b4e28c1",
		"simon": "88e16a1019277b15d58faf0541e11910eb756f6",
	}
	for in, want := range cases {
		sum := sha1.Sum([]byte(in)) //nolint:gosec
		if got := minecraftDigest(sum[:]); got != want {
			t.Errorf("digest(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestOfflineUUID(t *testing.T) {
	u := OfflineUUID("Notch")
	if v := u[6] >> 4; v != 3 {
		t.Errorf("version nibble = %d, want 3", v)
	}
	if v := u[8] >> 6; v != 2 {
		t.Errorf("variant bits = %d, want 0b10 (RFC 4122)", v)
	}
	if OfflineUUID("Notch") != u {
		t.Error("OfflineUUID is not deterministic")
	}
	if OfflineUUID("jeb_") == u {
		t.Error("different names produced the same UUID")
	}
}

func TestRSADecryptRoundTrip(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	if len(kp.PublicDER) == 0 {
		t.Fatal("empty PublicDER")
	}
	secret := []byte("0123456789abcdef") // 16-byte shared secret
	ct, err := rsa.EncryptPKCS1v15(crand.Reader, &kp.Private.PublicKey, secret)
	if err != nil {
		t.Fatal(err)
	}
	pt, err := kp.Decrypt(ct)
	if err != nil {
		t.Fatal(err)
	}
	if string(pt) != string(secret) {
		t.Errorf("decrypt = %q, want %q", pt, secret)
	}
}

func TestHasJoined(t *testing.T) {
	t.Run("authenticated", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"069a79f444e94726a5befca90e38aaf5","name":"Notch","properties":[]}`))
		}))
		defer srv.Close()
		restore := swapURL(srv.URL)
		defer restore()

		p, err := HasJoined(context.Background(), "Notch", "deadbeef")
		if err != nil {
			t.Fatal(err)
		}
		if p.Name != "Notch" {
			t.Errorf("name = %q", p.Name)
		}
		if _, err := p.UUID(); err != nil {
			t.Errorf("UUID parse: %v", err)
		}
	})

	t.Run("not authenticated", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()
		restore := swapURL(srv.URL)
		defer restore()

		if _, err := HasJoined(context.Background(), "Nobody", "h"); err != ErrNotAuthenticated {
			t.Errorf("err = %v, want ErrNotAuthenticated", err)
		}
	})
}

func swapURL(u string) func() {
	old := sessionHasJoinedURL
	sessionHasJoinedURL = u
	return func() { sessionHasJoinedURL = old }
}
