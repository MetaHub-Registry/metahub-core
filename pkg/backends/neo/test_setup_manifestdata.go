package neo

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func (n Neo) Bootstrap_ManifestData(ctx context.Context, bs utils.Bootstrap, br *utils.BootstrapResult, b string) (err error) {
	switch b {
	case "ft_arm64":
		err = n.Setup_ManifestData(ctx, br, types.FTarmManifestData(), b)
		if err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
	case "ft_amd64":
		err = n.Setup_ManifestData(ctx, br, types.FTamdManifestData(), b)
		if err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
	case "ft_zen_5c99":
		err = n.Setup_ManifestData(ctx, br, types.ManifestDataAmd64Zen, b)
		if err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
	case "ft_zen3":
		err = n.Setup_ManifestData(ctx, br, types.FTzen3ManifestData(), b)
		if err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
	case "glow150arm64":
		err = n.Setup_ManifestData(ctx, br, types.ManifestDataGlow150ARM64, b)
		if err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
	case "glow151arm64":
		err = n.Setup_ManifestData(ctx, br, types.ManifestDataGlow151ARM64, b)
		if err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
	default:
		return fmt.Errorf("no ManifestData Setup for '%s'", b)
	}
	return
}

func (n Neo) Setup_ManifestData(ctx context.Context, br *utils.BootstrapResult, maniData types.ManifestData, label string) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"func":  "Setup_ManifestData",
		"label": label,
	})
	defer span.End()
	err = n.ManifestDataGetInsert(ctx, &maniData)
	if err != nil {
		return utils.ErrorHandler(l, span, err, "could not insert ManifestData '%s'", label)
	}
	br.Add(fmt.Sprintf("manifestdata:%s#digest", label), maniData.Digest)
	utils.Debug(l, span, "ManifestData '%s#digest' inserted: %s", label, maniData.Digest)
	br.Add(fmt.Sprintf("manifestdata:%s#id", label), fmt.Sprint(maniData.ID))
	utils.Debug(l, span, "ManifestData '%s#id' inserted: %d", label, maniData.ID)
	// Check if manifest with the same label is already in DB
	key := fmt.Sprintf("manifest:%s#id", label)
	mID, e := br.GetInt64(key)
	if e != nil {
		utils.NewError(l, span, "Manifest seems not to be present, skipping linking manifestdata to it. Trust that this will be done later eventually. :)")
	} else {
		utils.Warn(l, span, "Found manifest with ID :%d", mID)
		// Link ManifestData to Manifest
		e := n.LinkManifestToManifestData(ctx, mID, maniData.ID, maniData.Type)
		if e != nil {
			return utils.ErrorHandler(l, span, e, "LinkManifestToManifestData failed - even though we found a manifest (%d). :/", mID)
		}
	}
	// Link from ManifestData to Upstream
	if upID, found, e := n.UpstreamGetIDByDigest(ctx, maniData.Upstream.Digest.String()); e != nil {
		return e
	} else if found {
		e = n.ManifestDataLinkToUpstream(ctx, maniData.ID, upID)
		if e != nil {
			return utils.ErrorHandler(l, span, e, "ManifestDataLinkToUpstream failed")
		}
	} else {
		return utils.ErrorHandler(l, span, e, "could not find upstream with digest '%s'", maniData.Upstream.Digest.String())
	}
	return
}
