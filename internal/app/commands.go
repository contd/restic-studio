package app

import (
	"fmt"
	"sort"
	"strings"

	"github.com/restic-studio/restic-studio/internal/config"
)

type Command struct {
	Program string
	Args    []string
	Env     map[string]string
	Secrets []string
}

func BackupCommand(cfg *config.Config, locationName string) (Command, error) {
	location, ok := cfg.Locations[locationName]
	if !ok {
		return Command{}, fmt.Errorf("location %q does not exist", locationName)
	}
	env, secrets, err := backendEnvironment(cfg, location.To, len(location.To) == 0)
	if err != nil {
		return Command{}, err
	}
	if cfg.Path == "" {
		return Command{}, fmt.Errorf("configuration path is empty")
	}
	return Command{
		Program: "autorestic",
		Args:    []string{"-c", cfg.Path, "-l", locationName, "backup"},
		Env:     env,
		Secrets: secrets,
	}, nil
}

func ResticCommand(backendName string, backend config.Backend, resticArgs ...string) (Command, error) {
	if strings.TrimSpace(backend.Path) == "" {
		return Command{}, fmt.Errorf("backend %q is missing repository path", backendName)
	}
	repository := backend.Path
	if backend.Type != "" && backend.Type != "local" && !strings.Contains(repository, ":") {
		repository = backend.Type + ":" + repository
	}
	env := make(map[string]string, len(backend.Env)+1)
	secrets := make([]string, 0, len(backend.Env)+1)
	for key, value := range backend.Env {
		env[key] = value
		if value != "" {
			secrets = append(secrets, value)
		}
	}
	if backend.Key != "" {
		env["RESTIC_PASSWORD"] = backend.Key
		secrets = append(secrets, backend.Key)
	}
	sort.Strings(secrets)
	args := []string{"-r", repository}
	args = append(args, resticArgs...)
	return Command{Program: "restic", Args: args, Env: env, Secrets: secrets}, nil
}

func backendEnvironment(cfg *config.Config, requested []string, useAll bool) (map[string]string, []string, error) {
	names := requested
	if useAll {
		names = cfg.BackendNames()
	}
	env := make(map[string]string)
	secrets := make([]string, 0)
	for _, name := range names {
		backend, ok := cfg.Backends[name]
		if !ok {
			return nil, nil, fmt.Errorf("location references undefined backend %q", name)
		}
		if backend.Key != "" {
			secrets = append(secrets, backend.Key)
		}
		for key, value := range backend.Env {
			if existing, exists := env[key]; exists && existing != value {
				return nil, nil, fmt.Errorf("backends set conflicting values for environment variable %s", key)
			}
			env[key] = value
			if value != "" {
				secrets = append(secrets, value)
			}
		}
	}
	sort.Strings(secrets)
	return env, secrets, nil
}
