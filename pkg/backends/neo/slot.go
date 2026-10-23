// Package neo slot.go implements the Slot helper functions for the
// slot-centred graph model (Path Z). A Slot is the primary per-triple anchor
// node keyed on (repo, tag, profile). It owns the POINTS_TO edge to the
// currently-pointed-to Index, the HAS_HISTORY chain of previous POINTS_TO
// targets, and the HAS_PUSHED / HAS_PUSHED_INDEX append-only push logs.
//
// See design.md §D2-D4 of the refactor-graph-model-slot-centred change for the
// semantics and invariants enforced by these helpers.
package neo

import (
	"context"
	"fmt"
	"sort"

	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/trace"
)

const (
	pkgNameSlot = "neo/slot.go"
	// genericSlotProfile is the slot key for the un-manipulated/generic index.
	// An empty profile is the generic pull; the slot triple rejects empty
	// components, so normalise it to the sentinel used by the tests and API.
	genericSlotProfile = "generic"
)

// normalizeSlotProfile maps the empty (generic) profile to its slot sentinel.
func normalizeSlotProfile(profile string) string {
	if profile == "" {
		return genericSlotProfile
	}
	return profile
}

// slotFromResult decodes a Slot from a cypher result that returned at least
// id(s) AS slotID, s.repo AS slotRepo, s.tag AS slotTag, s.profile AS slotProfile.
func slotFromResult(res types.Result) (s types.Slot, err error) {
	if e := res.CheckKeys("slotID", "slotRepo", "slotTag", "slotProfile"); e != nil {
		return s, fmt.Errorf("slotFromResult: missing keys: %w", e)
	}
	s.ID, err = res.GetInt64("slotID")
	if err != nil {
		return s, fmt.Errorf("slotFromResult: getting slotID: %w", err)
	}
	s.Repo, err = res.GetStr("slotRepo")
	if err != nil {
		return s, fmt.Errorf("slotFromResult: getting slotRepo: %w", err)
	}
	s.Tag, err = res.GetStr("slotTag")
	if err != nil {
		return s, fmt.Errorf("slotFromResult: getting slotTag: %w", err)
	}
	s.Profile, err = res.GetStr("slotProfile")
	if err != nil {
		return s, fmt.Errorf("slotFromResult: getting slotProfile: %w", err)
	}
	return s, nil
}

// slotReturnClause is the cypher RETURN suffix that every Slot getter uses so
// slotFromResult can decode the result.
const slotReturnClause = ` RETURN id(s) AS slotID, s.repo AS slotRepo, s.tag AS slotTag, s.profile AS slotProfile`

// SlotGetCreate returns a Slot for the (repo, tag, profile) triple, creating
// it and its :Repo / :Tag / :Profile join edges if they do not already exist.
// Idempotent: calling with the same triple returns the same Slot ID and does
// not create duplicate nodes or edges. String-identity nodes (Repo, Tag,
// Profile) are MERGEd, so brand-new triples transparently create them.
//
// On creation the Slot node is initialised with s.created_at = timestamp().
// Subsequent calls with the same triple do not update this value.
func (n Neo) SlotGetCreate(ctx context.Context, repo, tag, profile string) (s types.Slot, err error) {
	l, _, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameSlot,
		"func":    "SlotGetCreate",
		"repo":    repo,
		"tag":     tag,
		"profile": profile,
	})
	defer span.End()
	profile = normalizeSlotProfile(profile)
	if repo == "" || tag == "" || profile == "" {
		return s, utils.NewError(l, span, "SlotGetCreate: empty triple component repo=%q tag=%q profile=%q", repo, tag, profile)
	}
	// Step 1: MERGE the string-identity nodes. Each gets its own statement so
	// the MERGE pattern stays simple (neo4j MERGE with multiple labels/props
	// in one statement has subtle semantics). We use `repo` as the :Repo key
	// to stay compatible with the existing test_setup.go fixtures which
	// create `(c:Repo {repo: "qnib/featuretest"})`. The spec's `:Repo {name}`
	// is aspirational; a future follow-up can add a `name` alias if needed.
	mergeRepo := fmt.Sprintf(`MERGE (r:Repo {repo: "%s"})`, repo)
	if _, e := n.Execute(mergeRepo); e != nil {
		return s, utils.ErrorHandler(l, span, e, "SlotGetCreate: merging Repo node")
	}
	// Tag is per-collection ({name, repo}) — see RepoLinkTag; a global `latest`
	// node cross-links every collection's manifests.
	mergeTag := fmt.Sprintf(`MERGE (t:Tag {name: "%s", repo: "%s"})`, tag, repo)
	if _, e := n.Execute(mergeTag); e != nil {
		return s, utils.ErrorHandler(l, span, e, "SlotGetCreate: merging Tag node")
	}
	mergeProfile := fmt.Sprintf(`MERGE (p:Profile {label: "%s"})`, profile)
	if _, e := n.Execute(mergeProfile); e != nil {
		return s, utils.ErrorHandler(l, span, e, "SlotGetCreate: merging Profile node")
	}
	// Step 2: MERGE the Slot with the compound triple. The compound UNIQUE
	// constraint on (:Slot {repo, tag, profile}) enforces exactly one Slot
	// per triple. ON CREATE sets the timestamp so re-hits don't reset it.
	mergeSlot := fmt.Sprintf(`MERGE (s:Slot {repo: "%s", tag: "%s", profile: "%s"}) ON CREATE SET s.created_at = timestamp()`, repo, tag, profile)
	mergeSlot += slotReturnClause
	res, e := n.Execute(mergeSlot)
	if e != nil {
		return s, utils.ErrorHandler(l, span, e, "SlotGetCreate: merging Slot node")
	}
	s, err = slotFromResult(res)
	if err != nil {
		return s, utils.ErrorHandler(l, span, err, "SlotGetCreate: decoding Slot from result")
	}
	// Step 3: MERGE the three join edges. These are idempotent because the
	// surrounding MERGE pattern already pinned the string-identity nodes.
	joinEdges := fmt.Sprintf(
		`MATCH (s:Slot {repo: "%s", tag: "%s", profile: "%s"}), (r:Repo {repo: "%s"}), (t:Tag {name: "%s", repo: "%s"}), (p:Profile {label: "%s"}) MERGE (s)-[:OF_REPO]->(r) MERGE (s)-[:OF_TAG]->(t) MERGE (s)-[:OF_PROFILE]->(p)`,
		repo, tag, profile, repo, tag, repo, profile,
	)
	if _, e := n.Execute(joinEdges); e != nil {
		return s, utils.ErrorHandler(l, span, e, "SlotGetCreate: merging join edges")
	}
	utils.Debug(l, span, "SlotGetCreate: slotID=%d", s.ID)
	return s, nil
}

// SlotGetByTriple returns the Slot matching the triple, without creating
// anything. `found` is false when no Slot exists for the triple.
func (n Neo) SlotGetByTriple(ctx context.Context, repo, tag, profile string) (s types.Slot, found bool, err error) {
	l, _, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameSlot,
		"func":    "SlotGetByTriple",
		"repo":    repo,
		"tag":     tag,
		"profile": profile,
	})
	defer span.End()
	profile = normalizeSlotProfile(profile)
	q := fmt.Sprintf(`MATCH (s:Slot {repo: "%s", tag: "%s", profile: "%s"})`, repo, tag, profile)
	q += slotReturnClause
	res, found, e := n.QueryOne(q)
	if e != nil {
		return s, false, utils.ErrorHandler(l, span, e, "SlotGetByTriple: executing query")
	}
	if !found {
		return s, false, nil
	}
	s, err = slotFromResult(res)
	if err != nil {
		return s, false, utils.ErrorHandler(l, span, err, "SlotGetByTriple: decoding Slot from result")
	}
	return s, true, nil
}

// slotsFromQuery runs q and decodes every record into a Slot list. fn is used
// to attribute errors to the public entry point in log/error messages.
func (n Neo) slotsFromQuery(l *logrus.Entry, span trace.Span, fn, q string) ([]types.Slot, error) {
	results, e := n.Query(q)
	if e != nil {
		return nil, utils.ErrorHandler(l, span, e, fn+": executing query")
	}
	slots := make([]types.Slot, 0, len(results))
	for _, r := range results {
		sl, e := slotFromResult(r)
		if e != nil {
			return nil, utils.ErrorHandler(l, span, e, fn+": decoding Slot")
		}
		slots = append(slots, sl)
	}
	return slots, nil
}

// SlotGetAllForRepoTag returns every Slot sharing a (repo, tag) pair, across
// all profiles. Used by the compute path to know which profiles need an Index
// rebuild when a new Manifest lands on (repo, tag).
func (n Neo) SlotGetAllForRepoTag(ctx context.Context, repo, tag string) (slots []types.Slot, err error) {
	l, _, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameSlot,
		"func":    "SlotGetAllForRepoTag",
		"repo":    repo,
		"tag":     tag,
	})
	defer span.End()
	q := fmt.Sprintf(`MATCH (s:Slot) WHERE s.repo = "%s" AND s.tag = "%s"`, repo, tag)
	q += slotReturnClause
	return n.slotsFromQuery(l, span, "SlotGetAllForRepoTag", q)
}

// SlotGetAllForRepo returns every Slot for a given repo, across all (tag,
// profile) pairs. Used by the catalog endpoint to enumerate what a repo
// currently serves.
func (n Neo) SlotGetAllForRepo(ctx context.Context, repo string) (slots []types.Slot, err error) {
	l, _, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameSlot,
		"func":    "SlotGetAllForRepo",
		"repo":    repo,
	})
	defer span.End()
	q := fmt.Sprintf(`MATCH (s:Slot) WHERE s.repo = "%s"`, repo)
	q += slotReturnClause
	return n.slotsFromQuery(l, span, "SlotGetAllForRepo", q)
}

// SlotLinkPointsTo replaces the Slot's current POINTS_TO edge with a new one
// pointing at indexID, archiving the previous target into HAS_HISTORY with a
// timestamp. The replacement and archival happen in a single cypher
// transaction so an observer never sees a Slot with zero POINTS_TO edges
// except for the very first link (which has no previous target to archive).
//
// Idempotent short-circuit: if the current POINTS_TO target is already
// indexID, the call is a no-op and returns nil without touching HAS_HISTORY.
// The idempotency check is scoped to the index *digest*, not the neo4j node
// ID — two calls producing the same computed index with different node IDs
// would still create a history entry, which is correct: a digest-identity
// check belongs to the compute path, not the graph layer.
func (n Neo) SlotLinkPointsTo(ctx context.Context, slotID, indexID int64) (err error) {
	l, _, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameSlot,
		"func":    "SlotLinkPointsTo",
		"slotID":  slotID,
		"indexID": indexID,
	})
	defer span.End()
	// Short-circuit: already pointing at indexID.
	checkQ := fmt.Sprintf(`MATCH (s:Slot)-[:POINTS_TO]->(i:Index) WHERE id(s) = %d AND id(i) = %d RETURN id(i) AS existingIndexID`, slotID, indexID)
	if _, found, e := n.QueryOne(checkQ); e != nil {
		return utils.ErrorHandler(l, span, e, "SlotLinkPointsTo: checking existing POINTS_TO")
	} else if found {
		utils.Debug(l, span, "SlotLinkPointsTo: slot already points at index %d, no-op", indexID)
		return nil
	}
	// Archive the current POINTS_TO (if any) into HAS_HISTORY, then delete
	// the old edge and create the new one. Done in a single statement so
	// there is no observable moment with zero POINTS_TO edges after the
	// first successful call.
	q := fmt.Sprintf(`MATCH (s:Slot) WHERE id(s) = %d `, slotID)
	q += `OPTIONAL MATCH (s)-[r:POINTS_TO]->(oldIdx:Index) `
	q += `FOREACH (_ IN CASE WHEN oldIdx IS NULL THEN [] ELSE [1] END | `
	q += `  MERGE (s)-[h:HAS_HISTORY]->(oldIdx) ON CREATE SET h.timestamp = timestamp() `
	q += `  DELETE r `
	q += `) `
	q += fmt.Sprintf(`WITH s MATCH (newIdx:Index) WHERE id(newIdx) = %d MERGE (s)-[:POINTS_TO]->(newIdx)`, indexID)
	if _, e := n.Execute(q); e != nil {
		return utils.ErrorHandler(l, span, e, "SlotLinkPointsTo: archive+replace transaction")
	}
	utils.Debug(l, span, "SlotLinkPointsTo: slot %d now points at index %d", slotID, indexID)
	return nil
}

// SlotAppendPushedManifest appends a :HAS_PUSHED edge from the Slot to the
// Manifest, with a timestamp property. The edge is NOT deduplicated: every
// call creates a new edge even if the same Manifest was already pushed to
// this Slot before. This makes HAS_PUSHED a true append-only log suitable
// for reconstructing push history.
func (n Neo) SlotAppendPushedManifest(ctx context.Context, slotID, manifestID int64, user string) (err error) {
	l, _, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameSlot,
		"func":       "SlotAppendPushedManifest",
		"slotID":     slotID,
		"manifestID": manifestID,
		"user":       user,
	})
	defer span.End()
	// The Slot/Manifest ids are internal int64s (safe to interpolate), but `user`
	// is the OIDC preferred_username — caller/attacker-influenced — so bind it as a
	// parameter ($user), never interpolate it into the Cypher. It stamps the
	// append-only HAS_PUSHED edge for per-user usage attribution; empty for
	// non-user (system/collection) bindings.
	q := fmt.Sprintf(
		`MATCH (s:Slot), (m:Manifest) WHERE id(s) = %d AND id(m) = %d CREATE (s)-[:HAS_PUSHED {timestamp: timestamp(), user: $user}]->(m)`,
		slotID, manifestID,
	)
	if _, e := n.ExecuteWithParams(q, map[string]interface{}{"user": user}); e != nil {
		return utils.ErrorHandler(l, span, e, "SlotAppendPushedManifest: creating HAS_PUSHED edge")
	}
	utils.Debug(l, span, "SlotAppendPushedManifest: slot %d pushed manifest %d (user=%q)", slotID, manifestID, user)
	return nil
}

// SlotAppendPushedIndex appends a :HAS_PUSHED_INDEX edge from the Slot to the
// Index, with a timestamp property. Same append-only semantics as
// SlotAppendPushedManifest: never deduplicated.
func (n Neo) SlotAppendPushedIndex(ctx context.Context, slotID, indexID int64) (err error) {
	l, _, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameSlot,
		"func":    "SlotAppendPushedIndex",
		"slotID":  slotID,
		"indexID": indexID,
	})
	defer span.End()
	q := fmt.Sprintf(
		`MATCH (s:Slot), (i:Index) WHERE id(s) = %d AND id(i) = %d CREATE (s)-[:HAS_PUSHED_INDEX {timestamp: timestamp()}]->(i)`,
		slotID, indexID,
	)
	if _, e := n.Execute(q); e != nil {
		return utils.ErrorHandler(l, span, e, "SlotAppendPushedIndex: creating HAS_PUSHED_INDEX edge")
	}
	utils.Debug(l, span, "SlotAppendPushedIndex: slot %d pushed index %d", slotID, indexID)
	return nil
}

// latestPushTimestampsFromQuery runs a (Slot)-[HAS_PUSHED]->(m:Manifest)
// aggregation query returning id(m) AS mid, max(hp.timestamp) AS ts rows and
// folds them into a map of manifest node ID -> latest push timestamp. fn is
// used to attribute errors to the public entry point in log/error messages.
func (n Neo) latestPushTimestampsFromQuery(l *logrus.Entry, span trace.Span, fn, q string) (map[int64]int64, error) {
	res, e := n.Query(q)
	if e != nil {
		return nil, utils.ErrorHandler(l, span, e, fn+": executing query")
	}
	ts := make(map[int64]int64, len(res))
	for _, r := range res {
		mid, e := r.GetInt64("mid")
		if e != nil {
			return nil, utils.ErrorHandler(l, span, e, fn+": decoding mid")
		}
		t, e := r.GetInt64("ts")
		if e != nil {
			return nil, utils.ErrorHandler(l, span, e, fn+": decoding ts")
		}
		ts[mid] = t
	}
	return ts, nil
}

// SlotLatestPushTimestamps returns, for the (repo, tag, profile) Slot, a map
// of Manifest node ID -> timestamp of its most recent HAS_PUSHED edge. An
// empty map (no error) means the Slot does not exist or never pushed.
//
// HAS_PUSHED is an append-only push log, so a manifest can carry several
// edges; the map value is the LATEST timestamp. Consumers use this to apply
// latest-push-wins semantics per platform when computing a Slot's index.
func (n Neo) SlotLatestPushTimestamps(ctx context.Context, repo, tag, profile string) (map[int64]int64, error) {
	l, _, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameSlot,
		"func":    "SlotLatestPushTimestamps",
		"repo":    repo,
		"tag":     tag,
		"profile": profile,
	})
	defer span.End()
	profile = normalizeSlotProfile(profile)
	if repo == "" || tag == "" || profile == "" {
		return nil, utils.NewError(l, span, "SlotLatestPushTimestamps: empty triple component repo=%q tag=%q profile=%q", repo, tag, profile)
	}
	q := fmt.Sprintf(
		`MATCH (s:Slot {repo: "%s", tag: "%s", profile: "%s"})-[hp:HAS_PUSHED]->(m:Manifest) RETURN id(m) AS mid, max(hp.timestamp) AS ts`,
		repo, tag, profile,
	)
	return n.latestPushTimestampsFromQuery(l, span, "SlotLatestPushTimestamps", q)
}

// RepoTagLatestPushTimestamps returns a map of Manifest node ID -> timestamp
// of its most recent HAS_PUSHED edge across EVERY Slot of the (repo, tag)
// pair. Used as the fallback view for profiles that never pushed to the tag
// themselves: their computed index follows the registry tag's push history
// (latest push wins per platform) rather than a stale rule-union snapshot.
func (n Neo) RepoTagLatestPushTimestamps(ctx context.Context, repo, tag string) (map[int64]int64, error) {
	l, _, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameSlot,
		"func":    "RepoTagLatestPushTimestamps",
		"repo":    repo,
		"tag":     tag,
	})
	defer span.End()
	if repo == "" || tag == "" {
		return nil, utils.NewError(l, span, "RepoTagLatestPushTimestamps: empty component repo=%q tag=%q", repo, tag)
	}
	q := fmt.Sprintf(
		`MATCH (s:Slot {repo: "%s", tag: "%s"})-[hp:HAS_PUSHED]->(m:Manifest) RETURN id(m) AS mid, max(hp.timestamp) AS ts`,
		repo, tag,
	)
	return n.latestPushTimestampsFromQuery(l, span, "RepoTagLatestPushTimestamps", q)
}

// SlotProfilesForCollectionTag returns, for the (repo, tag) collection, a map
// of Manifest node ID -> the sorted labels of every Slot profile that pushed
// that manifest to THIS collection. This is the collection-scoped attribution
// ("who pushed this manifest into this collection") — in contrast to the
// manifest's global UPLOADED_BY edge, which records who uploaded it FIRST
// anywhere in the registry. The distinction matters when a manifest is
// re-pushed into another collection by a different profile: the UI listing
// must show the pushing profile of THIS collection, not the original
// uploader.
func (n Neo) SlotProfilesForCollectionTag(ctx context.Context, colID, colTagID int64) (map[int64][]string, error) {
	l, _, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":  pkgNameSlot,
		"func":     "SlotProfilesForCollectionTag",
		"colID":    colID,
		"colTagID": colTagID,
	})
	defer span.End()
	if colID < 0 || colTagID < 0 {
		return nil, utils.NewError(l, span, "SlotProfilesForCollectionTag: invalid ids colID=%d colTagID=%d", colID, colTagID)
	}
	// Return one row per (manifest, profile) — Result only has Strings/Int64s,
	// so we can't use collect() here. Deduplication and sorting happen in Go.
	q := fmt.Sprintf(
		`MATCH (c:Repo)-[:HAS_TAG]->(t:Tag)<-[:OF_TAG]-(s:Slot)-[:HAS_PUSHED]->(m:Manifest)
		 WHERE id(c) = %d AND id(t) = %d
		 RETURN id(m) AS mid, s.profile AS profile`,
		colID, colTagID,
	)
	res, e := n.Query(q)
	if e != nil {
		return nil, utils.ErrorHandler(l, span, e, "SlotProfilesForCollectionTag: executing query")
	}
	// Aggregate (mid -> set of profile labels) then sort for stable output.
	seen := make(map[int64]map[string]struct{})
	for _, r := range res {
		mid, e := r.GetInt64("mid")
		if e != nil {
			return nil, utils.ErrorHandler(l, span, e, "SlotProfilesForCollectionTag: decoding mid")
		}
		profile, _ := r.GetStr("profile")
		if profile == "" {
			continue
		}
		if _, ok := seen[mid]; !ok {
			seen[mid] = make(map[string]struct{})
		}
		seen[mid][profile] = struct{}{}
	}
	out := make(map[int64][]string, len(seen))
	for mid, profileSet := range seen {
		labels := make([]string, 0, len(profileSet))
		for p := range profileSet {
			labels = append(labels, p)
		}
		sort.Strings(labels)
		out[mid] = labels
	}
	return out, nil
}

// ManifestGetAllForSlot walks the HAS_PUSHED edges from the :Slot with id
// slotID and returns every content-leaf Manifest (currently labelled
// :Manifest until Phase C) that was ever pushed to that Slot. The
// result is deduplicated by manifest ID; repushes of the same manifest
// are collapsed to a single entry in the return slice even though they
// produce multiple HAS_PUSHED edges.
//
// This is the Slot-first walker used by the compute path: it replaces
// the Upstream-based raw-md fetch from Path Y for any (repo, tag) that
// has an active :Slot node. Callers that need to handle the pre-Slot
// edge case (a repo/tag that was pushed before Path Z task 6 phase A
// landed and therefore has no Slot yet) should fall back to
// ManifestGetRawByRepoTag when this helper returns an empty slice.
//
// Unlike ManifestGetRawByRepoTag, this helper scopes to a specific
// Slot because the Slot already carries the (repo, tag, profile)
// dimension. To get the union across all profiles for a given
// (repo, tag), the caller must enumerate via SlotGetAllForRepoTag first
// and then union the results — which is what the compute path does.
//
// Only the bare manifest fields (ID, Digest, MediaType, Size, Label)
// are populated on each returned Manifest. The caller is responsible
// for hydrating Config, Platform, Layers, and Upstream via
// ManifestGetByDigest or equivalent if they need the full object.
//
// The returned slice is empty (not nil) if the Slot exists but has no
// HAS_PUSHED edges yet.
func (n Neo) ManifestGetAllForSlot(ctx context.Context, slotID int64) (manis []types.Manifest, err error) {
	l, _, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameSlot,
		"func":    "ManifestGetAllForSlot",
		"slotID":  slotID,
	})
	defer span.End()
	q := fmt.Sprintf(
		`MATCH (s:Slot)-[:HAS_PUSHED]->(md:Manifest) WHERE id(s) = %d RETURN DISTINCT %s ORDER BY md.digest`,
		slotID, manifestDataReturn,
	)
	utils.Debug(l, span, q)
	results, e := n.Query(q)
	if e != nil {
		return nil, utils.ErrorHandler(l, span, e, "ManifestGetAllForSlot: executing query")
	}
	manis = make([]types.Manifest, 0, len(results))
	seen := map[int64]struct{}{}
	for _, r := range results {
		if err := r.CheckKeys("manifestDataID", "manifestDataDigest", "manifestDataMediaType", "manifestDataSize"); err != nil {
			return nil, utils.ErrorHandler(l, span, err, "ManifestGetAllForSlot: missing keys in result")
		}
		mdID, _ := r.GetInt64("manifestDataID")
		if _, dup := seen[mdID]; dup {
			continue
		}
		digestStr, _ := r.GetStr("manifestDataDigest")
		mediaType, _ := r.GetStr("manifestDataMediaType")
		size, _ := r.GetInt64("manifestDataSize")
		label, _ := r.GetStr("manifestDataLabel")
		manis = append(manis, types.Manifest{
			ID:        mdID,
			Digest:    digestStr,
			MediaType: mediaType,
			Size:      size,
			Label:     label,
		})
		seen[mdID] = struct{}{}
	}
	return manis, nil
}
