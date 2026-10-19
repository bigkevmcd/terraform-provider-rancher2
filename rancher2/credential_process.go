package rancher2

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/term"
)

const (
	credentialProcessExecCredentialKind = "ExecCredential"
	// credentialProcessWaitDelay bounds how long to wait for the command's
	// output to close after it exits, for example when it has started a
	// browser for login that inherited its stderr.
	credentialProcessWaitDelay = 2 * time.Second
)

// credentialProcessOpenTerminal is replaced in tests.
var credentialProcessOpenTerminal = openCredentialProcessTerminal

// credentialProcessTerminal is the user's terminal, connected to the stdin
// and stderr of a credential_process command so it can prompt for login.
type credentialProcessTerminal struct {
	in  *os.File
	out *os.File
}

func (t *credentialProcessTerminal) close() {
	t.in.Close()
	if t.out != t.in {
		t.out.Close()
	}
}

// credentialProcessOutput is the JSON document a credential_process command
// must write to stdout: a Kubernetes ExecCredential, as written by
// `rancher token`, with the token in status.token.
type credentialProcessOutput struct {
	Kind       string                         `json:"kind,omitempty"`
	APIVersion string                         `json:"apiVersion,omitempty"`
	Status     *credentialProcessOutputStatus `json:"status,omitempty"`
}

// credentialProcessOutputStatus is the status of an ExecCredential.
type credentialProcessOutputStatus struct {
	Token               string `json:"token,omitempty"`
	ExpirationTimestamp string `json:"expirationTimestamp,omitempty"`
}

// token returns the token from status.token of an ExecCredential.
func (o credentialProcessOutput) token() string {
	if o.Kind != credentialProcessExecCredentialKind || o.Status == nil {
		return ""
	}

	return o.Status.Token
}

// runCredentialProcess executes command with args, without a shell, and
// returns the token from the JSON document written to stdout. If the user's
// terminal is available, it is connected to the command's stdin and stderr so
// the command can prompt for login.
func runCredentialProcess(command string, args []string, timeout time.Duration) (string, error) {
	if command == "" {
		return "", fmt.Errorf("credential_process command is empty")
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.WaitDelay = credentialProcessWaitDelay

	if tty, err := credentialProcessOpenTerminal(); err == nil {
		defer tty.close()
		// Restore the terminal if the command is killed while it has echo
		// disabled for a password prompt.
		if state, err := term.GetState(int(tty.in.Fd())); err == nil {
			defer term.Restore(int(tty.in.Fd()), state)
		}
		cmd.Stdin = tty.in
		cmd.Stderr = io.MultiWriter(&stderr, tty.out)
	}

	// ErrWaitDelay means the command exited successfully, but something it
	// started still holds its output open; its stdout is complete.
	if err := cmd.Run(); err != nil && !errors.Is(err, exec.ErrWaitDelay) {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			err = fmt.Errorf("timed out after %s", timeout)
		}
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return "", fmt.Errorf("credential_process %q failed: %v: %s", command, err, msg)
		}

		return "", fmt.Errorf("credential_process %q failed: %v", command, err)
	}

	var output credentialProcessOutput
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		return "", fmt.Errorf("credential_process %q provided invalid JSON: %v", command, err)
	}
	token := output.token()
	if token == "" {
		return "", fmt.Errorf("credential_process %q provided no ExecCredential status.token", command)
	}

	return token, nil
}
