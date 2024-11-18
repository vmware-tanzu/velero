//go:build goexperiment.boringcrypto

package version

import "crypto/boring"

func init() {
	fipsEnabled = boring.Enabled()
}