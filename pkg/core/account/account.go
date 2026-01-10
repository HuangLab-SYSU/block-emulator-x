package account

// Address is defined as a byte array whose size is fixed to 20.
type Address [20]byte

// PublicKey is defined as a byte array whose size is fixed to 64.
type PublicKey [64]byte

type Account struct {
	Addr Address
	PKey PublicKey
}

func (a *Account) Encode() ([]byte, error) {
	return append(a.Addr[:], a.PKey[:]...), nil
}
