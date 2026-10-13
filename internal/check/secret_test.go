package check

import (
	"fmt"
	"testing"
)

func TestSecrets(t *testing.T) {
	body := "-----BEGIN OPENSSH PRIVATE KEY-----\n" +
		"password=hunter2\n" +
		"password= token=$TOKEN secret=<redacted> password=\"\" token=*** secret=xxxx\n" +
		"AKIAabcdefghijklmnopqrstuvwxyz0123456789ABCD\n" +
		"short abcDEF123+/\n" +
		"sha 0123456789abcdef0123456789abcdef01234567 /nix/store/0123456789abcdfghijklmnpqrsvwxyz-x\n" +
		"no digit ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnop\n"
	got := fmt.Sprint(secrets(body))
	want := "[{1 a private key header} {2 a password, token or secret assignment} {4 a run of 40 or more base64 characters}]"
	if got != want {
		t.Errorf("secrets:\n got %s\nwant %s", got, want)
	}
	if s := secrets(body + "<!-- memex: allow-secret -->\n"); s != nil {
		t.Errorf("allowed: %v", s)
	}
}
