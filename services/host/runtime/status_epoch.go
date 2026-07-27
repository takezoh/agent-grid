package runtime

import (
	"crypto/rand"
	"encoding/hex"
)

func newStatusEpoch() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic("runtime: cannot create status epoch: " + err.Error())
	}
	return hex.EncodeToString(value[:])
}
