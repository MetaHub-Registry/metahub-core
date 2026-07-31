package types

import "regexp"

// namespace.go — shared namespace-slug rules for the registry-namespace-authz
// model. metahub-api imports these so the Authentik group names it constructs
// (mh-<org>[-<project>][-admin]) use exactly the same slug shape the agent
// (change D) and metalib assume. See openspec add-registry-namespace-authz.

// GroupPrefix is the Authentik group-name prefix for registry RBAC groups
// (metahub-admin is the separate global-admin group).
const GroupPrefix = "mh-"

// PersonalTeamSlug is the reserved top-level segment for personal namespaces
// ("user/<username>"). A Waldur org slug MUST NOT be "user".
const PersonalTeamSlug = "user"

// slugRe is the canonical registry slug format, matching Waldur customer/project
// slugs: lowercase alphanumerics with internal single hyphens, e.g. "qnib-solut".
var slugRe = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

// ValidSlug reports whether s is a well-formed namespace slug.
func ValidSlug(s string) bool { return slugRe.MatchString(s) }

// ValidNamespacePair reports whether (org, project) are both valid slugs and org
// is not the reserved "user" segment (which is owned by personal namespaces
// user/<username>). This is the guard both metahub-api and the site-agent apply
// so a project namespace can never collide with the personal-namespace prefix.
func ValidNamespacePair(org, project string) bool {
	return org != PersonalTeamSlug && ValidSlug(org) && ValidSlug(project)
}
