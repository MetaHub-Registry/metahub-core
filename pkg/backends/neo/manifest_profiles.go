package neo

import (
	"context"
	"fmt"
	"strings"

	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

const pkgNameManifestProfiles = "neo/manifest_profiles.go"

// ManifestGetProfileLabels resolves which profile(s) serve each manifest in
// the given digest list. Returns a map of digest → []profileLabel, using the
// Slot→Index→Manifest graph path. Uses the raw neo4j driver because
// collect() returns a list that the Result.Add helper does not handle.
func (n Neo) ManifestGetProfileLabels(ctx context.Context, digests []string) (map[string][]string, error) {
	l, _, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManifestProfiles,
		"func":    "ManifestGetProfileLabels",
	})
	defer span.End()

	result := make(map[string][]string)
	if len(digests) == 0 {
		return result, nil
	}

	// Build quoted digest list for the IN clause.
	quoted := make([]string, len(digests))
	for i, d := range digests {
		quoted[i] = fmt.Sprintf(`"%s"`, d)
	}
	digestList := strings.Join(quoted, ", ")

	q := fmt.Sprintf(
		`MATCH (m:Manifest)<-[:REFERENCES]-(i:Index)<-[:POINTS_TO]-(s:Slot)-[:OF_PROFILE]->(p:Profile) `+
			`WHERE m.digest IN [%s] `+
			`RETURN m.digest AS digest, collect(DISTINCT p.label) AS profiles`,
		digestList,
	)
	utils.Debug(l, span, q)

	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close()

	neoResult, err := session.Run(q, nil)
	if err != nil {
		utils.ErrorHandler(l, span, err, "manifestGetProfileLabelsRaw query failed")
		return result, err
	}
	for neoResult.Next() {
		record := neoResult.Record()
		digVal, ok := record.Get("digest")
		if !ok {
			continue
		}
		dig, ok := digVal.(string)
		if !ok {
			continue
		}
		prfVal, ok := record.Get("profiles")
		if !ok {
			continue
		}
		prfList, ok := prfVal.([]interface{})
		if !ok {
			continue
		}
		labels := make([]string, 0, len(prfList))
		for _, p := range prfList {
			if s, ok := p.(string); ok {
				labels = append(labels, s)
			}
		}
		result[dig] = labels
	}
	return result, nil
}
