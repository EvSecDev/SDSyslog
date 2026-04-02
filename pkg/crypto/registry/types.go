// Central tracking of protocol's crypto suites and signature suites (Mapping wire protocol IDs to info and functions)
package registry

type NewKeyFunc func() (privateKey, publicKey []byte, err error)
type ValidateKeyFunc func([]byte) (err error)

type DerivcePublicKeyFunc func(privateKey []byte) (publicKey []byte, err error)
type EncryptFunc func(publicKey, payload []byte) (ciphertext, ephemeralPub, nonce []byte, err error)
type DecryptFunc func(privateKey, ciphertext, ephemeralPub, nonce []byte) (payload []byte, err error)

type SuiteInfo struct {
	Name            string
	KeySize         int
	NonceSize       int
	CipherOverhead  int
	ValidateKey     ValidateKeyFunc
	NewKey          NewKeyFunc
	DerivePublicKey DerivcePublicKeyFunc
	Encrypt         EncryptFunc
	Decrypt         DecryptFunc
}

type SignFunc func(privateKey []byte, message []byte) (sig []byte, err error)
type VerifyFunc func(publicKey []byte, message []byte, sig []byte) (valid bool)

type SigInfo struct {
	Name               string
	MinSignatureLength int
	MaxSignatureLength int
	ValidateKey        ValidateKeyFunc
	NewKey             NewKeyFunc
	Sign               SignFunc
	Verify             VerifyFunc
}
