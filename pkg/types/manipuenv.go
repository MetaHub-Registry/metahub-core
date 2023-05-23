package types

import (
	"fmt"

	"github.com/metahub-registry/metahub-core/pkg/utils"
)

type ManipuEnv struct {
	ID   int64
	Key  string
	Rule string
}

func (mpe ManipuEnv) ToString() string {
	return fmt.Sprintf("ManipuEnv{%s:%s (%s)", mpe.Rule, mpe.Key, utils.Base64Decode(mpe.Key))
}
