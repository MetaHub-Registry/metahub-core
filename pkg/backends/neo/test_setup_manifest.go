package neo

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// Bootstrap_ManifestWrapper handles the legacy "manifest:<label>" bootstrap
// cases. As of deprecate-manifest-wrapper Phase 4 these no longer create a
// :ManifestWrapper node — the content-leaf :Manifest (created via the
// "manifestdata:<label>" bootstrap and Setup_Manifest) is the canonical node.
//
// To keep the ~36 test sites that read "manifest:<label>#id" working without
// churn, these helpers alias "manifest:<label>#id" to the content-leaf id
// ("manifestdata:<label>#id") when it is already present. When the content
// leaf has not been set up yet (the "manifest:" bootstrap commonly precedes
// "manifestdata:"), Setup_Manifest / Setup_OrasManifest will populate the
// alias once the leaf exists. The Upstream (:ORIGINAL) and Platform
// (:HAS_PLATFORM) edges are wired by the content-leaf setup, so there is
// nothing to create here.
func (n Neo) Bootstrap_ManifestWrapper(ctx context.Context, bs utils.Bootstrap, br *utils.BootstrapResult, b string) (err error) {
	switch b {
	case "ft_arm64", "ft_zen", "ft_zen3", "ft_amd64", "ft_mpich_amd64", "ft_oras_cyclonedx":
		n.aliasManifestToLeaf(ctx, br, manifestLabelForWrapper(b))
	default:
		return fmt.Errorf("no Manifest Setup for '%s'", b)
	}
	return
}

// manifestLabelForWrapper maps a wrapper-bootstrap key to the content-leaf
// label used by the "manifestdata:<label>" bootstrap. The mpich case has no
// dedicated content-leaf fixture, so it maps to itself (alias remains unset
// until/unless a matching leaf appears, which is fine for the tests that use
// it).
func manifestLabelForWrapper(b string) string {
	switch b {
	case "ft_mpich_amd64":
		return "ft_mpich_amd64"
	default:
		return b
	}
}

// aliasManifestToLeaf sets "manifest:<label>#id" to the content-leaf id
// recorded under "manifestdata:<label>#id" when the latter is present. It is a
// no-op (non-fatal) when the content leaf has not been created yet — the
// content-leaf setup will populate the alias.
func (n Neo) aliasManifestToLeaf(ctx context.Context, br *utils.BootstrapResult, label string) {
	if leafID, e := br.GetInt64(fmt.Sprintf("manifestdata:%s#id", label)); e == nil {
		br.Add(fmt.Sprintf("manifest:%s#id", label), fmt.Sprint(leafID))
	}
}

// Setup_Manifest_FT_oras_cyclonedx is retained as an explicit no-op shim for
// backward compatibility. Wrapper-node creation has been removed; the ORAS
// content-leaf is created by Setup_OrasManifest and attached to a Slot.
func (n Neo) Setup_Manifest_FT_oras_cyclonedx(ctx context.Context, br *utils.BootstrapResult) (err error) {
	n.aliasManifestToLeaf(ctx, br, "ft_oras_cyclonedx")
	return errors.WithStack(err)
}
