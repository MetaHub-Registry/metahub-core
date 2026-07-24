package types

// Project is a registry tenant boundary owning the two-segment namespace
// "<team_slug>/<project_slug>" (mirrors a Waldur Customer -> Project). It carries
// a logical storage quota; per-project usage is derived from the graph (the sum
// of distinct blob sizes reachable from manifests under the namespace), not
// stored on the node. See openspec change add-registry-project-quotas.
type Project struct {
	ID          int64  `json:"id"`
	TeamSlug    string `json:"team_slug"`
	ProjectSlug string `json:"project_slug"`
	QuotaBytes  int64  `json:"quota_bytes"`
	CreatedAt   int64  `json:"created_at"`
}

// Namespace returns the two-segment repo prefix the project owns: "<team>/<project>".
func (p Project) Namespace() string {
	return p.TeamSlug + "/" + p.ProjectSlug
}
