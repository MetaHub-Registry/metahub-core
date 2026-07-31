package types

// Project visibility + kind values. Visibility governs pull access for project
// namespaces (private = members only, public = any authenticated user); kind
// distinguishes an org/project namespace from a per-user personal namespace.
// See openspec change add-registry-namespace-authz.
const (
	VisibilityPrivate = "private"
	VisibilityPublic  = "public"

	KindProject  = "project"
	KindPersonal = "personal"
)

// Project is a registry tenant boundary owning the two-segment namespace
// "<team_slug>/<project_slug>" (mirrors a Waldur Customer -> Project). It carries
// a logical storage quota; per-project usage is derived from the graph (the sum
// of distinct blob sizes reachable from manifests under the namespace), not
// stored on the node. See openspec change add-registry-project-quotas.
//
// Visibility/Kind/Owner (add-registry-namespace-authz): Visibility is
// private|public (default private); Kind is project|personal; Owner is the
// username for a personal namespace (empty for an org/project). Pre-existing
// nodes without these read back as private/project/"".
type Project struct {
	ID          int64  `json:"id"`
	TeamSlug    string `json:"team_slug"`
	ProjectSlug string `json:"project_slug"`
	QuotaBytes  int64  `json:"quota_bytes"`
	CreatedAt   int64  `json:"created_at"`
	Visibility  string `json:"visibility"`
	Kind        string `json:"kind"`
	Owner       string `json:"owner,omitempty"`
}

// Namespace returns the two-segment repo prefix the project owns: "<team>/<project>".
func (p Project) Namespace() string {
	return p.TeamSlug + "/" + p.ProjectSlug
}
