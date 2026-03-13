// Central tracking of protocol's crypto suites and signature suites (Mapping wire protocol IDs to info and functions)
package registry

type SuiteInfo struct {
	Name           string
	KeySize        int
	NonceSize      int
	CipherOverhead int
}

type NewKeyFunc func() (privateKey, publicKey []byte, err error)

type ValidateKeyFunc func([]byte) (err error)

type SignFunc func(privateKey []byte, message []byte) (sig []byte, err error)
type VerifyFunc func(publicKey []byte, message []byte, sig []byte) (valid bool)

type SigInfo struct {
	Name               string
	MinSignatureLength int
	MaxSignatureLength int
	ValidateKey        ValidateKeyFunc
	Sign               SignFunc
	Verify             VerifyFunc
	NewKey             NewKeyFunc
}
