package app

import (
	"reflect"
	"testing"

	"github.com/restic-studio/restic-studio/internal/config"
)

func TestResticCommandBuildsRepositoryAndCredentials(t *testing.T) {
	backend := config.Backend{
		Type: "s3",
		Path: "bucket.example/restic",
		Key:  "repository-password",
		Env:  map[string]string{"AWS_ACCESS_KEY_ID": "access-secret"},
	}
	command, err := ResticCommand("backup", backend, "snapshots", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"-r", "s3:bucket.example/restic", "snapshots", "--json"}; !reflect.DeepEqual(command.Args, want) {
		t.Fatalf("args = %#v, want %#v", command.Args, want)
	}
	if command.Env["RESTIC_PASSWORD"] != "repository-password" || len(command.Secrets) != 2 {
		t.Fatalf("credentials not attached or redacted: %#v", command)
	}
}

func TestBackupCommandRejectsConflictingBackendEnvironment(t *testing.T) {
	cfg := &config.Config{
		Path:      "/tmp/autorestic.yml",
		Locations: map[string]config.Location{"home": {From: config.StringList{"/home"}, To: config.StringList{"a", "b"}}},
		Backends: map[string]config.Backend{
			"a": {Path: "/a", Env: map[string]string{"TOKEN": "first"}},
			"b": {Path: "/b", Env: map[string]string{"TOKEN": "second"}},
		},
	}
	_, err := BackupCommand(cfg, "home")
	if err == nil || err.Error() != "backends set conflicting values for environment variable TOKEN" {
		t.Fatalf("BackupCommand() error = %v", err)
	}
}
