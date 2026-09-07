package magazine

import (
	"context"
	"fmt"
	"net/http"

	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/backends/neo"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	pkgNameML = "magazine/manifestlist.go"
)

func (mag Magazine) ComputeManifestlistForCTTP(ctx context.Context, er *types.ExprRes, ctpp types.CollectionTagProfilePair) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameML,
		"func":    "ComputeManifestlistForCTTP",
		"cttp":    ctpp.String(),
	})
	defer span.End()
	if er.IsEmpty() {
		return utils.NewError(l, span, "ExprRes is empty, skipping manifestlist computation for CTTP: %s", ctpp.String())
	}
	span.SetAttributes(attribute.String("cttp", ctpp.String()))
	utils.Info(l, span, "Compute Manifest Lists for CTTP: %s", ctpp.String())
	utils.Info(l, span, "Repo: '%s'", ctpp.String())
	ml := types.Index{
		SchemaVersion: 2,
		MediaType:     v1.MediaTypeImageIndex,
		Manifests:     []types.Manifest{},
	}
	mds, err := mag.filterManifestsForCTTP(ctx, l, span, er, ctpp, &ml)
	if err != nil {
		return err
	}
	if len(mds) == 0 {
		utils.Warn(l, span, "No Manifests for profile: %s", ctpp.Profile.Name)
		return nil
	}
	utils.Info(l, span, "Storing Index with %d Manifests for CTTP: %s", len(mds), ctpp.String())
	mlID, err := mag.persistComputedIndex(ctx, l, span, &ml)
	if err != nil {
		return err
	}
	// Path Z task 6 phase B: wire the Slot POINTS_TO chain EARLY in the
	// function body — right after the Index is persisted, before the
	// wrapper-era LinkManifestListTo{Manifest,Collection,CollectionTag,Profile}
	// calls. The early placement means the Slot-based state is always
	// produced for every successful compute, even when a pre-existing
	// flake in ManifestGetByDigest / LinkManifestListToManifest
	// causes the wrapper linking loop to fail partway through. Since the
	// Slot chain is the new source of truth, its durability matters more
	// than the wrapper links that are scheduled for deletion in Phase C2.
	mag.wireSlotPointsToOnCompute(ctx, l, span, ctpp.Repo.Repo, ctpp.Repo.ColTag.Tag, ctpp.Profile.Label, mlID)
	return mag.linkComputedIndexToWrapperEra(ctx, l, span, ctpp, mlID, mds)
}

// filterManifestsForCTTP runs the ExprRes-based filter + per-manifest
// profile annotation check to decide which manifests belong in the
// computed Index. The matching manifests are appended to both the
// returned slice (used by the wrapper-era linking loop) and to
// ml.Manifests (used by ToOCIBytes to build the stored blob). Factored
// out of ComputeManifestlistForCTTP to keep its cognitive complexity
// below the SonarQube threshold.
func (mag Magazine) filterManifestsForCTTP(ctx context.Context, l *logrus.Entry, span trace.Span, er *types.ExprRes, ctpp types.CollectionTagProfilePair, ml *types.Index) ([]types.Manifest, error) {
	tmpMDS := er.FilterByCTPP(ctx, ctpp)
	utils.Debug(l, span, "MDS: %d", len(tmpMDS))
	mds := make([]types.Manifest, 0, len(tmpMDS))
	for _, m := range tmpMDS {
		utils.Debug(l, span, "ManifestID:%d, Platform:%s", m.ID, m.Platform.GetLabel())
		// Use annotation-only check: manifests from parse collections have no
		// UPLOADED_BY/PULLED_BY creation relationship to a profile, so
		// ManifestCheckProfile would always return false.
		ok, e := mag.Database.ManifestCheckProfileAnnotations(ctx, m, ctpp.Profile)
		if e != nil {
			return nil, utils.ErrorHandler(l, span, e, "Failed to check profile")
		}
		if !ok {
			continue
		}
		utils.Debug(l, span, "Adding Manifest to Index: %s", m.Upstream.String())
		mds = append(mds, m)
	}
	// Collapse a re-pushed mutable runtime tag (same upstream reg:repo:tag on the
	// same platform) to its latest manifest, so the computed index never carries
	// two same-platform members for one tag — which docker resolves
	// nondeterministically and can serve the stale base (e.g. a runtime image
	// rebuilt with a new entrypoint that never takes effect).
	mds = types.DedupeLatestByUpstreamTagPlatform(mds)
	// Slot-scoped latest-push-wins: HAS_PUSHED is an append-only log, so a
	// profile re-pushing different content to the same tag (e.g. a re-tagged
	// image whose Upstream triple differs from the earlier push) would
	// otherwise accumulate every historical same-platform manifest into the
	// slot's computed index — the upstream-keyed dedupe above cannot collapse
	// those. Per platform, keep only the manifest from the most recent push:
	// the slot's own if the profile pushed to the tag itself, otherwise the
	// (repo, tag)-wide history so a passive profile's view still follows the
	// registry tag. Platforms with no push-log entry (multi-arch union
	// members) are untouched.
	ts, tsErr := mag.Database.SlotLatestPushTimestamps(ctx, ctpp.Repo.Repo, ctpp.Tag.Tag, ctpp.Profile.Label)
	if tsErr != nil {
		utils.Warn(l, span, "SlotLatestPushTimestamps failed (non-fatal, skipping latest-push dedupe): %v", tsErr)
	} else {
		if len(ts) == 0 {
			globalTs, gErr := mag.Database.RepoTagLatestPushTimestamps(ctx, ctpp.Repo.Repo, ctpp.Tag.Tag)
			if gErr != nil {
				utils.Warn(l, span, "RepoTagLatestPushTimestamps failed (non-fatal, skipping latest-push dedupe): %v", gErr)
			} else {
				ts = globalTs
			}
		}
		mds = latestPerPlatformForSlot(mds, ts)
	}
	ml.Manifests = append(ml.Manifests, mds...)
	return mds, nil
}

// latestPerPlatformForSlot applies latest-push-wins per platform to a slot's
// computed-index candidates. ts maps manifest node ID -> the manifest's latest
// HAS_PUSHED timestamp (slot-scoped, or (repo, tag)-wide as fallback).
//
//   - Candidates with no entry in ts are union members (pushed via another
//     profile's slot, or seeded by fixtures). They survive unless their
//     platform was pushed via this slot/tag — the author's own latest push is
//     the authoritative statement for that platform.
//   - Within a platform group holding at least one ts-carrying member, exactly
//     one manifest survives: the highest timestamp (ties broken by the higher
//     node ID for determinism).
//
// An empty ts map disables the dedupe entirely (no push log — fixtures).
// The relative order of surviving manifests is preserved so the computed
// index blob stays reproducible.
type platformGroup struct {
	hasTS   bool
	bestIdx int
	bestTS  int64
}

// isBetterCandidate reports whether a manifest with timestamp t and node ID
// mdID beats the current best in group g (newer timestamp wins; ties broken by
// higher node ID for determinism).
func isBetterCandidate(g *platformGroup, t, mdID, bestMDID int64) bool {
	return !g.hasTS || t > g.bestTS || (t == g.bestTS && mdID > bestMDID)
}

func latestPerPlatformForSlot(mds []types.Manifest, ts map[int64]int64) []types.Manifest {
	if len(ts) == 0 || len(mds) < 2 {
		return mds
	}
	groups := map[string]*platformGroup{}
	for i, md := range mds {
		label := md.Platform.GetLabel()
		g, ok := groups[label]
		if !ok {
			g = &platformGroup{bestIdx: -1}
			groups[label] = g
		}
		if t, pushed := ts[md.ID]; pushed {
			var bestMDID int64
			if g.bestIdx >= 0 {
				bestMDID = mds[g.bestIdx].ID
			}
			if isBetterCandidate(g, t, md.ID, bestMDID) {
				g.hasTS = true
				g.bestTS = t
				g.bestIdx = i
			}
		}
	}
	out := make([]types.Manifest, 0, len(mds))
	for i, md := range mds {
		if g := groups[md.Platform.GetLabel()]; g.hasTS && i != g.bestIdx {
			// An older same-platform push (or a union member) loses to the
			// slot's latest push for that platform.
			continue
		}
		out = append(out, md)
	}
	return out
}

// persistComputedIndex stores the computed Index bytes in minio, MERGEs
// the :Index node in neo4j via InsertManifestList, and tags it with
// origin=computed via SetIndexOrigin (first-writer-wins so a previously
// client-pushed digest stays as origin=client-pushed).
func (mag Magazine) persistComputedIndex(ctx context.Context, l *logrus.Entry, span trace.Span, ml *types.Index) (int64, error) {
	mlBlob := ml.ToOCIBytes()
	mlDigest := digest.FromBytes(mlBlob)
	if putErr := mag.Minio.ManifestsPutObj(ctx, mlDigest.String(), mlBlob); putErr != nil {
		return 0, utils.ErrorHandler(l, span, putErr, "failed ManifestsPutObj")
	}
	mlID, insertErr := mag.Database.InsertManifestList(ctx, mlDigest, ml.MediaType, len(mlBlob))
	if insertErr != nil {
		return 0, utils.ErrorHandler(l, span, insertErr, "failed InsertManifestList")
	}
	// Path Z task 8.2 / design D8: tag the Index with origin=computed so
	// the pull path can distinguish server-computed indexes from
	// client-pushed indexes. First-writer-wins semantics: if this Index
	// digest was previously pushed by a client (origin=client-pushed),
	// the SET statement is a no-op and the client-pushed origin is
	// preserved.
	if originErr := mag.Database.SetIndexOrigin(ctx, mlID, neo.IndexOriginComputed); originErr != nil {
		// Non-fatal: the Index is still usable without the origin
		// property; downstream consumers that need origin can fall back
		// to "unknown".
		utils.Debug(l, span, "SetIndexOrigin(computed) failed (non-fatal): %v", originErr)
	}
	return mlID, nil
}

// linkComputedIndexToWrapperEra runs the legacy wrapper-era links from
// the Index to Manifest/Collection/CollectionTag/Profile nodes. These
// links are preserved during the Phase A/B/C migration window so the
// pre-Path-Z pull path continues to work; Phase C3 will delete them in
// favour of the Slot-based state established by wireSlotPointsToOnCompute
// above. Factored out of ComputeManifestlistForCTTP to keep its
// cognitive complexity below the SonarQube threshold.
func (mag Magazine) linkComputedIndexToWrapperEra(ctx context.Context, l *logrus.Entry, span trace.Span, ctpp types.CollectionTagProfilePair, mlID int64, mds []types.Manifest) error {
	c := ctpp.Repo
	ct := ctpp.Repo.ColTag
	p := ctpp.Profile
	// Link Index to Manifest
	for _, md := range mds {
		utils.Debug(l, span, " Linking Manifest ID:%d to Index", md.ID)
		mdID, found, getErr := mag.Database.ManifestGetByDigest(ctx, md.Digest)
		if getErr != nil {
			return utils.ErrorHandler(l, span, getErr, "failed ManifestGetByDigest")
		}
		if !found {
			return utils.NewError(l, span, "Manifest not found by digest: %s", md.Digest)
		}
		md.ID = mdID.ID
		if linkErr := mag.Database.LinkManifestListToManifest(ctx, mlID, md.ID); linkErr != nil {
			return utils.ErrorHandler(l, span, linkErr, "while linking Index to Manifest")
		}
	}
	// Link Index to Repo
	if e := mag.Database.LinkManifestListToCollection(mlID, c.ID); e != nil {
		return utils.ErrorHandler(l, span, e, "failed LinkManifestListToCollection")
	}
	// Prune stale Tag→REF edges before linking the new index. Without this,
	// every recompute leaves an orphaned Tag-[:REF]->oldIndex alongside the new
	// one — the wrapper-era query layer (Tag-REF→Index) then matches both,
	// surfacing stale indexes in the UI. Mirrors the profile pattern above.
	if e := mag.Database.UnLinkCollectionTagFromOtherIndexes(mlID, ct.ID); e != nil {
		return utils.ErrorHandler(l, span, e, "failed UnLinkCollectionTagFromOtherIndexes")
	}
	// Link Index to CollectionTag
	if e := mag.Database.LinkManifestListToCollectionTag(mlID, ct.ID); e != nil {
		return utils.ErrorHandler(l, span, e, "failed LinkManifestListToCollectionTag")
	}
	// Link Index to Profile — but first detach the profile from every other
	// Index in this collection. The Profile→Index REF edge drives the
	// profile-scoped serve resolution; without the detach a re-push leaves the
	// old attachment in place and the resolver matches two indexes for
	// (profile, repo, tag), serving stale content nondeterministically.
	if e := mag.Database.UnLinkProfileFromOtherRepoIndexes(mlID, p.ID, c.ID); e != nil {
		return utils.ErrorHandler(l, span, e, "failed UnLinkProfileFromOtherRepoIndexes")
	}
	if e := mag.Database.LinkManifestListToProfile(mlID, p.ID); e != nil {
		return utils.ErrorHandler(l, span, e, "failed LinkManifestListToProfile")
	}
	return nil
}

// wireSlotPointsToOnCompute updates the Slot's POINTS_TO edge to target
// the freshly-computed Index for a single (repo, tag, profile) triple.
// Extracted from ComputeManifestlistForCTTP to keep that function's
// cognitive complexity manageable. Non-fatal by design: if any step
// fails, the legacy wrapper-based links are already in place, so the
// pull path can still resolve the index via the wrapper walk.
func (mag Magazine) wireSlotPointsToOnCompute(ctx context.Context, l *logrus.Entry, span trace.Span, repo, tag, profile string, indexID int64) {
	if repo == "" || tag == "" || profile == "" {
		utils.Debug(l, span, "wireSlotPointsToOnCompute: empty triple component, skipping (repo=%q tag=%q profile=%q)", repo, tag, profile)
		return
	}
	slot, slotErr := mag.Database.SlotGetCreate(ctx, repo, tag, profile)
	if slotErr != nil {
		utils.Debug(l, span, "wireSlotPointsToOnCompute: SlotGetCreate failed (non-fatal): %v", slotErr)
		return
	}
	if linkErr := mag.Database.SlotLinkPointsTo(ctx, slot.ID, indexID); linkErr != nil {
		utils.Debug(l, span, "wireSlotPointsToOnCompute: SlotLinkPointsTo failed (non-fatal): %v", linkErr)
		return
	}
	utils.Debug(l, span, "wireSlotPointsToOnCompute: slot %d (%s:%s#%s) now POINTS_TO index %d", slot.ID, repo, tag, profile, indexID)
}

func (mag Magazine) ComputeManifestlists(ctx context.Context) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameML,
		"func":    "ComputeManifestlists",
	})
	defer span.End()
	err = mag.Database.RuleEvalAll(ctx)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "while evaluating all rules")
		return
	}
	rls, err := mag.Database.RulesGetAll(ctx)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "failed to get rules")
		return
	}
	utils.Debug(l, span, "Rules fetched: %d", len(rls))
	er := types.NewExprRes()
	for _, rl := range rls {
		label := fmt.Sprintf("%s:%s", rl.Name, rl.Tag)
		utils.Debug(l, span, "Processing Rule: %s", label)
		mds, e := mag.Database.ManifestsMatchingRule(ctx, rl)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to get manifestdata by rule")
			return
		}
		for _, md := range mds {
			utils.Debug(l, span, "  manifestdata: %s", md.Label)
			er.Add(label, md)
		}
	}
	if er.IsEmpty() {
		utils.Warn(l, span, "No manifestdata found in ExprRes, skipping manifestlist computation")
		return
	}
	repoRegex, ok := mag.Config.Args["repoRegex"]
	if !ok || repoRegex == "" {
		repoRegex = ".*"
	}
	tagRegex, ok := mag.Config.Args["tagRegex"]
	if !ok || tagRegex == "" {
		tagRegex = ".*"
	}
	prfRegex, ok := mag.Config.Args["profileRegex"]
	if !ok || prfRegex == "" {
		prfRegex = ".*"
	}
	utils.Info(l, span, "Computing manifestlists for regexes: %s, %s, %s", repoRegex, tagRegex, prfRegex)
	ctpps, err := mag.Database.CollectionTagProfilePairGetRegexFiltered(ctx, repoRegex, tagRegex, prfRegex)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "failed to get profile manifestdata pairs")
		return
	}
	for _, ctpp := range ctpps {
		err = mag.ComputeManifestlistForCTTP(ctx, er, ctpp)
		if err != nil {
			err = utils.ErrorHandler(l, span, err, "failed to compute manifestlist for cttp: %v", ctpp)
			continue
		}
	}
	return
}

// ComputeManifestlistsForRepoTag is the push-time entry point that rebuilds
// every per-profile Index linked to (repo, tag) after a successful
// single-manifest push. It is the per-(repo,tag) cousin of the full-system
// ComputeManifestlists sweep.
//
// Workflow:
//  1. Resolve the Repo + CollectionTag for (repo, tag) (idempotent get-or-
//     create — the caller has just pushed, so the nodes already exist).
//  2. Fetch every Manifest currently linked to that (collection, tag).
//     If the result is empty, return an error: the caller asked us to compute
//     for a tag that has no manifestdata, which means either the push has not
//     yet been persisted or the wrong (repo, tag) was provided.
//  3. If pusherPrf != nil, call ProfileInjectDeterministicAnnotations on every
//     manifestdata so the pusher's deterministic ("anno eq") rules become
//     concrete annotations the rule eval can match against. Non-deterministic
//     rules are skipped at the database layer.
//  4. Run RuleEvalAll so the freshly injected annotations produce
//     Rule-[:TRUE]->Annotation edges the CollectionTagProfilePair query
//     depends on. (A scoped RuleEvalForManifest is a documented follow-up.)
//  5. Build an ExprRes by walking every Rule and the manifestdata it now
//     matches — same shape as ComputeManifestlists.
//  6. Iterate every Profile in the system. For each, construct a CTPP keyed on
//     (col, tag, profile) hydrated with the tag's manifestdata, and call
//     ComputeManifestlistForCTTP. The inner helper short-circuits on empty
//     matches and stores any non-empty index via StoreManifestListUpload.
//
// pusherPrf may be nil — in that case the annotation-injection step is
// skipped, but the rule eval and per-profile rebuild still run.
func (mag Magazine) ComputeManifestlistsForRepoTag(ctx context.Context, repo, tag string, pusherPrf *types.Profile) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameML,
		"func":    "ComputeManifestlistsForRepoTag",
		"repo":    repo,
		"tag":     tag,
	})
	defer span.End()

	// 1. Fetch the RAW manifestdata for (repo, tag). Path Z task 6 phase B
	// introduces a Slot-first walker that unions HAS_PUSHED edges across
	// every Slot for the (repo, tag) pair. If at least one Slot exists
	// (post-phase-A pushes always create one), the Slot walker is the
	// source of truth because it preserves per-triple attribution. If
	// no Slot exists yet (pre-phase-A pushes, upgraded databases, or
	// bootstrap-seeded fixtures), fall back to ManifestGetRawByRepoTag
	// which walks via `(md:Manifest)-[:ORIGINAL]->(u:Upstream)`.
	//
	// Both walkers are deliberately scoped to the RAW push log rather
	// than the profile-filtered ManifestGetByCollectionTag:
	//
	//   a) ManifestGetByCollectionTag is a CTPP-hydration helper that
	//      filters out manifestdata not yet matching any Profile's
	//      annotation rules — at push time the just-persisted md has no
	//      profile linkage yet, so it would always return empty.
	//
	//   b) Walking via the legacy `:ManifestWrapper` wrapper node only returns
	//      the most recently pushed manifestdata because the wrapper is
	//      keyed on a UNIQUE `host/repo:tag` label and its outgoing edge
	//      is replaced on every push. For multi-arch aggregation we need
	//      the full push history, which is preserved on the Slot side
	//      (HAS_PUSHED is append-only) and on the Upstream side (each
	//      push creates its own Upstream {reg, repo, tag, digest} node).
	mds, err := mag.fetchRawManifestsForRepoTag(ctx, l, span, repo, tag)
	if err != nil {
		return utils.ErrorHandler(l, span, err, "failed to fetch manifestdata for %s:%s", repo, tag)
	}
	if len(mds) == 0 {
		return utils.NewError(l, span, "no manifestdata found for %s:%s — nothing to compute", repo, tag)
	}
	utils.Info(l, span, "computing manifestlists for %s:%s with %d manifestdata rows", repo, tag, len(mds))

	// 2. Resolve the Repo + CollectionTag. The CTPP builder in
	// step 6 needs the Repo object, so we fetch it after the raw
	// md lookup (idempotent get-or-create).
	col, _, err := mag.Database.CollectionGetCreateByRepoTag(ctx, repo, tag)
	if err != nil {
		return utils.ErrorHandler(l, span, err, "failed to resolve collection for %s:%s", repo, tag)
	}

	// 3. Inject the pusher profile's deterministic annotations onto every
	// manifestdata in the tag. Non-deterministic rules are silently skipped
	// at the database layer.
	if err = mag.injectPusherAnnotations(ctx, l, span, pusherPrf, mds); err != nil {
		return err
	}

	// 4. Re-evaluate every rule so the new annotations produce
	// Rule-[:TRUE]->Annotation edges. This is the same coarse step
	// ComputeManifestlists uses; a scoped per-manifestdata version is a
	// documented follow-up.
	if err = mag.Database.RuleEvalAll(ctx); err != nil {
		return utils.ErrorHandler(l, span, err, "failed to re-evaluate rules after annotation injection")
	}

	// 5. Build the ExprRes shared by all per-profile rebuilds.
	er, err := mag.buildExprResForAllRules(ctx, l, span)
	if err != nil {
		return err
	}

	// 6. Iterate every profile and rebuild its per-(col, tag) index.
	return mag.rebuildIndexesPerProfile(ctx, l, span, rebuildCtx{
		col:       col,
		mds:       mds,
		er:        er,
		repo:      repo,
		tag:       tag,
		pusherPrf: pusherPrf,
	})
}

// fetchRawManifestsForRepoTag implements the Slot-first raw-md walker
// for the compute path per Path Z task 6 phase B. It first enumerates
// every :Slot for the (repo, tag) pair via SlotGetAllForRepoTag, walks
// HAS_PUSHED for each Slot, and unions the results deduplicated by
// manifest ID. If the Slot walk returns zero manifests (either because
// no Slots exist yet, or because the Slots that exist have no
// HAS_PUSHED edges — e.g. a bootstrap-seeded fixture that built the
// :Slot via SlotGetCreate but did not call SlotAppendPushedManifest),
// the helper falls back to the Upstream-based
// ManifestGetRawByRepoTag walker from Path Y.
//
// The fallback is deliberately loose: the Slot-based path is the
// intended steady state, but during the Path Z migration window there
// are legitimate sources of :Slot-less data (upstream pull fixtures,
// legacy test bootstraps, pre-phase-A pushes on upgraded databases).
// Once Phase C deletes the legacy wrapper code and Phase D normalizes
// the edge labels, the fallback can be removed in a future cleanup
// pass.
func (mag Magazine) fetchRawManifestsForRepoTag(ctx context.Context, l *logrus.Entry, span trace.Span, repo, tag string) ([]types.Manifest, error) {
	slots, err := mag.Database.SlotGetAllForRepoTag(ctx, repo, tag)
	if err != nil {
		// Slot lookup is a NEW code path (Path Z) and a failure here
		// might indicate a graph-schema drift. Don't mask it as
		// "no Slot found"; surface the error to the caller.
		return nil, utils.ErrorHandler(l, span, err, "SlotGetAllForRepoTag failed for %s:%s", repo, tag)
	}
	utils.Debug(l, span, "SlotGetAllForRepoTag: %d slots for %s:%s", len(slots), repo, tag)
	if len(slots) == 0 {
		// No Slot exists yet — pre-phase-A push, upgraded data, or
		// bootstrap-seeded fixture. Fall back to the Upstream walker.
		utils.Debug(l, span, "no Slot for %s:%s, falling back to ManifestGetRawByRepoTag", repo, tag)
		return mag.Database.ManifestGetRawByRepoTag(ctx, repo, tag)
	}

	// Union HAS_PUSHED across every Slot for the (repo, tag), dedupe by
	// manifest ID. The dedupe key is the neo4j node ID rather than the
	// digest so that two distinct pushes of the same digest across
	// different profiles collapse to a single entry (which is correct:
	// same content, even if attribution is split across profiles).
	seen := map[int64]struct{}{}
	unioned := make([]types.Manifest, 0)
	for _, slot := range slots {
		slotMds, e := mag.Database.ManifestGetAllForSlot(ctx, slot.ID)
		if e != nil {
			// Per-slot failure is logged but doesn't abort the union;
			// the other slots may still have valid data. If EVERY slot
			// fails the result will be empty and we fall back below.
			utils.Debug(l, span, "ManifestGetAllForSlot failed for slot %d (%s): %v", slot.ID, slot.Key(), e)
			continue
		}
		for _, md := range slotMds {
			if _, dup := seen[md.ID]; dup {
				continue
			}
			seen[md.ID] = struct{}{}
			unioned = append(unioned, md)
		}
	}

	if len(unioned) == 0 {
		// Every Slot was empty (or every Slot walk failed). Fall back
		// to the Upstream walker to avoid returning a false empty on
		// a (repo, tag) that does have pushed manifests via the
		// legacy path.
		utils.Debug(l, span, "Slot walker returned 0 manifests for %s:%s, falling back to ManifestGetRawByRepoTag", repo, tag)
		return mag.Database.ManifestGetRawByRepoTag(ctx, repo, tag)
	}
	utils.Debug(l, span, "Slot walker returned %d unioned manifests for %s:%s", len(unioned), repo, tag)
	return unioned, nil
}

// injectPusherAnnotations applies the pusher profile's deterministic
// ("anno eq") rules as annotations on every provided Manifest. Non-
// deterministic rules are silently counted as skipped at the database
// layer. Returns early (nil) if pusherPrf is nil.
func (mag Magazine) injectPusherAnnotations(ctx context.Context, l *logrus.Entry, span trace.Span, pusherPrf *types.Profile, mds []types.Manifest) error {
	if pusherPrf == nil {
		return nil
	}
	var totalInjected, totalSkipped int
	for _, md := range mds {
		injected, skipped, e := mag.Database.ProfileInjectDeterministicAnnotations(ctx, pusherPrf.ID, md.ID)
		if e != nil {
			return utils.ErrorHandler(l, span, e, "failed to inject annotations from profile %d on manifestdata %d", pusherPrf.ID, md.ID)
		}
		totalInjected += injected
		totalSkipped += skipped
	}
	utils.Info(l, span, "injected %d / skipped %d rules from pusher profile %s across %d manifestdata", totalInjected, totalSkipped, pusherPrf.Label, len(mds))
	return nil
}

// buildExprResForAllRules walks every Rule in the database and populates a
// fresh ExprRes mapping rule-label -> matching Manifest. This is the
// same shape ComputeManifestlists builds; the per-profile CTPP rebuild
// consumes it to resolve which manifestdata match which profile's rule
// expression for the current (col, tag).
func (mag Magazine) buildExprResForAllRules(ctx context.Context, l *logrus.Entry, span trace.Span) (*types.ExprRes, error) {
	rls, err := mag.Database.RulesGetAll(ctx)
	if err != nil {
		return nil, utils.ErrorHandler(l, span, err, "failed to fetch all rules")
	}
	er := types.NewExprRes()
	for _, rl := range rls {
		label := fmt.Sprintf("%s:%s", rl.Name, rl.Tag)
		ruleMDs, e := mag.Database.ManifestsMatchingRule(ctx, rl)
		if e != nil {
			return nil, utils.ErrorHandler(l, span, e, "failed to get manifestdata by rule %s", label)
		}
		for _, md := range ruleMDs {
			er.Add(label, md)
		}
	}
	return er, nil
}

// rebuildCtx bundles the per-(repo,tag) state needed by the per-profile
// index rebuild loop. Grouping these fields in a struct keeps the helper
// signatures below the 7-parameter linter threshold (go:S107) while still
// keeping the contract explicit.
type rebuildCtx struct {
	col       types.Repo
	mds       []types.Manifest
	er        *types.ExprRes
	repo      string
	tag       string
	pusherPrf *types.Profile  // the profile that triggered the push (nil for batch compute)
	pins      []collectionPin // explicit (non-generic) profile pins for (repo, tag)
}

// collectionPin is one explicit `profile:` binding within a collection: the
// manifest mdID is pinned to profile (from a Slot{repo,tag,profile≠generic}
// -[:HAS_PUSHED]-> md edge). It drives FilterByCTPP's pin-exclusivity so a
// pinned runtime image never leaks into another profile's computed index.
type collectionPin struct {
	mdID    int64
	profile string
}

// gatherCollectionPins enumerates every explicit (non-generic) profile pin
// for (repo, tag) by walking each non-generic Slot's HAS_PUSHED edges. The
// result is applied to every per-profile CTPP so FilterByCTPP can drop
// manifests pinned to a different profile. Best-effort: any failure is
// warned and yields an empty/partial pin set, degrading gracefully to the
// historical rule-union membership rather than failing the compute.
func (mag Magazine) gatherCollectionPins(ctx context.Context, l *logrus.Entry, span trace.Span, repo, tag string) []collectionPin {
	pins := make([]collectionPin, 0)
	slots, err := mag.Database.SlotGetAllForRepoTag(ctx, repo, tag)
	if err != nil {
		utils.Warn(l, span, "gatherCollectionPins: SlotGetAllForRepoTag(%s:%s) failed (non-fatal, pin-exclusivity off): %v", repo, tag, err)
		return pins
	}
	for _, slot := range slots {
		// "generic"/"" is the non-exclusive multi-arch default (see
		// types.CollectionTagProfilePair.AddPinnedProfile, which also
		// ignores it). Skip early to avoid a needless HAS_PUSHED walk.
		if slot.Profile == "" || slot.Profile == "generic" {
			continue
		}
		slotMds, e := mag.Database.ManifestGetAllForSlot(ctx, slot.ID)
		if e != nil {
			utils.Warn(l, span, "gatherCollectionPins: ManifestGetAllForSlot(%d) failed (non-fatal): %v", slot.ID, e)
			continue
		}
		for _, md := range slotMds {
			pins = append(pins, collectionPin{mdID: md.ID, profile: slot.Profile})
		}
	}
	utils.Debug(l, span, "gatherCollectionPins: %d explicit pins for %s:%s", len(pins), repo, tag)
	return pins
}

// rebuildIndexesPerProfile walks every Profile in the database, constructs
// a CTPP for (col, tag, profile) hydrated with the target tag's
// manifestdata, and calls ComputeManifestlistForCTTP to rebuild its index.
// Per-profile failures are logged as warnings and skipped so one bad
// profile cannot poison the rest of the rebuild.
func (mag Magazine) rebuildIndexesPerProfile(ctx context.Context, l *logrus.Entry, span trace.Span, rc rebuildCtx) error {
	prfs, err := mag.Database.ProfileGetAll(ctx)
	if err != nil {
		return utils.ErrorHandler(l, span, err, "failed to fetch profiles")
	}
	// Gather explicit `profile:` pins ONCE for the (repo, tag): they are
	// profile-independent collection facts, so we compute them here and
	// share the slice across every per-profile CTPP below.
	rc.pins = mag.gatherCollectionPins(ctx, l, span, rc.repo, rc.tag)
	utils.Debug(l, span, "rebuilding indexes for %d profiles on %s:%s", len(prfs), rc.repo, rc.tag)
	for _, prf := range prfs {
		if e := mag.rebuildIndexForProfile(ctx, l, span, rc, prf); e != nil {
			return e
		}
	}
	return nil
}

// rebuildIndexForProfile constructs a CTPP for a single (col, tag, profile)
// triple, hydrates it with the manifestdata in `rc.mds`, and invokes
// ComputeManifestlistForCTTP. Hard CTPP-construction errors are returned;
// per-profile compute failures are logged as warnings and treated as
// best-effort (they do not fail the caller).
func (mag Magazine) rebuildIndexForProfile(ctx context.Context, l *logrus.Entry, span trace.Span, rc rebuildCtx, prf types.Profile) error {
	ctpp, e := types.NewCollectionTagProfilePair(rc.col, rc.col.ColTag, prf)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "failed to construct CTPP for profile %s", prf.Label)
	}
	mds := rc.mds
	// Apply manipulation ONLY when the profile is the one that triggered the
	// push. Other profiles get unmanipulated originals at compute time;
	// manipulation for them happens lazily on first pull.
	er := rc.er
	if rc.pusherPrf != nil && prf.Label == rc.pusherPrf.Label {
		manipulatedMds, manipErr := mag.ApplyManipulationsToManifests(ctx, prf, mds)
		if manipErr != nil {
			utils.Warn(l, span, "Manipulation failed for pusher profile %s (non-fatal): %v", prf.Label, manipErr)
		} else {
			mds = manipulatedMds
			// Augment ExprRes with any newly-created manipulated
			// manifests so FilterByCTPP's rule-index intersection
			// includes them. Rule evaluation for the manipulated
			// manifest runs inline here — rule → true → platform
			// edges are inherited via HAS_PLATFORM from the original,
			// so a platform rule that matched the original matches
			// the manipulated one too. Annotation rules are not yet
			// copied (manipulated manifests have no annotations).
			if augmented, augErr := mag.augmentExprResWithManipulated(ctx, l, span, rc.er, manipulatedMds); augErr != nil {
				utils.Warn(l, span, "augmentExprResWithManipulated failed for profile %s (non-fatal): %v", prf.Label, augErr)
			} else {
				er = augmented
			}
		}
	}
	for _, md := range mds {
		if addErr := ctpp.AddManifest(md); addErr != nil {
			return utils.ErrorHandler(l, span, addErr, "failed to add manifestdata to CTPP for profile %s", prf.Label)
		}
	}
	// Apply explicit-pin exclusivity: a manifest pinned to another profile
	// is dropped from this profile's index by FilterByCTPP. AddPinnedProfile
	// ignores the generic/empty default, so multi-arch collections are
	// unaffected.
	for _, p := range rc.pins {
		ctpp.AddPinnedProfile(fmt.Sprintf("%d", p.mdID), p.profile)
	}
	if computeErr := mag.ComputeManifestlistForCTTP(ctx, er, ctpp); computeErr != nil {
		utils.Warn(l, span, "ComputeManifestlistForCTTP failed for profile %s: %v", prf.Label, computeErr)
	}
	return nil
}

// augmentExprResWithManipulated appends manipulated manifests to a
// copy of the shared ExprRes so FilterByCTPP's rule-index intersection
// includes them. The approach is "by lineage": for every manipulated
// manifest we look up its MANIPULATED_FROM parent(s) and copy every
// rule the parent already matches onto the manipulated manifest.
// Rule semantics are preserved — a manipulated manifest is a superset
// of its origin (more layers, optionally mutated config) so any rule
// the origin satisfies remains satisfied by the manipulated variant.
//
// The original ExprRes is never mutated so other profiles (iterated in
// the same rebuildIndexesPerProfile loop) still see the untouched
// rule→manifest mapping.
func (mag Magazine) augmentExprResWithManipulated(ctx context.Context, l *logrus.Entry, span trace.Span, er *types.ExprRes, manipulatedMds []types.Manifest) (*types.ExprRes, error) {
	if len(manipulatedMds) == 0 {
		return er, nil
	}
	augmented := er.Clone()
	for _, manipMd := range manipulatedMds {
		origID, ok, err := mag.Database.ManifestGetManipulatedFrom(ctx, manipMd.ID)
		if err != nil {
			utils.Warn(l, span, "ManifestGetManipulatedFrom(%d) failed during augmentation: %v", manipMd.ID, err)
			continue
		}
		if !ok {
			// Not manipulated (the ApplyManipulationsToManifests call
			// left it unchanged). Nothing to augment.
			continue
		}
		// Every rule the origin matched is inherited by the manipulated
		// manifest. Walk every rule key in the base ExprRes to find
		// which ones contain the origin and record the manipulated ID
		// under the same keys.
		origIDStr := fmt.Sprintf("%d", origID)
		for rule, ids := range er.RuleManiMap {
			if !ids.Contains(origIDStr) {
				continue
			}
			augmented.Add(rule, manipMd)
		}
	}
	return augmented, nil
}

// ComputeManifestlistsForManiProfile computes manifestlists for a given Manifest and Profile
func (mag Magazine) ComputeManifestlistsForManiProfile(ctx context.Context, maniID, prfID int64) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameML,
		"func":    "ComputeManifestlistsForManiProfile",
	})
	defer span.End()
	l.Info("Compute Manifest Lists")
	err = mag.Database.RuleEvalAll(ctx)
	if err != nil {
		err = errors.Wrap(err, "while evaluating all rules")
		span.SetStatus(codes.Error, err.Error())
		span.AddEvent("log", trace.WithAttributes(
			attribute.String(attrLogSeverity, "error"),
			attribute.String(attrLogMessage, err.Error()),
		))
		span.RecordError(err)
		l.Error(err.Error())
		return
	}
	/// TODO: We need
	/// 1. Generate ExprRes? (a rule<>mdID mapping)
	rls, err := mag.Database.RulesGetAll(ctx)
	if err != nil {
		logrus.Errorf("failed to get rules: %v", err)
		return
	}
	er := types.NewExprRes()
	for _, rl := range rls {
		label := fmt.Sprintf("%s:%s", rl.Name, rl.Tag)
		l.Warnf("Rule: %s", label)
		mds, e := mag.Database.ManifestsMatchingRule(ctx, rl)
		if e != nil {
			err = errors.Wrapf(e, "failed to get manifestdata by rule: %v", err)
			return
		}
		for _, md := range mds {
			er.Add(label, md)
		}
	}
	repoRegex, ok := mag.Config.Args["repoRegex"]
	if !ok || repoRegex == "" {
		repoRegex = ".*"
	}
	tagRegex, ok := mag.Config.Args["tagRegex"]
	if !ok || tagRegex == "" {
		tagRegex = ".*"
	}
	prfRegex, ok := mag.Config.Args["profileRegex"]
	if !ok || prfRegex == "" {
		prfRegex = ".*"
	}
	ctpps, err := mag.Database.CollectionTagProfilePairGetRegexFiltered(ctx, repoRegex, tagRegex, prfRegex)
	if err != nil {
		l.Errorf("failed to get profile manifestdata pairs: %v", err)
		return
	}

	for _, ctpp := range ctpps {
		utils.Warn(l, span, "CTTP.Manifest: %d", len(ctpp.Manifests))
		err = mag.ComputeManifestlistForCTTP(ctx, er, ctpp)
		if err != nil {
			err = errors.Wrapf(err, "failed to compute manifestlist for cttp: %v", ctpp)
			continue
		}
	}
	return
}

func (mag Magazine) IndexGetByRequest(ctx context.Context, req types.Request) (ml types.Index, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameML,
		"func":    "IndexGetByRequest",
		"repo":    req.Repo,
		"tag":     req.Tag,
		"digest":  utils.ShortenDigest(req.Digest),
	})
	defer span.End()
	_ = l
	// Fetch ML from SQLite
	ml, found, err = mag.Database.IndexGetByRequest(ctx, req)
	if err != nil {
		logrus.Errorf("Failed to get Index from Database: %v", err)
		return
	}
	if !found {
		// We did not find a Index
		return
	}
	return
}

func (mag Magazine) ManifestListGetByHTTPRequest(ctx context.Context, r *http.Request) (ml types.Index, found bool, err error) {
	ctx, span := tracer.Start(ctx, "ManifestListGetByHTTPRequest")
	defer span.End()
	req := types.NewRequest(ctx, r)
	msg := fmt.Sprintf("%-5s ManifestListHandler(%s, ", r.Method, req.Repo)
	if req.Tag == "" {
		msg += fmt.Sprintf("%s)", req.Digest)
	} else {
		msg += fmt.Sprintf("%s)", req.Tag)
	}
	if req.Profile != "generic" {
		msg += fmt.Sprintf(" - profile: %s", req.Profile)
	}
	logrus.Info(msg)
	ml, found, err = mag.IndexGetByRequest(ctx, req)
	if err != nil {
		return
	}
	return
}

// func (mag Magazine) ManifestListPutByRequest(ctx context.Context, req types.Request, buf []byte) (ml types.Index, err error) {
// 	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
// 		"package": pkgNameML,
// 		"func":    "ManifestListPutByRequest",
// 		"repo":    req.Repo,
// 		"tag":     req.Tag,
// 		"digest":  utils.ShortenDigest(req.Digest),
// 	})
// 	defer span.End()

//		err = mag.Database.ManifestListInsert(ctx, req, buf)
//		return
//	}

// StoreIndexUpload is the client-pushed OCI image-index upload handler.
// It is called from the metahub-api PUT /v2/<name>/manifests/<reference>
// handler when the body is an `application/vnd.oci.image.index.v1+json`
// (or docker v2 manifest-list equivalent) and implements Path Z design
// decision D8 — dual-storage for client-pushed indexes:
//
//  1. The raw OCI bytes are stored verbatim in minio keyed by the client
//     digest. This preserves byte-identity for pulls that ask for the
//     index by its client-computed digest, regardless of how we'd
//     serialise it server-side.
//  2. An :Index node is MERGEd in neo4j (first-writer-wins on digest) and
//     tagged with `origin = "client-pushed"` so the pull path and the
//     compute path can distinguish client uploads from server-computed
//     indexes. If this digest already exists because the compute path
//     produced the same index earlier, the origin stays as "computed"
//     (or whatever the first writer set it to).
//  3. Every member descriptor is validated to exist in the graph as a
//     content-leaf :Manifest node. Missing members cause the whole
//     upload to fail with an error the handler layer maps to
//     MANIFEST_BLOB_UNKNOWN. This matches the OCI spec requirement that
//     a client-pushed index must only reference already-pushed manifests.
//  4. For each member, a :REFERENCES edge is MERGEd with an `order`
//     property capturing the descriptor's position in the original
//     manifest list. The edge uses MERGE so repushes of the same index
//     don't create duplicate edges.
//  5. If req.Profile is non-empty, the pusher :Slot for the
//     (req.Repo, req.Tag, req.Profile) triple is resolved via
//     SlotGetCreate, and a :HAS_PUSHED_INDEX edge is appended to the
//     Slot. This gives the Slot's history log a record of every client
//     push (append-only, never deduplicated, timestamped).
//
// Path Z task 8.2. Task 8.1 renamed the function from the old
// StoreManifestListUpload spelling; a shim on the old name is kept below
// during the migration window so metahub-api handlers that still call
// the old name keep working until they migrate.
//
// Not yet implemented (deferred to a future session):
//   - Task 8.2's final bullet: invoke ComputeIndexForRepoTag (the
//     renamed ComputeManifestlistsForRepoTag from task 6.1). The rename
//     hasn't happened yet, and calling ComputeManifestlistsForRepoTag
//     from inside a push handler risks double-compute during the Path Z
//     migration window because the handler already calls
//     computeAfterManifestPush. Phase C3 will wire the recompute here.
//   - Task 8.3's full Slot-based IndexGetByRequest rewrite. Task 11's
//     manifests:null fix already makes IndexGetByRequest work via the
//     existing Profile->Index walk; the Slot-based variant is an
//     optimisation that can land later.
func (mag Magazine) StoreIndexUpload(ctx context.Context, req types.Request, ml *types.Index) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameML,
		"func":    "StoreIndexUpload",
		"repo":    req.Repo,
		"tag":     req.Tag,
		"profile": req.Profile,
		"digest":  utils.ShortenDigest(req.Digest),
	})
	defer span.End()
	utils.Debug(l, span, "%s", ml.ToOCIBytes())
	if err = mag.ensureIndexPersisted(ctx, l, span, ml); err != nil {
		return err
	}
	// Task 8.2: mark the Index with origin=client-pushed. First-writer-wins
	// semantics mean that if the same digest was previously computed
	// server-side, the origin stays as "computed"; only genuinely new
	// Index nodes get the "client-pushed" tag.
	if originErr := mag.Database.SetIndexOrigin(ctx, ml.ID, neo.IndexOriginClientPushed); originErr != nil {
		utils.Debug(l, span, "SetIndexOrigin(client-pushed) failed (non-fatal): %v", originErr)
	}
	if err = mag.attachIndexAnnotationsAndSubject(ctx, l, span, ml); err != nil {
		return err
	}
	if err = mag.linkIndexReferencesInOrder(ctx, l, span, ml); err != nil {
		return err
	}
	mag.wireSlotPushedIndexOnUpload(ctx, l, span, req, ml.ID)
	return nil
}

// ensureIndexPersisted either reuses an existing :Index for the given
// digest (idempotent re-push) or creates a fresh one by storing the raw
// OCI bytes in minio and MERGEing the :Index node in neo4j. On return,
// ml.ID is populated with the resolved node ID.
func (mag Magazine) ensureIndexPersisted(ctx context.Context, l *logrus.Entry, span trace.Span, ml *types.Index) error {
	existingML, err := mag.Database.ManifestListGetByDigest(ctx, ml.Digest)
	if err != nil {
		return utils.ErrorHandler(l, span, err, "while checking existing Index by digest")
	}
	if existingML != nil {
		utils.Info(l, span, "Index with Digest %s already exists (reuse), skipping insert but still running downstream wiring", ml.Digest)
		ml.ID = existingML.ID
		return nil
	}
	utils.Debug(l, span, "No existing Index found for digest, creating fresh")
	mlBlob := ml.ToOCIBytes()
	mlDigest, parseErr := digest.Parse(ml.Digest)
	if parseErr != nil {
		return utils.ErrorHandler(l, span, parseErr, "failed to parse manifestlist digest")
	}
	if putErr := mag.Minio.ManifestsPutObj(ctx, mlDigest.String(), mlBlob); putErr != nil {
		return utils.ErrorHandler(l, span, putErr, "failed ManifestsPutObj")
	}
	var insertErr error
	ml.ID, insertErr = mag.Database.InsertManifestList(ctx, mlDigest, ml.MediaType, len(mlBlob))
	if insertErr != nil {
		return utils.ErrorHandler(l, span, insertErr, "failed InsertManifestList")
	}
	return nil
}

// attachIndexAnnotationsAndSubject MERGEs each annotation on the Index
// node and, if the Index has a Subject descriptor, creates the
// corresponding subject link. Factored out of StoreIndexUpload to keep
// that function's cognitive complexity manageable.
func (mag Magazine) attachIndexAnnotationsAndSubject(ctx context.Context, l *logrus.Entry, span trace.Span, ml *types.Index) error {
	utils.Debug(l, span, "Adding %d annotations to Index", len(ml.Annotations))
	for key, value := range ml.Annotations {
		utils.Debug(l, span, " Adding annotation '%s':'%s' to Index", key, value)
		a := types.NewAnnotation(key, value)
		if annoErr := mag.Database.ManifestListAddAnnotation(ctx, ml.ID, &a); annoErr != nil {
			return utils.ErrorHandler(l, span, annoErr, "failed to add annotation to Index")
		}
	}
	if ml.Subject == nil {
		return nil
	}
	utils.Debug(l, span, "Has a subject: %s", utils.ShortenDigest(string(ml.Subject.Digest)))
	if subjErr := mag.Database.LinkSubjectByDigestMediaType(ctx, ml.ID, ml.Subject.Digest.String(), ml.Subject.MediaType); subjErr != nil {
		return utils.ErrorHandler(l, span, subjErr, "while linking Descriptor to Descriptor")
	}
	return nil
}

// linkIndexReferencesInOrder validates that every member :Manifest
// exists in the graph, then MERGEs :REFERENCES edges carrying the
// descriptor's position in the original OCI manifest list as the
// `order` property. Missing members fail with an error that includes
// the MANIFEST_BLOB_UNKNOWN marker so the caller (and the metahub-api
// handler layer above it) can map the error to the right OCI error
// code per the OCI spec.
func (mag Magazine) linkIndexReferencesInOrder(ctx context.Context, l *logrus.Entry, span trace.Span, ml *types.Index) error {
	for order, md := range ml.Manifests {
		utils.Debug(l, span, " Index contains Manifest: %s (order=%d)", md.Platform.Arch, order)
		mdID, found, getErr := mag.Database.ManifestGetByDigest(ctx, md.Digest)
		if getErr != nil {
			return utils.ErrorHandler(l, span, getErr, "failed ManifestGetByDigest for member '%s'", md.Digest)
		}
		if !found {
			// OCI spec: a client-pushed index must only reference already
			// pushed manifests. Return an error the handler maps to
			// MANIFEST_BLOB_UNKNOWN.
			return utils.NewError(l, span, "Manifest not found by digest: %s (MANIFEST_BLOB_UNKNOWN)", md.Digest)
		}
		md.ID = mdID.ID
		utils.Debug(l, span, " Linking Manifest ID:%d to Index with order=%d", md.ID, order)
		if linkErr := mag.Database.LinkIndexReferenceWithOrder(ctx, ml.ID, md.ID, order); linkErr != nil {
			return utils.ErrorHandler(l, span, linkErr, "while linking Index to Manifest with order")
		}
	}
	return nil
}

// wireSlotPushedIndexOnUpload resolves the pusher's :Slot for the
// (req.Repo, req.Tag, req.Profile) triple and appends a timestamped
// :HAS_PUSHED_INDEX edge to it. Non-fatal by design: any failure is
// debug-logged but the caller's push still succeeds because the
// primary Index upload and its :REFERENCES edges are already in place.
// Gated on req.Profile != "" so pre-Path-Z callers that don't thread
// profile through their request don't suddenly fail.
func (mag Magazine) wireSlotPushedIndexOnUpload(ctx context.Context, l *logrus.Entry, span trace.Span, req types.Request, indexID int64) {
	if req.Profile == "" {
		return
	}
	slot, slotErr := mag.Database.SlotGetCreate(ctx, req.Repo, req.Tag, req.Profile)
	if slotErr != nil {
		utils.Debug(l, span, "SlotGetCreate failed (non-fatal, index already persisted): %v", slotErr)
		return
	}
	if pushIdxErr := mag.Database.SlotAppendPushedIndex(ctx, slot.ID, indexID); pushIdxErr != nil {
		utils.Debug(l, span, "SlotAppendPushedIndex failed (non-fatal): %v", pushIdxErr)
		return
	}
	utils.Debug(l, span, "Slot %d HAS_PUSHED_INDEX -> Index %d (client-pushed)", slot.ID, indexID)
}

// StoreManifestListUpload is the pre-Path-Z name for StoreIndexUpload.
// Kept as a thin shim during the Path Z migration window so metahub-api
// handlers that still call the old name keep compiling. New code should
// call StoreIndexUpload directly. This shim will be removed in Phase C3
// together with the wrapper deletion.
//
// Deprecated: use StoreIndexUpload instead.
func (mag Magazine) StoreManifestListUpload(ctx context.Context, req types.Request, ml *types.Index) (err error) {
	return mag.StoreIndexUpload(ctx, req, ml)
}
