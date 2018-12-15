package osutil

import (
	"os"
)

func Getenv(name, def string) string {
	if env := os.Getenv(name); env != "" {
		return env
	}
	return def
}
