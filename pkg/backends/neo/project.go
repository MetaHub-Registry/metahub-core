package neo

import (
	"context"

	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

// project.go implements the registry-project + storage-quota model
// (openspec change add-registry-project-quotas, task groups 1 & 2).
//
// A :Project owns the two-segment namespace "<team_slug>/<project_slug>". It is
// bound to content purely by the leading two segments of :Repo.repo — there are
// NO edges from :Project to content, so deleting a project can never touch a
// shared, content-addressed :Blob. Per-project usage is derived on-read from the
// graph (sum of distinct blob sizes reachable from the project's manifests);
// nothing is cached on the node in v1.

const pkgNameProject = "backends/neo/project.go"

// projectReturn is the shared RETURN projection so every read yields the same shape.
// visibility/kind/owner use coalesce so pre-existing :Project nodes (created before
// add-registry-namespace-authz) read back as private/project/"" (migration on read).
const projectReturn = ` id(p) AS id, p.team_slug AS team, p.project_slug AS project,` +
	` p.quota_bytes AS quota, p.created_at AS created,` +
	` coalesce(p.visibility, "private") AS visibility,` +
	` coalesce(p.kind, "project") AS kind, coalesce(p.owner, "") AS owner`

// recToProject extracts a types.Project from the current record of a result.
func recToProject(result neo4j.Result) (p types.Project) {
	rec := result.Record()
	if v, ok := rec.Get("id"); ok {
		p.ID, _ = v.(int64)
	}
	if v, ok := rec.Get("team"); ok {
		p.TeamSlug, _ = v.(string)
	}
	if v, ok := rec.Get("project"); ok {
		p.ProjectSlug, _ = v.(string)
	}
	if v, ok := rec.Get("quota"); ok {
		p.QuotaBytes, _ = v.(int64)
	}
	if v, ok := rec.Get("created"); ok {
		p.CreatedAt, _ = v.(int64)
	}
	if v, ok := rec.Get("visibility"); ok {
		p.Visibility, _ = v.(string)
	}
	if v, ok := rec.Get("kind"); ok {
		p.Kind, _ = v.(string)
	}
	if v, ok := rec.Get("owner"); ok {
		p.Owner, _ = v.(string)
	}
	return
}

// ProjectCreate MERGEs a project by (team, project). quota_bytes/created_at are
// set only on create; use ProjectUpdateQuota to change the quota afterwards.
func (n Neo) ProjectCreate(ctx context.Context, team, project string, quotaBytes int64) (p types.Project, err error) {
	_, span := tracer.Start(ctx, "ProjectCreate")
	defer span.End()
	q := `MERGE (p:Project {team_slug: $team, project_slug: $project})` +
		` ON CREATE SET p.quota_bytes = $quota, p.created_at = TIMESTAMP(),` +
		` p.visibility = "private", p.kind = "project", p.owner = ""` +
		` RETURN` + projectReturn
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	result, err := session.Run(q, map[string]interface{}{"team": team, "project": project, "quota": quotaBytes})
	if err != nil {
		logrus.Errorf("ProjectCreate failed: %v", err)
		return
	}
	if result.Next() {
		p = recToProject(result)
	}
	return
}

// ProjectGet returns a project by (team, project); found is false when absent.
func (n Neo) ProjectGet(ctx context.Context, team, project string) (p types.Project, found bool, err error) {
	_, span := tracer.Start(ctx, "ProjectGet")
	defer span.End()
	q := `MATCH (p:Project {team_slug: $team, project_slug: $project}) RETURN` + projectReturn
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close()
	result, err := session.Run(q, map[string]interface{}{"team": team, "project": project})
	if err != nil {
		logrus.Errorf("ProjectGet failed: %v", err)
		return
	}
	if result.Next() {
		found = true
		p = recToProject(result)
	}
	return
}

// ProjectUpdateQuota sets a project's quota_bytes.
func (n Neo) ProjectUpdateQuota(ctx context.Context, team, project string, quotaBytes int64) (err error) {
	_, span := tracer.Start(ctx, "ProjectUpdateQuota")
	defer span.End()
	q := `MATCH (p:Project {team_slug: $team, project_slug: $project}) SET p.quota_bytes = $quota`
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	_, err = session.Run(q, map[string]interface{}{"team": team, "project": project, "quota": quotaBytes})
	if err != nil {
		logrus.Errorf("ProjectUpdateQuota failed: %v", err)
	}
	return
}

// ProjectDelete removes ONLY the :Project node. Plain DELETE (never DETACH
// DELETE): the node has no blob edges, so shared content-addressed blobs are
// inherently untouched. found is false when no project matched.
func (n Neo) ProjectDelete(ctx context.Context, team, project string) (found bool, err error) {
	_, span := tracer.Start(ctx, "ProjectDelete")
	defer span.End()
	q := `MATCH (p:Project {team_slug: $team, project_slug: $project}) DELETE p RETURN count(*) AS deleted`
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	result, err := session.Run(q, map[string]interface{}{"team": team, "project": project})
	if err != nil {
		logrus.Errorf("ProjectDelete failed: %v", err)
		return
	}
	if result.Next() {
		if v, ok := result.Record().Get("deleted"); ok {
			d, _ := v.(int64)
			found = d > 0
		}
	}
	return
}

// ProjectList returns all projects ordered by (team, project).
func (n Neo) ProjectList(ctx context.Context) (ps []types.Project, err error) {
	_, span := tracer.Start(ctx, "ProjectList")
	defer span.End()
	q := `MATCH (p:Project) RETURN` + projectReturn + ` ORDER BY team, project`
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close()
	result, err := session.Run(q, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("ProjectList failed: %v", err)
		return
	}
	for result.Next() {
		ps = append(ps, recToProject(result))
	}
	return
}

// ProjectQuotaSum returns Σ(quota_bytes) across all projects (for the
// no-overcommit pool-capacity check in metahub-api).
func (n Neo) ProjectQuotaSum(ctx context.Context) (total int64, err error) {
	_, span := tracer.Start(ctx, "ProjectQuotaSum")
	defer span.End()
	q := `MATCH (p:Project) RETURN coalesce(sum(p.quota_bytes), 0) AS total`
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close()
	result, err := session.Run(q, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("ProjectQuotaSum failed: %v", err)
		return
	}
	if result.Next() {
		if v, ok := result.Record().Get("total"); ok {
			total, _ = v.(int64)
		}
	}
	return
}

// ProjectUsageBytes is the logical per-project usage: the sum of the sizes of the
// DISTINCT blobs reachable from manifests whose repo is under the project's
// "<team>/<project>" namespace. Deduped within the project (collect DISTINCT);
// blobs with unknown size (< 0) are excluded. A leading OPTIONAL MATCH ensures a
// row is always returned (0 for an empty/unknown project).
func (n Neo) ProjectUsageBytes(ctx context.Context, team, project string) (usage int64, err error) {
	_, span := tracer.Start(ctx, "ProjectUsageBytes")
	defer span.End()
	prefix := team + "/" + project
	q := `OPTIONAL MATCH (r:Repo)-[:HAS_TAG]->(:Tag)-[:REF]->(:Index)-[:REFERENCES]->(md:Manifest)` +
		` WHERE r.repo STARTS WITH $prefixSlash OR r.repo = $prefix` +
		` OPTIONAL MATCH (md)-[]->(b:Blob) WHERE b.type IN ["config", "layer"] AND b.size >= 0` +
		` WITH collect(DISTINCT b) AS blobs` +
		` RETURN reduce(s = 0, x IN blobs | s + x.size) AS usage`
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close()
	result, err := session.Run(q, map[string]interface{}{"prefix": prefix, "prefixSlash": prefix + "/"})
	if err != nil {
		logrus.Errorf("ProjectUsageBytes failed: %v", err)
		return
	}
	if result.Next() {
		if v, ok := result.Record().Get("usage"); ok {
			usage, _ = v.(int64)
		}
	}
	return
}

// UnknownUsageBucket is the reserved per-user usage key for bytes that cannot be
// attributed to a real pusher — compute-generated artefacts (repackage/injection
// Jobs push as a system identity) or legacy HAS_PUSHED edges written before
// per-user stamping. Consumers that report per-user usage to an external system
// (e.g. the Waldur site-agent) drop this bucket; it exists so the split still
// reconciles with the project total.
const UnknownUsageBucket = "unknown"

// ProjectUserUsageBytes attributes the project's logical usage to individual
// pushers. It takes the SAME billable blob set as ProjectUsageBytes (the DISTINCT
// config/layer blobs reachable from the project's manifests) and, for each blob,
// splits its size equally among the DISTINCT users who pushed a manifest
// referencing it within the namespace — the `user` stamped on the append-only
// HAS_PUSHED edge. A deduped re-push still appends a HAS_PUSHED edge, so a second
// pusher of an already-stored blob correctly dilutes the split (Alice+Bob → ½
// each). Blobs with no attributable pusher fall into UnknownUsageBucket. By
// construction Σ(all buckets) == ProjectUsageBytes (modulo float rounding), so the
// per-user split reconciles exactly with the project total.
func (n Neo) ProjectUserUsageBytes(ctx context.Context, team, project string) (usage map[string]float64, err error) {
	_, span := tracer.Start(ctx, "ProjectUserUsageBytes")
	defer span.End()
	usage = map[string]float64{}
	prefix := team + "/" + project
	// Billable set = ProjectUsageBytes's reachable config/layer blobs; then per
	// blob collect the DISTINCT non-empty pushers (scoped to this namespace's
	// Slots) off the HAS_PUSHED edge. Split is done in Go for testability.
	q := `OPTIONAL MATCH (r:Repo)-[:HAS_TAG]->(:Tag)-[:REF]->(:Index)-[:REFERENCES]->(md:Manifest)` +
		` WHERE r.repo STARTS WITH $prefixSlash OR r.repo = $prefix` +
		` OPTIONAL MATCH (md)-[]->(b:Blob) WHERE b.type IN ["config", "layer"] AND b.size >= 0` +
		` WITH collect(DISTINCT b) AS blobs` +
		` UNWIND blobs AS b` +
		` OPTIONAL MATCH (s:Slot)-[hp:HAS_PUSHED]->(:Manifest)-[]->(b)` +
		`   WHERE (s.repo STARTS WITH $prefixSlash OR s.repo = $prefix) AND hp.user IS NOT NULL AND hp.user <> ""` +
		` RETURN b.size AS size, collect(DISTINCT hp.user) AS users`
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close()
	result, err := session.Run(q, map[string]interface{}{"prefix": prefix, "prefixSlash": prefix + "/"})
	if err != nil {
		logrus.Errorf("ProjectUserUsageBytes failed: %v", err)
		return
	}
	for result.Next() {
		rec := result.Record()
		sv, _ := rec.Get("size")
		size, _ := sv.(int64)
		if size <= 0 {
			continue
		}
		uv, _ := rec.Get("users")
		// The Cypher already excludes null/empty pushers, so toStringSlice yields
		// exactly the distinct real usernames for this blob.
		users := toStringSlice(uv)
		if len(users) == 0 {
			usage[UnknownUsageBucket] += float64(size)
			continue
		}
		share := float64(size) / float64(len(users))
		for _, u := range users {
			usage[u] += share
		}
	}
	return
}

// ProjectBlobDigests returns the DISTINCT digests of the config/layer blobs
// reachable from the project's manifests — the set used to dedup an incoming
// push against what the project already references (the quota guard at push).
func (n Neo) ProjectBlobDigests(ctx context.Context, team, project string) (digests []string, err error) {
	_, span := tracer.Start(ctx, "ProjectBlobDigests")
	defer span.End()
	prefix := team + "/" + project
	q := `OPTIONAL MATCH (r:Repo)-[:HAS_TAG]->(:Tag)-[:REF]->(:Index)-[:REFERENCES]->(:Manifest)-[]->(b:Blob)` +
		` WHERE (r.repo STARTS WITH $prefixSlash OR r.repo = $prefix) AND b.type IN ["config", "layer"]` +
		` RETURN collect(DISTINCT b.digest) AS digests`
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close()
	result, err := session.Run(q, map[string]interface{}{"prefix": prefix, "prefixSlash": prefix + "/"})
	if err != nil {
		logrus.Errorf("ProjectBlobDigests failed: %v", err)
		return
	}
	if result.Next() {
		if v, ok := result.Record().Get("digests"); ok {
			digests = toStringSlice(v)
		}
	}
	return
}

// toStringSlice converts a neo4j list value (of strings) into a []string,
// skipping any non-string elements.
func toStringSlice(v interface{}) []string {
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, d := range arr {
		if s, ok := d.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// BlobRefCount returns the number of DISTINCT manifests referencing a blob by
// digest (config or layer). Zero means the blob is unreferenced (a future GC
// candidate).
func (n Neo) BlobRefCount(ctx context.Context, digest string) (refs int64, err error) {
	_, span := tracer.Start(ctx, "BlobRefCount")
	defer span.End()
	q := `MATCH (m:Manifest)-[]->(b:Blob {digest: $digest}) RETURN count(DISTINCT m) AS refs`
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close()
	result, err := session.Run(q, map[string]interface{}{"digest": digest})
	if err != nil {
		logrus.Errorf("BlobRefCount failed: %v", err)
		return
	}
	if result.Next() {
		if v, ok := result.Record().Get("refs"); ok {
			refs, _ = v.(int64)
		}
	}
	return
}
