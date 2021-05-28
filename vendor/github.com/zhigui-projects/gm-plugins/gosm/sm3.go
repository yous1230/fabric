package gosm

import (
	"hash"

	"github.com/zhigui-projects/gm-go/sm3"
)

type GoSm3 struct{}

func (gs *GoSm3) NewSm3() hash.Hash {
	return sm3.New()
}
