package envutil

import (
	"fmt"
	"os"
	"strings"
)

func Require(keys ...string) {
	var missing []string
	for _, key := range keys {
		if strings.TrimSpace(os.Getenv(key)) == "" {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		fmt.Fprintf(os.Stderr, "FATAL: missing required environment variables: %s\n", strings.Join(missing, ", "))
		os.Exit(1)
	}
}

func Optional(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
