package utils

import "crypto/sha256"

// Encodable implements Encode, CalcHash will call Encode to calculate Hash
type Encodable interface {
	Encode() ([]byte, error)
}

// CalcHash calculate the hash of the encodedByte
func CalcHash(encodable Encodable) ([]byte, error) {
	b, err := encodable.Encode()
	if err != nil {
		return []byte{}, err
	}
	sum := sha256.Sum256(b)
	return sum[:], nil
}
