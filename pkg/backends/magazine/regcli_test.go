package magazine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRegclient(t *testing.T) {
	magLocal := Magazine{
		Config: MagConfig{
			Args: map[string]string{
				"TEST_REGISTRY": "http://localhost:5000",
			},
		},
	}
	rc, regOut, err := magLocal.NewRegClient(ctx, "registry:5050")
	if err != nil {
		t.Fatal(err)
	}
	if rc == nil {
		t.Fatal("rc is nil")
	}
	assert.Equal(t, "http://localhost:5000", regOut)
	// Actual test
}
