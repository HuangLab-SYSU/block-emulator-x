package bloom

import (
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
)

var defaultFilterFs = []bfHashFunc{Sha256, Sha512, Sha1}

func Sha256(b []byte) []byte {
	h := sha256.Sum256(b)
	return h[:]
}

func Sha512(b []byte) []byte {
	h := sha512.Sum512(b)
	return h[:]
}

func Sha1(b []byte) []byte {
	h := sha1.Sum(b)
	return h[:]
}
