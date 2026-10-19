//go:build windows

package rancher2

import "os"

// openCredentialProcessTerminal opens the console of the user running
// Terraform, so a credential_process command can prompt for login.
func openCredentialProcessTerminal() (*credentialProcessTerminal, error) {
	in, err := os.OpenFile("CONIN$", os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	out, err := os.OpenFile("CONOUT$", os.O_RDWR, 0)
	if err != nil {
		in.Close()
		return nil, err
	}

	return &credentialProcessTerminal{in: in, out: out}, nil
}
