package utils

import (
	"os"
	"path"
	"strings"
)

func ResolvePackagePath(packageLocalName string) string {
	gopathEnv := os.Getenv("GOPATH")
	gopaths := strings.Split(gopathEnv, ":")
	for _, gopath := range gopaths {
		p := path.Join(gopath, "src", packageLocalName)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
