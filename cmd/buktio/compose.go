package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// composeDir locates the directory holding docker-compose.yml. Override with
// BUKTIO_COMPOSE_DIR or the global -C flag (env wins if set).
func composeDir() string {
	if d := os.Getenv("BUKTIO_COMPOSE_DIR"); d != "" {
		return d
	}
	for _, c := range []string{"deploy/docker-compose", "."} {
		if _, err := os.Stat(filepath.Join(c, "docker-compose.yml")); err == nil {
			return c
		}
	}
	return "deploy/docker-compose"
}

// compose builds a `docker compose ...` command rooted at the compose dir.
func compose(args ...string) *exec.Cmd {
	c := exec.Command("docker", append([]string{"compose"}, args...)...)
	c.Dir = composeDir()
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = os.Stdin
	return c
}

// dotenv reads KEY=VALUE pairs from the compose dir's .env (best-effort).
func dotenv() map[string]string {
	out := map[string]string{}
	f, err := os.Open(filepath.Join(composeDir(), ".env"))
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if k, v, ok := strings.Cut(line, "="); ok {
			out[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return out
}

// setDotenvKey updates or appends KEY=VALUE in the compose dir's .env.
func setDotenvKey(key, value string) error {
	path := filepath.Join(composeDir(), ".env")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	found := false
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), key+"=") {
			lines[i] = key + "=" + value
			found = true
			break
		}
	}
	if !found {
		lines = append(lines, key+"="+value)
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o600)
}

// run executes a command, returning a wrapped error on failure.
func run(c *exec.Cmd) error {
	if err := c.Run(); err != nil {
		return fmt.Errorf("%s: %w", strings.Join(c.Args, " "), err)
	}
	return nil
}
