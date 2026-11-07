// Package config reads settings from the environment. Locally they come from
// backend/.env (never committed); on DigitalOcean they are encrypted app env vars.
package config

import (
	"bufio"
	"os"
	"strings"
)

// LoadDotEnv sets every KEY=value in path that the environment doesn't
// already have. A missing file is fine (production has no .env).
func LoadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if _, set := os.LookupEnv(k); !set {
			os.Setenv(strings.TrimSpace(k), strings.TrimSpace(v))
		}
	}
}
