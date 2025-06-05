package credentials

import "os"

func DefaultStoreDirectory() string {
	return os.TempDir() + "/credentials"
}
