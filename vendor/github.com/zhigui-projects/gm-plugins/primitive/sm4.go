package primitive

import "crypto/cipher"

type Sm4Crypro interface {
	NewSm4Cipher(key []byte) (cipher.Block, error)
	Sm4BlockSize() int
}
