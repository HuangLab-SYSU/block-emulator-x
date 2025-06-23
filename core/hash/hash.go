package hash

import "crypto/sha256"

type Hash [32]byte

// Encodable implements Encode, CalcHash will call Encode to calculate Hash
type Encodable interface {
	Encode() ([]byte, error)
}

// CalcHash calculate the hash of the encodedByte
func CalcHash(encodable Encodable) (Hash, error) {
	b, err := encodable.Encode()
	if err != nil {
		return Hash{}, err
	}
	return sha256.Sum256(b), nil
}
