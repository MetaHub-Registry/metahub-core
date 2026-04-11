package neo

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func (n Neo) Bootstrap_Scenario(ctx context.Context, bs utils.Bootstrap, br *utils.BootstrapResult, b string) (err error) {
	switch {
	case strings.HasPrefix(b, "manifest_"):
		if e := n.Setup_Platforms(br, types.Example_PlatformGeneric); e != nil {
			return errors.Wrapf(e, whileFmrt, b)
		}
		// bs.AddBootstrap("rules:rules/cpu/noop.yaml")
		if e := n.Setup_Rules(ctx, bs, br, "rules/cpu/noop.yaml"); e != nil {
			return errors.Wrapf(e, whileFmrt, b)
		}
		// bs.AddBootstrap("rules:rules/os/linux.yaml")
		if e := n.Setup_Rules(ctx, bs, br, "rules/os/linux.yaml"); e != nil {
			return errors.Wrapf(e, whileFmrt, b)
		}
		// bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
		if e := n.Setup_Profiles(ctx, bs, br, "profiles/cpu/generic.yaml"); e != nil {
			return errors.Wrapf(e, whileFmrt, b)
		}
		_, b, _ = strings.Cut(b, "manifest_")
		var cfg types.Config
		var up types.Upstream
		var md types.Manifest
		switch b {
		case "ft_amd64":
			cfg = types.FTamd64Config()
			up = types.FTamd64Upstream()
			md = types.FTamdManifest()
		case "ft_arm64":
			cfg = types.FTarmConfig()
			up = types.FTarm64Upstream()
			md = types.FTarmManifest()
		default:
			return fmt.Errorf("unknown scenario manifest '%s'", b)
		}
		// Setup qnib/featuretest:amd up to manifest
		// - manifestdata is present
		// - manifest is present and connected to manifestdata
		// - layers are present and connected to manifestdata
		// - config is present and connected to manifestdata
		// - upstream is present and connected to manifestdata
		// - profile is present and connected to manifestdata
		// - platform is present and connected to profile
		// - rules are present and connected to profile
		// - collection is present and connected to manifest
		// - tag is present and connected to manifest
		whileFmrt := "while Scenario Bootstrap.%s '%s'"
		// bs.AddBootstrap("config:ft_amd64")
		if e := n.Setup_Config(ctx, br, cfg, b); e != nil {
			return errors.Wrapf(e, whileFmrt, "config", b)
		}
		for _, l := range br.ToLines() {
			logrus.Infof("Bootstrap Result: %s", l)
		}
		// bs.AddBootstrap("upstream:ft_amd64")
		if e := n.Setup_Upstream(br, up, b); e != nil {
			return errors.Wrapf(e, whileFmrt, "upstream", b)
		}
		switch b {
		case "ft_amd64":
			// bs.AddBootstrap("layer:0:ft_amd64")
			if e := n.Setup_Layer(ctx, br, types.FTamd64Layer0(), `0:`+b); e != nil {
				return errors.Wrapf(e, whileFmrt, "layer:0", b)
			}
			// bs.AddBootstrap("layer:1:ft_amd64")
			if e := n.Setup_Layer(ctx, br, types.FTamd64Layer1(), `1:`+b); e != nil {
				return errors.Wrapf(e, whileFmrt, "layer:1", b)
			}
		case "ft_arm64":
			// bs.AddBootstrap("layer:0:ft_amd64")
			if e := n.Setup_Layer(ctx, br, types.FTarm64Layer0(), `0:`+b); e != nil {
				return errors.Wrapf(e, whileFmrt, "layer:0", b)
			}
			// bs.AddBootstrap("layer:1:ft_amd64")
			if e := n.Setup_Layer(ctx, br, types.FTarm64Layer1(), `1:`+b); e != nil {
				return errors.Wrapf(e, whileFmrt, "layer:1", b)
			}
		default:
			return fmt.Errorf("unknown scenario layer '%s'", b)

		}
		switch b {
		case "ft_amd64":
			// Setup AMD64 manifest for generic featuretest
			err = n.Setup_Manifest_FT_amd64(ctx, br)
			if err != nil {
				return errors.Wrapf(err, whileFmrt, "manifest", b)
			} // bs.AddBootstrap("collection:ft_generic")
			if e := n.Setup_Collection_FT_amd64(ctx, br); e != nil {
				return errors.Wrapf(e, whileFmrt, "collection", b)
			}
		case "ft_arm64":
			// Setup AMD64 manifest for generic featuretest
			err = n.Setup_Manifest_FT_arm64(br)
			if err != nil {
				return errors.Wrapf(err, whileFmrt, "manifest", b)
			}
			// bs.AddBootstrap("collection:ft_generic")
			if e := n.Setup_Collection_FT_arm64(ctx, br); e != nil {
				return errors.Wrapf(e, whileFmrt, "collection", b)
			}
		default:
			return fmt.Errorf("unknown scenario manifest '%s'", b)
		}
		// bs.AddBootstrap("manifestdata:ft_amd64")
		if e := n.Setup_Manifest(ctx, br, md, b); e != nil {
			return errors.Wrapf(e, whileFmrt, "manifestdata", b)
		}

		return
	case strings.HasPrefix(b, "oras_cyclonedx"):
		return n.Bootstrap_Scenario_OrasCyclonedx(ctx, bs, br)
	}
	return fmt.Errorf("no Scenario Setup for '%s'", b)
}

// Bootstrap_Scenario_OrasCyclonedx sets up a minimal ORAS CycloneDX artifact in Neo4j:
//   - upstream (ft_oras_cyclonedx)
//   - config blob  (ft_oras_cyclonedx, mediaType=application/vnd.cyclonedx)
//   - manifest     (ft_oras_cyclonedx)
//   - manifestdata (ft_oras_cyclonedx, linked to config and upstream)
//   - collection   (qnib/featuretest:cyclonedx, linked to manifest)
//
// No platform is set up because ORAS artifacts are not platform-specific.
func (n Neo) Bootstrap_Scenario_OrasCyclonedx(ctx context.Context, bs utils.Bootstrap, br *utils.BootstrapResult) (err error) {
	const label = "ft_oras_cyclonedx"
	// Upstream
	if e := n.Setup_Upstream(br, types.FTOrasUpstream(), label); e != nil {
		return errors.Wrapf(e, "Bootstrap_Scenario_OrasCyclonedx: upstream")
	}
	// Config (non-container mediaType — this is what makes it an ORAS artifact)
	if e := n.Setup_Config(ctx, br, types.FTOrasConfig(), label); e != nil {
		return errors.Wrapf(e, "Bootstrap_Scenario_OrasCyclonedx: config")
	}
	// Manifest
	if e := n.Setup_Manifest_FT_oras_cyclonedx(ctx, br); e != nil {
		return errors.Wrapf(e, "Bootstrap_Scenario_OrasCyclonedx: manifest")
	}
	// Manifest (links config + upstream; no platform)
	if e := n.Setup_OrasManifest(ctx, br, types.FTOrasManifest(), label); e != nil {
		return errors.Wrapf(e, "Bootstrap_Scenario_OrasCyclonedx: manifestdata")
	}
	// Repo
	if e := n.Setup_Collection_FT_oras_cyclonedx(ctx, br); e != nil {
		return errors.Wrapf(e, "Bootstrap_Scenario_OrasCyclonedx: collection")
	}
	return
}
