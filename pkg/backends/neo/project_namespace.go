package neo

import (
	"context"
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

// project_namespace.go adds the namespace-authz model helpers (openspec change
// add-registry-namespace-authz): personal namespaces (user/<username>) and
// project visibility mutation. Shares projectReturn/recToProject with project.go.
// The reserved "user" segment lives in types.PersonalTeamSlug.

// PersonalNamespaceGetOrCreate get-or-creates the personal namespace
// "user/<username>" as a :Project with kind=personal, owner=<username>,
// visibility=private and — only on first creation — quota_bytes=defaultQuotaBytes.
// Idempotent: a repeat call returns the existing node and never resets its quota.
func (n Neo) PersonalNamespaceGetOrCreate(ctx context.Context, username string, defaultQuotaBytes int64) (p types.Project, err error) {
	_, span := tracer.Start(ctx, "PersonalNamespaceGetOrCreate")
	defer span.End()
	q := `MERGE (p:Project {team_slug: $team, project_slug: $user})` +
		` ON CREATE SET p.quota_bytes = $quota, p.created_at = TIMESTAMP(),` +
		` p.visibility = "private", p.kind = "personal", p.owner = $user` +
		` RETURN` + projectReturn
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	result, err := session.Run(q, map[string]interface{}{
		"team": types.PersonalTeamSlug, "user": username, "quota": defaultQuotaBytes,
	})
	if err != nil {
		logrus.Errorf("PersonalNamespaceGetOrCreate failed: %v", err)
		return
	}
	if result.Next() {
		p = recToProject(result)
	}
	return
}

// ProjectSetVisibility sets a project's visibility to "private" or "public".
// found is false when no project matched; an invalid visibility value is rejected.
func (n Neo) ProjectSetVisibility(ctx context.Context, team, project, visibility string) (found bool, err error) {
	_, span := tracer.Start(ctx, "ProjectSetVisibility")
	defer span.End()
	if visibility != types.VisibilityPrivate && visibility != types.VisibilityPublic {
		return false, fmt.Errorf("invalid visibility %q (want private|public)", visibility)
	}
	q := `MATCH (p:Project {team_slug: $team, project_slug: $project})` +
		` SET p.visibility = $visibility RETURN count(*) AS updated`
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	result, err := session.Run(q, map[string]interface{}{
		"team": team, "project": project, "visibility": visibility,
	})
	if err != nil {
		logrus.Errorf("ProjectSetVisibility failed: %v", err)
		return
	}
	if result.Next() {
		if v, ok := result.Record().Get("updated"); ok {
			c, _ := v.(int64)
			found = c > 0
		}
	}
	return
}
