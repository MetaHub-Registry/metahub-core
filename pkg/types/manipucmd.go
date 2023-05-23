package types

import (
	"fmt"

	"github.com/metahub-registry/metahub-core/pkg/utils"
)

type ManipuCmd struct {
	ID   int64
	Rule string
	Cmd  string
}

func (mpc ManipuCmd) ToString() string {
	return fmt.Sprintf("ManipuCmd{%s:%s (%s)}", mpc.Rule, mpc.Cmd, utils.Base64Decode(mpc.Cmd))
}
