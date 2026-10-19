package rancher2

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/stretchr/testify/assert"
)

const testCredentialProcessEnv = "RANCHER2_TEST_CREDENTIAL_PROCESS"

// TestCredentialProcessHelper is not a real test. It is executed as the
// credential_process command by the tests below.
func TestCredentialProcessHelper(t *testing.T) {
	mode := os.Getenv(testCredentialProcessEnv)
	if mode == "" {
		t.Skip("credential process helper")
	}
	switch mode {
	case "success":
		fmt.Fprintln(os.Stdout, `{"kind":"ExecCredential","apiVersion":"client.authentication.k8s.io/v1beta1","spec":{},"status":{"expirationTimestamp":"2026-10-20T12:00:00Z","token":"kubeconfig-u-abc:secret"}}`)
	case "exec-credential-no-status":
		fmt.Fprintln(os.Stdout, `{"kind":"ExecCredential","apiVersion":"client.authentication.k8s.io/v1beta1","spec":{}}`)
	case "other-kind":
		fmt.Fprintln(os.Stdout, `{"kind":"Secret","status":{"token":"kubeconfig-u-abc:secret"}}`)
	case "fail":
		fmt.Fprint(os.Stderr, "unable to unlock vault")
		os.Exit(3)
	case "invalid":
		fmt.Fprint(os.Stdout, "not json")
	case "token-key":
		fmt.Fprint(os.Stdout, `{"token_key": "token-abc:secret"}`)
	case "empty":
		fmt.Fprint(os.Stdout, `{}`)
	case "slow":
		time.Sleep(10 * time.Second)
	case "prompt":
		fmt.Fprint(os.Stderr, "Enter Password: ")
		password, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		fmt.Fprintf(os.Stdout, `{"kind":"ExecCredential","status":{"token":"kubeconfig-u-abc:%s"}}`, password[:len(password)-1])
	case "background":
		// Leave a process running that holds stderr open, like a browser
		// started for login.
		child := exec.Command(os.Args[0], testCredentialProcessArgs()...)
		child.Env = append(os.Environ(), testCredentialProcessEnv+"=slow")
		child.Stderr = os.Stderr
		if err := child.Start(); err != nil {
			os.Exit(1)
		}
		fmt.Fprint(os.Stdout, `{"kind":"ExecCredential","status":{"token":"kubeconfig-u-abc:secret"}}`)
	}
	os.Exit(0)
}

// withoutCredentialProcessTerminal stops tests connecting to the terminal of
// the user running them.
func withoutCredentialProcessTerminal(t *testing.T) {
	orig := credentialProcessOpenTerminal
	credentialProcessOpenTerminal = func() (*credentialProcessTerminal, error) {
		return nil, errors.New("no terminal")
	}
	t.Cleanup(func() { credentialProcessOpenTerminal = orig })
}

func testCredentialProcessArgs() []string {
	return []string{"-test.run=^TestCredentialProcessHelper$"}
}

func TestRunCredentialProcess(t *testing.T) {
	withoutCredentialProcessTerminal(t)
	tests := []struct {
		name    string
		mode    string
		timeout time.Duration
		want    string
		wantErr string
	}{
		{name: "success", mode: "success", timeout: 10 * time.Second, want: "kubeconfig-u-abc:secret"},
		{name: "exec credential without status", mode: "exec-credential-no-status", timeout: 10 * time.Second, wantErr: "provided no ExecCredential status.token"},
		{name: "status token ignored for other kinds", mode: "other-kind", timeout: 10 * time.Second, wantErr: "provided no ExecCredential status.token"},
		{name: "non-zero exit includes stderr", mode: "fail", timeout: 10 * time.Second, wantErr: "exit status 3: unable to unlock vault"},
		{name: "invalid json", mode: "invalid", timeout: 10 * time.Second, wantErr: "provided invalid JSON"},
		{name: "token_key output not supported", mode: "token-key", timeout: 10 * time.Second, wantErr: "provided no ExecCredential status.token"},
		{name: "empty object", mode: "empty", timeout: 10 * time.Second, wantErr: "provided no ExecCredential status.token"},
		{name: "timeout", mode: "slow", timeout: 500 * time.Millisecond, wantErr: "timed out after 500ms"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(testCredentialProcessEnv, tt.mode)
			got, err := runCredentialProcess(os.Args[0], testCredentialProcessArgs(), tt.timeout)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRunCredentialProcessTerminal(t *testing.T) {
	inR, inW, err := os.Pipe()
	assert.NoError(t, err)
	outR, outW, err := os.Pipe()
	assert.NoError(t, err)
	orig := credentialProcessOpenTerminal
	credentialProcessOpenTerminal = func() (*credentialProcessTerminal, error) {
		return &credentialProcessTerminal{in: inR, out: outW}, nil
	}
	t.Cleanup(func() { credentialProcessOpenTerminal = orig })

	_, err = io.WriteString(inW, "typed-secret\n")
	assert.NoError(t, err)
	assert.NoError(t, inW.Close())

	t.Setenv(testCredentialProcessEnv, "prompt")
	got, err := runCredentialProcess(os.Args[0], testCredentialProcessArgs(), 10*time.Second)
	assert.NoError(t, err)
	assert.Equal(t, "kubeconfig-u-abc:typed-secret", got)

	prompt, err := io.ReadAll(outR)
	assert.NoError(t, err)
	assert.Contains(t, string(prompt), "Enter Password: ")
}

func TestRunCredentialProcessBackgroundChild(t *testing.T) {
	withoutCredentialProcessTerminal(t)
	t.Setenv(testCredentialProcessEnv, "background")

	start := time.Now()
	got, err := runCredentialProcess(os.Args[0], testCredentialProcessArgs(), 10*time.Second)
	assert.NoError(t, err)
	assert.Equal(t, "kubeconfig-u-abc:secret", got)
	assert.Less(t, time.Since(start), 5*time.Second)
}

func TestRunCredentialProcessCommandNotFound(t *testing.T) {
	_, err := runCredentialProcess("rancher2-credential-process-does-not-exist", nil, time.Second)
	assert.ErrorContains(t, err, "executable file not found")
}

func TestProviderConfigureCredentialProcess(t *testing.T) {
	withoutCredentialProcessTerminal(t)
	t.Setenv("RANCHER_TOKEN_KEY", "")
	t.Setenv("RANCHER_ACCESS_KEY", "")
	t.Setenv("RANCHER_SECRET_KEY", "")
	os.Unsetenv("RANCHER_TOKEN_KEY")
	os.Unsetenv("RANCHER_ACCESS_KEY")
	os.Unsetenv("RANCHER_SECRET_KEY")
	t.Setenv(testCredentialProcessEnv, "success")

	args := []interface{}{}
	for _, a := range testCredentialProcessArgs() {
		args = append(args, a)
	}
	process := []interface{}{
		map[string]interface{}{
			"command": os.Args[0],
			"args":    args,
		},
	}

	p := Provider().(*schema.Provider)

	t.Run("token from command", func(t *testing.T) {
		d := schema.TestResourceDataRaw(t, p.Schema, map[string]interface{}{
			"api_url":            "https://rancher.example.com",
			"credential_process": process,
		})
		config, err := providerConfigure(d)
		assert.NoError(t, err)
		assert.Equal(t, "kubeconfig-u-abc:secret", config.(*Config).TokenKey)
	})

	t.Run("conflicts with token_key", func(t *testing.T) {
		d := schema.TestResourceDataRaw(t, p.Schema, map[string]interface{}{
			"api_url":            "https://rancher.example.com",
			"token_key":          "token-xyz:secret",
			"credential_process": process,
		})
		_, err := providerConfigure(d)
		assert.ErrorContains(t, err, "credential_process can not be used with token_key")
	})

	t.Run("conflicts with bootstrap", func(t *testing.T) {
		d := schema.TestResourceDataRaw(t, p.Schema, map[string]interface{}{
			"api_url":            "https://rancher.example.com",
			"bootstrap":          true,
			"credential_process": process,
		})
		_, err := providerConfigure(d)
		assert.ErrorContains(t, err, "Cannot use credential_process with Bootstrap mode")
	})
}
