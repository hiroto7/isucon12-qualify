package isuports

import (
	"sync"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

type signedTokenKey struct{ keyFilename, raw string }

var signedTokens = struct {
	sync.RWMutex
	values map[signedTokenKey]jwt.Token
}{values: make(map[signedTokenKey]jwt.Token)}

// A cached token is immutable and proved signed by this fixed public key.
// Time-dependent validation still runs on every request, including cache hits.
func verifiedJWT(raw, keyFilename string, key interface{}, now time.Time) (jwt.Token, error) {
	cacheKey := signedTokenKey{keyFilename, raw}
	signedTokens.RLock()
	token := signedTokens.values[cacheKey]
	signedTokens.RUnlock()
	validation := jwt.WithClock(jwt.ClockFunc(func() time.Time { return now }))
	if token != nil {
		if err := jwt.Validate(token, validation); err != nil {
			return nil, err
		}
		return token, nil
	}
	token, err := jwt.Parse([]byte(raw), jwt.WithKey(jwa.RS256, key), jwt.WithValidate(false))
	if err != nil {
		return nil, err
	}
	if err := jwt.Validate(token, validation); err != nil {
		return nil, err
	}
	signedTokens.Lock()
	// Bound retained credentials. Eviction only causes another signature check.
	if len(signedTokens.values) >= 32768 {
		signedTokens.values = make(map[signedTokenKey]jwt.Token)
	}
	signedTokens.values[cacheKey] = token
	signedTokens.Unlock()
	return token, nil
}
