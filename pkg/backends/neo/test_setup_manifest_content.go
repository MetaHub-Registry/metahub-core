package neo

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func (n Neo) Bootstrap_Manifest(ctx context.Context, bs utils.Bootstrap, br *utils.BootstrapResult, b string) (err error) {
	switch b {
	case "ft_arm64":
		err = n.Setup_Manifest(ctx, br, types.FTarmManifest(), b)
		if err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
	case "ft_amd64":
		err = n.Setup_Manifest(ctx, br, types.FTamdManifest(), b)
		if err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
	case "ft_zen_5c99":
		err = n.Setup_Manifest(ctx, br, types.ManifestAmd64Zen, b)
		if err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
	case "ft_zen3":
		err = n.Setup_Manifest(ctx, br, types.FTzen3Manifest(), b)
		if err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
	case "glow150arm64":
		err = n.Setup_Manifest(ctx, br, types.ManifestGlow150ARM64, b)
		if err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
	case "glow151arm64":
		err = n.Setup_Manifest(ctx, br, types.ManifestGlow151ARM64, b)
		if err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
	case "ft_oras_cyclonedx":
		err = n.Setup_OrasManifest(ctx, br, types.FTOrasManifest(), b)
		if err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
	default:
		return fmt.Errorf("no Manifest Setup for '%s'", b)
	}
	return
}

// Setup_OrasManifest inserts a Manifest for an ORAS artifact and records the result
// in the BootstrapResult. Unlike Setup_Manifest it skips platform linking because ORAS
// artifacts are not platform-specific.
func (n Neo) Setup_OrasManifest(ctx context.Context, br *utils.BootstrapResult, maniData types.Manifest, label string) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"func":  "Setup_OrasManifest",
		"label": label,
	})
	defer span.End()
	err = n.ManifestGetInsert(ctx, &maniData)
	if err != nil {
		return utils.ErrorHandler(l, span, err, "could not insert ORAS Manifest '%s'", label)
	}
	br.Add(fmt.Sprintf("manifestdata:%s#digest", label), maniData.Digest)
	utils.Debug(l, span, "ORAS Manifest '%s#digest' inserted: %s", label, maniData.Digest)
	br.Add(fmt.Sprintf("manifestdata:%s#id", label), fmt.Sprint(maniData.ID))
	utils.Debug(l, span, "ORAS Manifest '%s#id' inserted: %d", label, maniData.ID)
	br.Add(fmt.Sprintf("manifestdata:%s#mediaType", label), maniData.MediaType)
	utils.Debug(l, span, "ORAS Manifest '%s#mediaType' inserted: %s", label, maniData.MediaType)
	// Phase 4 (deprecate-manifest-wrapper): the content-leaf :Manifest is the
	// canonical node. Alias "manifest:<label>#id" to the content-leaf id so the
	// legacy test sites that read it keep working.
	br.Add(fmt.Sprintf("manifest:%s#id", label), fmt.Sprint(maniData.ID))
	// Link from Manifest to Upstream
	if upID, found, e := n.UpstreamGetIDByDigest(ctx, maniData.Upstream.Digest.String()); e != nil {
		return e
	} else if found {
		if e = n.ManifestLinkToUpstream(ctx, maniData.ID, upID); e != nil {
			return utils.ErrorHandler(l, span, e, "ManifestLinkToUpstream failed")
		}
	} else {
		return utils.ErrorHandler(l, span, e, "could not find upstream with digest '%s'", maniData.Upstream.Digest.String())
	}
	// Slot-centred wiring: Slot(repo, tag, generic) -> HAS_PUSHED -> content-leaf
	n.setupSlotForLeaf(ctx, br, label, maniData.Upstream.Repo, maniData.Upstream.Tag, "generic", maniData.ID)
	return
}

// setupSlotForLeaf wires the Slot-centred graph for a content-leaf :Manifest
// fixture: it MERGEs a :Slot for the (repo, tag, profile) triple (creating the
// :Repo/:Tag/:Profile string-identity nodes), records the slot id in the
// BootstrapResult under "slot:<label>#id", and appends a :HAS_PUSHED edge from
// the Slot to the content-leaf Manifest. This mirrors the production
// wireSlotOnPush path so fixtures exercise the Slot-centred model rather than
// only the legacy :ManifestWrapper node.
//
// It is intentionally additive and non-fatal: wrapper-node creation is retained
// elsewhere until the read-side Cypher is migrated (Phase 4 / tasks §6), so a
// missing triple component here is logged and skipped rather than failing the
// fixture.
func (n Neo) setupSlotForLeaf(ctx context.Context, br *utils.BootstrapResult, label, repo, tag, profile string, mdID int64) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"func":    "setupSlotForLeaf",
		"label":   label,
		"repo":    repo,
		"tag":     tag,
		"profile": profile,
	})
	defer span.End()
	if repo == "" || tag == "" {
		utils.Warn(l, span, "setupSlotForLeaf: empty repo/tag for label %q (repo=%q tag=%q); skipping Slot wiring", label, repo, tag)
		return
	}
	if profile == "" {
		profile = "generic"
	}
	slot, e := n.SlotGetCreate(ctx, repo, tag, profile)
	if e != nil {
		utils.Warn(l, span, "setupSlotForLeaf: SlotGetCreate failed for %q (non-fatal): %v", label, e)
		return
	}
	br.Add(fmt.Sprintf("slot:%s#id", label), fmt.Sprint(slot.ID))
	if e := n.SlotAppendPushedManifest(ctx, slot.ID, mdID, ""); e != nil {
		utils.Warn(l, span, "setupSlotForLeaf: SlotAppendPushedManifest failed for %q (non-fatal): %v", label, e)
		return
	}
	utils.Debug(l, span, "setupSlotForLeaf: slot %d (%s:%s/%s) -> HAS_PUSHED manifest %d", slot.ID, repo, tag, profile, mdID)
}

func (n Neo) Setup_Manifest(ctx context.Context, br *utils.BootstrapResult, maniData types.Manifest, label string) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"func":  "Setup_Manifest",
		"label": label,
	})
	defer span.End()
	err = n.ManifestGetInsert(ctx, &maniData)
	if err != nil {
		return utils.ErrorHandler(l, span, err, "could not insert Manifest '%s'", label)
	}
	br.Add(fmt.Sprintf("manifestdata:%s#digest", label), maniData.Digest)
	utils.Debug(l, span, "Manifest '%s#digest' inserted: %s", label, maniData.Digest)
	br.Add(fmt.Sprintf("manifestdata:%s#id", label), fmt.Sprint(maniData.ID))
	utils.Debug(l, span, "Manifest '%s#id' inserted: %d", label, maniData.ID)
	br.Add(fmt.Sprintf("manifestdata:%s#mediaType", label), maniData.MediaType)
	utils.Debug(l, span, "Manifest '%s#mediaType' inserted: %s", label, maniData.MediaType)
	// Phase 4 (deprecate-manifest-wrapper): the content-leaf :Manifest is the
	// canonical node. Alias "manifest:<label>#id" to the content-leaf id so the
	// legacy test sites that read it keep working.
	br.Add(fmt.Sprintf("manifest:%s#id", label), fmt.Sprint(maniData.ID))
	// Link from Manifest to Upstream
	if upID, found, e := n.UpstreamGetIDByDigest(ctx, maniData.Upstream.Digest.String()); e != nil {
		return e
	} else if found {
		e = n.ManifestLinkToUpstream(ctx, maniData.ID, upID)
		if e != nil {
			return utils.ErrorHandler(l, span, e, "ManifestLinkToUpstream failed")
		}
	} else {
		return utils.ErrorHandler(l, span, e, "could not find upstream with digest '%s'", maniData.Upstream.Digest.String())
	}
	k := fmt.Sprintf("%s/%s", maniData.Config.Cfg.OS, maniData.Config.Cfg.Architecture)
	if maniData.Config.Cfg.Variant != "" {
		k = fmt.Sprintf("%s/%s", k, maniData.Config.Cfg.Variant)
	}
	platId := fmt.Sprintf("platform:%s#id", k)
	platID, e := br.GetInt64(platId)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "could not find platform id for key '%s'", platId)
	}
	e = n.LinkManifestToPlatform(ctx, maniData.ID, platID)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "could not link manifestdata to platform")
	}
	// Slot-centred wiring: Slot(repo, tag, generic) -> HAS_PUSHED -> content-leaf
	n.setupSlotForLeaf(ctx, br, label, maniData.Upstream.Repo, maniData.Upstream.Tag, "generic", maniData.ID)
	return
}
