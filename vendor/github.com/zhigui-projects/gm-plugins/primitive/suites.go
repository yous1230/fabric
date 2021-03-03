package primitive

type Context interface {
	KeysGenerator
	Sm2Crypto
	Sm3Crypro
	Sm4Crypro
}
