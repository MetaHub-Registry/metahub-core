package sqlite

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

var (
	testCfg = types.Config{
		Digest:    "sha256:1234567890",
		MediaType: "application/vnd.oci.image.config.v1+json",
	}
)

func Test_Config_Insert(t *testing.T) {
	err := sq.Config.Insert(&testCfg)
	assert.NoError(t, err)
	assert.NotEqual(t, int64(-1), testCfg.SqlID)

}
