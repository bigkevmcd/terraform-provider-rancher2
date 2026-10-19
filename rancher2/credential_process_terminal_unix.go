//go:build !windows

package rancher2

import "os"

// openCredentialProcessTerminal opens the controlling terminal of the user
// running Terraform, so a credential_process command can prompt for login.
func openCredentialProcessTerminal() (*credentialProcessTerminal, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}

	return &credentialProcessTerminal{in: tty, out: tty}, nil
}
