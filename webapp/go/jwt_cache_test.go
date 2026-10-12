package isuports

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

func TestVerifiedTokenCacheExpiryAndSignature(t *testing.T) {
	private, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	token := jwt.New()
	token.Set(jwt.SubjectKey, "p")
	token.Set(jwt.NotBeforeKey, now.Add(-time.Second))
	token.Set(jwt.ExpirationKey, now.Add(10*time.Second))
	raw, err := jwt.Sign(token, jwt.WithKey(jwa.RS256, private))
	if err != nil {
		t.Fatal(err)
	}
	name := t.TempDir() + "/key"
	for i := 0; i < 2; i++ {
		parsed, err := verifiedJWT(string(raw), name, &private.PublicKey, now)
		if err != nil || parsed.Subject() != "p" {
			t.Fatal(parsed, err)
		}
	}
	if _, err := verifiedJWT(string(raw), name, &private.PublicKey, now.Add(20*time.Second)); err == nil {
		t.Fatal("cached token remained valid after expiry")
	}
	if _, err := verifiedJWT(string(raw), name, &private.PublicKey, now.Add(-10*time.Second)); err == nil {
		t.Fatal("cached token ignored not-before")
	}
	pieces := strings.Split(string(raw), ".")
	pieces[1] = base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"forged"}`))
	if _, err := verifiedJWT(strings.Join(pieces, "."), name, &private.PublicKey, now); err == nil {
		t.Fatal("unverified payload accepted")
	}
}
