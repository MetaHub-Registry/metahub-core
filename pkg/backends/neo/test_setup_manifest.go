package neo

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func (n Neo) Bootstrap_Manifest(ctx context.Context, bs utils.Bootstrap, br *utils.BootstrapResult, b string) (err error) {
	switch b {
	case "ft_arm64":
		err = n.Setup_Manifest_FT_arm64(br)
		if err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
	case "ft_zen":
		err = n.SetupManifestFTZen(br)
		if err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
	case "ft_zen3":
		err = n.Setup_Manifest_FT_zen3(br)
		if err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
	case "ft_amd64":
		err = n.Setup_Manifest_FT_amd64(ctx, br)
		if err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
	case "ft_mpich_amd64":
		err = n.Setup_Manifest_FT_mpich_amd64(br)
		if err != nil {
			br.PrintDict()
			return errors.Wrapf(err, whileFmrt, b)
		}
	default:
		return fmt.Errorf("no ManifestData Setup for '%s'", b)
	}
	return
}

func (n Neo) Setup_Manifest_FT_amd64(ctx context.Context, br *utils.BootstrapResult) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"func": "Setup_Manifest_FT_amd64",
	})
	defer span.End()
	// Add Upstream
	upID, err := br.GetInt64("upstream:ft_amd64#id")
	if err != nil {
		return err
	}
	utils.Debug(l, span, "Get Upstream w/ ID: %d", upID)
	up, _, err := n.UpstreamGetByID(ctx, upID)
	if err != nil {
		return utils.ErrorHandler(l, span, err, "failed to get upstream id from BR")
	}
	/// Get Platform
	plaID, err := br.GetInt64("platform:linux/amd64#id")
	if err != nil {
		return utils.ErrorHandler(l, span, err, "failed to get platform id from BR")
	}
	utils.Debug(l, span, "Get Platform w/ ID: %d", plaID)
	plat, found, err := n.Platform_GetByID(plaID)
	if err != nil {
		return utils.ErrorHandler(l, span, err, "failed to get platform by id")
	}
	if !found {
		return utils.NewError(l, span, "platformID '%d' not found", plaID)
	}
	utils.Debug(l, span, "Get Platform: %s/%s/%s", plat.OS, plat.Arch, plat.Variant)
	as := types.NewAnnotations()
	m := types.Manifest{
		Upstream:    up,
		Platform:    plat,
		Annotations: as,
	}
	maniID, err := n.ManifestGetInsert(ctx, m.Upstream, m.Annotations.ToSlice()...)
	if err != nil {
		return utils.ErrorHandler(l, span, err, "failed to insert manifest")
	}
	utils.Debug(l, span, "Inserted Manifest with ID: %d", maniID)
	br.Add("manifest:ft_amd64#id", fmt.Sprint(maniID))
	err = n.ManifestLinkToUpstream(ctx, maniID, upID)
	if err != nil {
		return utils.ErrorHandler(l, span, err, "failed to link manifest to upstream")
	}
	err = n.LinkManifestToPlatform(ctx, maniID, plaID)
	if err != nil {
		return utils.ErrorHandler(l, span, err, "failed to link manifest to platform")
	}
	// Link Config
	config_id, err := br.GetInt64(fmt.Sprintf("config:ft_%s#id", m.Platform.Arch))
	if err != nil {
		return utils.ErrorHandler(l, span, err, "failed to get config id from BR")
	}
	config, found, err := n.ConfigGetByID(ctx, config_id)
	if err != nil {
		return utils.ErrorHandler(l, span, err, "failed to get config by id")
	}
	if !found {
		err = fmt.Errorf("config not found")
		return
	}
	config.MediaType, err = br.Get(fmt.Sprintf("config:ft_%s#mediaType", m.Platform.Arch))
	if err != nil {
		return utils.ErrorHandler(l, span, err, "failed to get config mediaType from BR")
	}
	// ManifestData
	maniDataID, e := br.GetInt64("manifestdata:ft_amd64#id")
	if e != nil {
		utils.NewError(l, span, "ManifestData seems not to be present, skipping linking manifest to it. Trust that this will be done later eventually. :)")
	} else {
		err = n.LinkManifestToManifestData(ctx, maniID, maniDataID, "ORIGINAL")
		if err != nil {
			return utils.ErrorHandler(l, span, err, "failed to link manifest to config")
		}
	}
	return
}

func (n Neo) Setup_Manifest_FT_arm64(br *utils.BootstrapResult) (err error) {
	ctx := context.Background()
	// Add Manifest
	/// Get Upstream
	upID, err := br.GetInt64("upstream:ft_arm64#id")
	if err != nil {
		return err
	}
	up, _, err := n.UpstreamGetByID(ctx, upID)
	if err != nil {
		return err
	}
	/// Get Platform
	plaID, err := br.GetInt64("platform:linux/arm64#id")
	if err != nil {
		return err
	}
	plat, found, err := n.Platform_GetByID(plaID)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("platform not found")
	}
	m := types.Manifest{
		Upstream: up,
		Platform: plat,
	}
	maniID, err := n.ManifestGetInsert(ctx, m.Upstream, m.Annotations.ToSlice()...)
	br.Add("manifest:ft_arm64#id", fmt.Sprint(maniID))
	if err != nil {
		return err
	}
	err = n.ManifestLinkToUpstream(ctx, maniID, upID)
	if err != nil {
		return err
	}
	// Link Platform
	err = n.LinkManifestToPlatform(ctx, maniID, plaID)
	if err != nil {
		logrus.Error(err)
		return err
	}
	/*
		// Link Config
		arm_config_id, err := br.GetInt64(fmt.Sprintf("config:ft_%s#id", m.Platform.Arch))
		if err != nil {
			logrus.Error(err)
			return err
		}
		arm_config, found, err := n.ConfigGetByID(ctx, arm_config_id)
		if err != nil {
			logrus.Error(err)
			return err
		}
		if !found {
			err = fmt.Errorf("config not found")
			return
		}
		arm_config.MediaType, err = br.Get(fmt.Sprintf("config:ft_%s#mediaType", m.Platform.Arch))
		if err != nil {
			logrus.Error(err)
			return err
		}
		err = n.ManifestDataLinkToConfig(ctx, maniID, arm_config)
		if err != nil {
			logrus.Error(err)
			return err
		}
	*/
	return
}
func (n Neo) SetupManifestFTZen(br *utils.BootstrapResult) (err error) {
	ctx := context.Background()
	// Add Manifest
	/// Get Upstream
	upID, err := br.GetInt64("upstream:ft_zen#id")
	if err != nil {
		return err
	}
	up, _, err := n.UpstreamGetByID(ctx, upID)
	if err != nil {
		return err
	}
	/// Get Platform
	plaID, err := br.GetInt64("platform:linux/amd64#id")
	if err != nil {
		return err
	}
	plat, found, err := n.Platform_GetByID(plaID)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("platform not found")
	}
	// Annotations
	as := types.NewAnnotations()
	z3 := types.NewAnnotation("org.supercontainers.hardware.cpu.target.microarch", "zen")
	as.AddAnnotation(z3)
	m := types.Manifest{
		Upstream:    up,
		Platform:    plat,
		Annotations: as,
	}
	// Manifest
	maniID, err := n.ManifestGetInsert(ctx, m.Upstream, m.Annotations.ToSlice()...)
	br.Add("manifest:ft_zen#id", fmt.Sprint(maniID))
	if err != nil {
		return err
	}
	// Link Platform
	err = n.LinkManifestToPlatform(ctx, maniID, plaID)
	if err != nil {
		logrus.Error(err)
		return err
	}
	// Link Config
	config_id, err := br.GetInt64("config:ft_zen#id")
	if err != nil {
		logrus.Error(err)
		return err
	}
	config, found, err := n.ConfigGetByID(ctx, config_id)
	if err != nil {
		logrus.Error(err)
		return err
	}
	if !found {
		err = fmt.Errorf("config not found")
		return
	}
	config.MediaType, err = br.Get("config:ft_zen#mediaType")
	if err != nil {
		logrus.Error(err)
		return err
	}
	err = n.ManifestDataLinkToConfig(ctx, maniID, config)
	if err != nil {
		logrus.Error(err)
		return err
	}
	return
}

func (n Neo) Setup_Manifest_FT_zen3(br *utils.BootstrapResult) (err error) {
	ctx := context.Background()
	// Add Manifest
	/// Get Upstream
	upID, err := br.GetInt64("upstream:ft_zen3#id")
	if err != nil {
		return err
	}
	up, _, err := n.UpstreamGetByID(ctx, upID)
	if err != nil {
		return err
	}
	/// Get Platform
	plaID, err := br.GetInt64("platform:linux/amd64#id")
	if err != nil {
		return err
	}
	plat, found, err := n.Platform_GetByID(plaID)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("platform not found")
	}
	// Annotations
	as := types.NewAnnotations()
	z3 := types.NewAnnotation("org.supercontainers.hardware.cpu.target.microarch", "zen3")
	as.AddAnnotation(z3)
	m := types.Manifest{
		Upstream:    up,
		Platform:    plat,
		Annotations: as,
	}
	// Manifest
	maniID, err := n.ManifestGetInsert(ctx, m.Upstream, m.Annotations.ToSlice()...)
	br.Add("manifest:ft_zen3#id", fmt.Sprint(maniID))
	if err != nil {
		return err
	}
	err = n.ManifestLinkToUpstream(ctx, maniID, upID)
	if err != nil {
		return err
	}
	// Link Platform
	err = n.LinkManifestToPlatform(ctx, maniID, plaID)
	if err != nil {
		logrus.Error(err)
		return err
	}
	// Link Config
	config_id, err := br.GetInt64("config:ft_zen3#id")
	if err != nil {
		logrus.Error(err)
		return err
	}
	config, found, err := n.ConfigGetByID(ctx, config_id)
	if err != nil {
		logrus.Error(err)
		return err
	}
	if !found {
		err = fmt.Errorf("config not found")
		return
	}
	config.MediaType, err = br.Get("config:ft_zen3#mediaType")
	if err != nil {
		logrus.Error(err)
		return err
	}
	err = n.ManifestDataLinkToConfig(ctx, maniID, config)
	if err != nil {
		logrus.Error(err)
		return err
	}
	return
}
