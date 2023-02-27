package hybrid

import (
	"github.com/DogeProtocol/dp/crypto/signaturealgorithm"
	"testing"
)

func TestHybridSig_Basic(t *testing.T) {

	var sig signaturealgorithm.SignatureAlgorithm
	sig = CreateHybridSig()

	signaturealgorithm.SignatureAlgorithmTest(t, sig)
}
