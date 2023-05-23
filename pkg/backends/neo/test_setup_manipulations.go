package neo

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"github.com/metahub-registry/metahub-core/pkg/yaml/config"
)

func (n Neo) Bootstrap_Manipulation(ctx context.Context, bs utils.Bootstrap, br *utils.BootstrapResult, b string) (err error) {
	err = n.Setup_Manipulation(ctx, bs, br, b)
	if err != nil {
		err = errors.Wrapf(err, "Setup_Manipulation(%s)", b)
		return
	}
	return
}

func (n Neo) Setup_Manipulation(ctx context.Context, bs utils.Bootstrap, br *utils.BootstrapResult, d string) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"func": "Setup_Manipulation",
		"dir":  d,
	})
	defer span.End()
	pre := bs.YAMLPrefix
	if pre == "" {
		return utils.NewError(l, span, `the YAMLPrefix is empty, please use "bs.SetYAMLPrefix(<relative path to yaml root>)"`)
	}
	path := filepath.Join(pre, d)
	cfgMps, e := config.FindManipulations(ctx, path)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "failed to find manipulation in %s", path)
	}
	for _, cfgMp := range cfgMps {
		tmp, e := n.ManipulationFromConfigManipulation(ctx, cfgMp)
		if e != nil {
			return utils.ErrorHandler(l, span, e, "failed to create manipulation from config manipulation")
		}
		if e := n.ManipulationGetCreate(ctx, &tmp); e != nil {
			return utils.ErrorHandler(l, span, e, "failed to create manipulation")
		}
		br.Add(fmt.Sprintf("manipulation:%s#id", tmp.Label), fmt.Sprint(tmp.ID))
		utils.Debug(l, span, "Manipulation '%s' created with ID: %d", tmp.Label, tmp.ID)
		for _, mpl := range tmp.ManipuLayers {
			br.Add(fmt.Sprintf("manipulayer:%s#id", mpl.Label), fmt.Sprint(mpl.ID))
		}

	}
	return
}
