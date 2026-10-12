package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

const maxOutputBytes = 4 << 20

type Result struct {
	Output   string
	ExitCode int
	Duration time.Duration
}

type Executor struct {
	LookupPath func(string) (string, error)
}

func New() *Executor {
	return &Executor{LookupPath: exec.LookPath}
}

func (e *Executor) Check(program string) error {
	lookup := e.LookupPath
	if lookup == nil {
		lookup = exec.LookPath
	}
	if _, err := lookup(program); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return fmt.Errorf("%s was not found on PATH; install it and try again", program)
		}
		return fmt.Errorf("find %s on PATH: %w", program, err)
	}
	return nil
}

// Run executes a program with explicit arguments. Configured variables override
// inherited variables; output and returned errors redact every supplied secret.
func (e *Executor) Run(ctx context.Context, program string, args []string, configuredEnv map[string]string, secrets []string) (Result, error) {
	return e.RunWithProgress(ctx, program, args, configuredEnv, secrets, nil)
}

func (e *Executor) RunWithProgress(ctx context.Context, program string, args []string, configuredEnv map[string]string, secrets []string, onOutput func(string)) (Result, error) {
	start := time.Now()
	if err := e.Check(program); err != nil {
		return Result{}, err
	}
	cmd := exec.CommandContext(ctx, program, args...)
	cmd.Env = MergeEnvironment(os.Environ(), configuredEnv)
	var output limitedBuffer
	if onOutput == nil {
		cmd.Stdout = &output
		cmd.Stderr = &output
	} else {
		progress := newProgressRedactor(secrets, onOutput)
		stream := io.MultiWriter(&output, progress)
		cmd.Stdout = stream
		cmd.Stderr = stream
		defer progress.Flush()
	}
	err := cmd.Run()
	outputText := output.String()
	if output.Truncated() {
		outputText = trimPartialSecretSuffix(outputText, secrets)
		outputText += "\n[output truncated at 4 MiB]\n"
	}
	result := Result{
		Output:   Redact(outputText, secrets),
		ExitCode: 0,
		Duration: time.Since(start),
	}
	if err == nil {
		return result, nil
	}
	if ctx.Err() != nil {
		return result, fmt.Errorf("command cancelled: %w", ctx.Err())
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, fmt.Errorf("%s exited with status %d", program, exitErr.ExitCode())
	}
	return result, fmt.Errorf("run %s: %w", program, err)
}

type progressRedactor struct {
	mu      sync.Mutex
	pending []byte
	secrets []string
	write   func(string)
}

func newProgressRedactor(secrets []string, write func(string)) *progressRedactor {
	filtered := make([]string, 0, len(secrets))
	for _, secret := range secrets {
		if secret != "" {
			filtered = append(filtered, secret)
		}
	}
	secrets = filtered
	sort.Slice(secrets, func(i, j int) bool {
		return len(secrets[i]) > len(secrets[j])
	})
	return &progressRedactor{secrets: secrets, write: write}
}

func (r *progressRedactor) Write(chunk []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pending = append(r.pending, chunk...)
	r.emitSafePrefix()
	return len(chunk), nil
}

func (r *progressRedactor) emitSafePrefix() {
	safeEnd := len(r.pending)
	for _, secret := range r.secrets {
		for length := 1; length < len(secret) && length <= len(r.pending); length++ {
			if bytes.HasSuffix(r.pending, []byte(secret[:length])) && len(r.pending)-length < safeEnd {
				safeEnd = len(r.pending) - length
			}
		}
	}
	for changed := true; changed; {
		changed = false
		for _, secret := range r.secrets {
			secretBytes := []byte(secret)
			for start := bytes.Index(r.pending, secretBytes); start >= 0; {
				end := start + len(secretBytes)
				if start < safeEnd && end > safeEnd {
					safeEnd = start
					changed = true
				}
				next := bytes.Index(r.pending[start+1:], secretBytes)
				if next < 0 {
					break
				}
				start += next + 1
			}
		}
	}
	if safeEnd > 0 {
		r.write(Redact(string(r.pending[:safeEnd]), r.secrets))
		r.pending = append(r.pending[:0], r.pending[safeEnd:]...)
	}
}

func (r *progressRedactor) Flush() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.pending) > 0 {
		r.write(Redact(string(r.pending), r.secrets))
		r.pending = nil
	}
}

func MergeEnvironment(inherited []string, configured map[string]string) []string {
	type environmentValue struct {
		key, value string
	}
	values := make(map[string]environmentValue, len(inherited)+len(configured))
	normalize := func(key string) string {
		if runtime.GOOS == "windows" {
			return strings.ToUpper(key)
		}
		return key
	}
	for _, entry := range inherited {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			values[normalize(key)] = environmentValue{key: key, value: value}
		}
	}
	for key, value := range configured {
		values[normalize(key)] = environmentValue{key: key, value: value}
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		entry := values[key]
		result = append(result, entry.key+"="+entry.value)
	}
	return result
}

func trimPartialSecretSuffix(output string, secrets []string) string {
	cut := len(output)
	for _, secret := range secrets {
		for length := 1; length < len(secret) && length <= len(output); length++ {
			if strings.HasSuffix(output, secret[:length]) && len(output)-length < cut {
				cut = len(output) - length
			}
		}
	}
	return output[:cut]
}

func Redact(text string, secrets []string) string {
	secrets = append([]string(nil), secrets...)
	sort.Slice(secrets, func(i, j int) bool {
		return len(secrets[i]) > len(secrets[j])
	})
	for _, secret := range secrets {
		if secret != "" {
			text = strings.ReplaceAll(text, secret, "[REDACTED]")
		}
	}
	return text
}

type limitedBuffer struct {
	mu sync.Mutex
	bytes.Buffer
	truncated bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	original := len(p)
	remaining := maxOutputBytes - b.Len()
	if remaining > 0 {
		if len(p) > remaining {
			p = p[:remaining]
			b.truncated = true
		}
	} else if len(p) > 0 {
		b.truncated = true
	}
	if len(p) > 0 {
		_, _ = b.Buffer.Write(p)
	}
	return original, nil
}

func (b *limitedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.Buffer.String()
}

func (b *limitedBuffer) Truncated() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.truncated
}
