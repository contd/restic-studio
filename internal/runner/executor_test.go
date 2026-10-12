package runner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestMergeEnvironment(t *testing.T) {
	got := MergeEnvironment([]string{"KEEP=value", "OVERRIDE=old", "BROKEN"}, map[string]string{"OVERRIDE": "new", "ADDED": "configured"})
	joined := strings.Join(got, "\n")
	for _, want := range []string{"KEEP=value", "OVERRIDE=new", "ADDED=configured"} {
		if !strings.Contains(joined, want) {
			t.Errorf("environment missing %q: %s", want, joined)
		}
	}
	if strings.Contains(joined, "BROKEN") || strings.Contains(joined, "OVERRIDE=old") {
		t.Fatalf("invalid or overridden value retained: %s", joined)
	}
}

func TestRunCapturesRedactedOutputAndExitCode(t *testing.T) {
	program, args, env := helperProcess(t, "fail", "secret-value")
	executor := New()
	result, err := executor.Run(context.Background(), program, args, env, []string{"secret-value"})
	if err == nil || !strings.Contains(err.Error(), "status 7") {
		t.Fatalf("Run() error = %v", err)
	}
	if result.ExitCode != 7 || strings.Contains(result.Output, "secret-value") || !strings.Contains(result.Output, "[REDACTED]") {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestRunCancellation(t *testing.T) {
	program, args, env := helperProcess(t, "sleep")
	executor := New()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := executor.Run(ctx, program, args, env, nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run() error = %v, want context deadline", err)
	}
}

func TestCheckMissingExecutable(t *testing.T) {
	executor := New()
	executor.LookupPath = func(string) (string, error) { return "", exec.ErrNotFound }
	if err := executor.Check("restic"); err == nil || !strings.Contains(err.Error(), "install it") {
		t.Fatalf("Check() error = %v", err)
	}
}

func TestTrimPartialSecretAtOutputLimit(t *testing.T) {
	if got := trimPartialSecretSuffix("output secret-prefix", []string{"secret-prefix-more"}); got != "output " {
		t.Fatalf("trimPartialSecretSuffix() = %q", got)
	}
}

func TestProgressRedactsCredentialsAcrossWrites(t *testing.T) {
	var output strings.Builder
	redactor := newProgressRedactor([]string{"secret-value"}, func(chunk string) {
		output.WriteString(chunk)
	})
	_, _ = redactor.Write([]byte("backup secret-"))
	if strings.Contains(output.String(), "secret-") {
		t.Fatalf("partial secret escaped before redaction: %q", output.String())
	}
	_, _ = redactor.Write([]byte("value complete"))
	redactor.Flush()
	if got, want := output.String(), "backup [REDACTED] complete"; got != want {
		t.Fatalf("redacted progress = %q, want %q", got, want)
	}
}

func TestRunnerHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	separator := -1
	for index, arg := range os.Args {
		if arg == "--" {
			separator = index
			break
		}
	}
	if separator < 0 || separator+1 >= len(os.Args) {
		os.Exit(2)
	}
	switch os.Args[separator+1] {
	case "fail":
		if separator+2 < len(os.Args) {
			_, _ = fmt.Fprintln(os.Stderr, os.Args[separator+2])
		}
		os.Exit(7)
	case "sleep":
		time.Sleep(5 * time.Second)
	}
	os.Exit(0)
}

func helperProcess(t *testing.T, mode string, args ...string) (string, []string, map[string]string) {
	t.Helper()
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	processArgs := []string{"-test.run=TestRunnerHelperProcess", "--", mode}
	processArgs = append(processArgs, args...)
	return program, processArgs, map[string]string{"GO_WANT_HELPER_PROCESS": "1"}
}
