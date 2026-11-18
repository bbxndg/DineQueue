package cidemo

import "testing"

// TestIntentionalCIFailure is confined to a disposable EP4 demonstration branch.
// Close its PR without merging; this file must not enter staging or main.
func TestIntentionalCIFailure(t *testing.T) {
	t.Fatal("INTENTIONAL EP4 CI FAILURE: evidence demonstration only; do not merge this PR")
}
