package facts

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A fact reference that cannot resolve must fail, including in a suite that
// gathers no facts at all. Passing the literal text through produced a test
// that ran a nonsense command and reported success.
func TestFactReferenceFailsWithAnEmptyStore(t *testing.T) {
	_, err := RenderTemplate(`echo v={{ fact "db" "ipv4" }}`, FactStore{}, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no facts are available in this suite")
	// The message says where facts come from, since the fix is to add them
	assert.Contains(t, err.Error(), "facts:")
}

// With facts present, an unknown node still names the node specifically.
func TestFactReferenceFailsForUnknownNode(t *testing.T) {
	store := FactStore{"web": map[string]string{"ipv4": "10.0.0.1"}}
	_, err := RenderTemplate(`echo v={{ fact "db" "ipv4" }}`, store, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `no facts for node "db"`)
}

// Text without a fact reference is untouched by an empty store, so always
// rendering cannot break suites that use no facts.
func TestEmptyStoreLeavesOtherTextAlone(t *testing.T) {
	for _, text := range []string{
		"echo hello",
		"echo {{capture.addr}}",
		"systemctl is-active nginx",
	} {
		out, err := RenderTemplate(text, FactStore{}, "")
		require.NoError(t, err, text)
		assert.Equal(t, text, out)
	}
}

// A resolvable reference still renders with a populated store.
func TestFactReferenceRendersWhenAvailable(t *testing.T) {
	store := FactStore{"db": map[string]string{"ipv4": "10.0.0.5"}}
	out, err := RenderTemplate(`echo v={{ fact "db" "ipv4" }}`, store, "")
	require.NoError(t, err)
	assert.Equal(t, "echo v=10.0.0.5", out)
}
