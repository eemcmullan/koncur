package targets

import (
	"path/filepath"
	"strings"
)

// IsBinaryFile returns true if the path appears to be a binary artifact (.jar, .war, or .ear)
func IsBinaryFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".jar" || ext == ".war" || ext == ".ear"
}
