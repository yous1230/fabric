package primitive

import "hash"

type Sm3Crypro interface {
	NewSm3() hash.Hash
}
