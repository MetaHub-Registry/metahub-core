package neo

import (
	"context"
	"fmt"
	"strings"

	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

func (n Neo) Manifest_LinkToAnnotation(ctx context.Context, maniID, annotationID int64) (err error) {
	q := `MATCH (m:Manifest), (a:Annotation) WHERE id(m) = %d AND id(a) = %d MERGE (m)-[:HAS_ANNOTATION]->(a)`
	query := fmt.Sprintf(q, maniID, annotationID)
	_, err = n.Execute(query)
	if err != nil {
		return
	}
	return
}

func (n Neo) Manifest_GetInsert(ctx context.Context, m types.Manifest) (maniID int64, err error) {
	/*** TODO:
	1. Check if Manifest exists as a connection between
	   a. Upstream
	   b. Collection
	   (c. Annotation?)
	*/
	up := m.Upstream
	_, err = n.Upstream_GetInsert(&up)
	if err != nil {
		logrus.Error(err)
		return
	}
	// If Manifest between collection and upstream does not exists
	maniID, found, err := n.Manifest_Get(up.ID)
	if err != nil {
		logrus.Error(err)
		return
	}
	if !found {
		// Create ManifestStandin
		maniID, err = n.Manifest_Create(up.ID)
		if err != nil {
			logrus.Error(err)
			return
		}
	}
	for _, a := range m.Annotations.Data {
		aID, e := n.Annotation_GetInsert(ctx, a)
		if e != nil {
			err = errors.Wrap(e, fmt.Sprintf("failed to get/insert annotation: %s", a.Key))
			return
		}
		e = n.Manifest_LinkToAnnotation(ctx, maniID, aID)
		if e != nil {
			err = errors.Wrap(e, fmt.Sprintf("failed to link manifest to annotation: %s", a.Key))
			return
		}
		// Check if Annotation exists
	}
	/**** Do we need this still ****/
	/// OK! We got the ManifestStandin, Collection and Upstream
	// Check if Manifest exists
	//maniID, found, err = n.ManifestData_Get(msID, m.Digest)
	return
}

func (n Neo) Manifest_Create(upID int64) (msID int64, err error) {
	q := `MATCH (u:Upstream) WHERE id(u) = %d`
	q += ` MERGE (m:Manifest {label: u.reg+"/"+u.repo+":"+u.tag})-[:UPSTREAM]->(u)`
	q += ` RETURN id(m) AS ID`
	query := fmt.Sprintf(q, upID)
	//fmt.Printf("Query: \n%s\n", query)
	res, err := n.Execute(query)
	if err != nil {
		logrus.Error(err)
		return
	}
	msID, err = res.GetInt64("ID")
	return
}

func (n Neo) LinkManifestToPlatform(ctx context.Context, maniID, platID int64) (err error) {
	q := `MATCH (m:Manifest),(p:Platform) WHERE id(m) = %d AND id(p) = %d`
	q += ` MERGE (m)-[:IS]->(p)`
	query := fmt.Sprintf(q, maniID, platID)
	//fmt.Printf("query: \n%s\n", query)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	_, err = session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("failed to create link (m:Manifest)-[:IS]->(p:Platform): %v", err)
		return
	}
	return
}

func (n Neo) Manifest_AddAnnotation(ctx context.Context, maniID int64, a *types.Annotation) (err error) {
	aID, err := n.Annotation_GetInsert(ctx, *a)
	if err != nil {
		return
	}
	a.ID = aID
	err = n.Manifest_LinkToAnnotation(ctx, maniID, aID)
	return
}

func (n Neo) Manifest_AddAnnotations(ctx context.Context, maniID int64, as *types.Annotations) (err error) {
	for _, a := range as.Data {
		aID, e := n.Annotation_GetInsert(ctx, a)
		if e != nil {
			err = errors.Wrap(e, "failed to add annotation")
			return
		}
		a.ID = aID
		err = n.Manifest_LinkToAnnotation(ctx, maniID, aID)
	}
	return
}

func (n Neo) Manifest_Get(upID int64) (msID int64, found bool, err error) {
	q := `MATCH (m:Manifest)-[]->(u:Upstream)`
	q += ` WHERE id(u) = %d`
	q += ` RETURN id(m) AS ID`
	query := fmt.Sprintf(q, upID)
	//fmt.Printf("ManifestStandin_Get Query: \n%s\n", query)
	res, found, err := n.QueryOne(query)
	if err != nil {
		logrus.Error(err)
		return
	}
	if !found {
		// Not found, return
		return
	}
	msID, err = res.GetInt64("ID")
	// error will be nil if found
	return

}

func (n Neo) LinkManifestToManifestData(ctx context.Context, maniID, maniDataID int64, props ...string) (err error) {
	q := `MATCH (m:Manifest), (md:ManifestData) WHERE id(m) = %d AND id(md) = %d`
	switch {
	case len(props) > 0:
		q += fmt.Sprintf(` MERGE (m)-[:%s]->(md)`, strings.ToUpper(props[0]))
	case len(props) == 0:
		q += ` MERGE (m)-[:ORIGINAL]->(md)`
	default:
		return fmt.Errorf("expected one property for LinkManifestToManifestData, got %d", len(props))
	}
	query := fmt.Sprintf(q, maniID, maniDataID)
	_, err = n.Execute(query)
	return
}

func (n Neo) Manifest_GetAll() (manis []types.Manifest, err error) {
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	q := `MATCH (p:Platform)-[]-(m:Manifest)-[]-(u:Upstream)`
	q += ` RETURN id(p) AS platformID, p.arch AS platformArch, p.os AS platformOS, p.variant AS platformVariant, p.features AS platformFeatures,`
	q += ` id(m) AS manifestID,`
	q += ` id(u) AS upstreamID, u.reg AS upstreamReg, u.repo AS upstreamRepo, u.tag AS upstreamTag`
	//fmt.Printf("query: \n%s\n", q)
	input := map[string]interface{}{}
	result, err := session.Run(q, input)
	if err != nil {
		return
	}
	ctx := context.Background()
	manis = make([]types.Manifest, 0)
	for result.Next() {
		p, e := PlatformFromRecord(result)
		if e != nil {
			err = e
			return
		}
		u, e := UpstreamFromRecord(result)
		if e != nil {
			err = e
			return
		}
		m, e := ManifestFromRecord(result)
		if e != nil {
			err = e
			return
		}
		anno, e := n.GetAnnotationsByManifestID(ctx, m.ID)
		if e != nil {
			err = e
			return
		}
		m.Annotations = anno
		m.Platform = p
		m.Upstream = u
		mds, e := n.ManifestData_GetAllByManifestID(ctx, m.ID)
		if e != nil {
			err = e
			return
		}
		m.Manifests = mds
		// Add manifest to list
		manis = append(manis, m)
	}
	return
}

func (n Neo) Manifest_LinkToUpstream(ctx context.Context, maniID, upID int64) (err error) {
	q := `MATCH (m:Manifest),(u:Upstream) WHERE id(m) = %d AND id(u) = %d`
	q += ` MERGE (m)-[:UPSTREAM]->(u)`
	query := fmt.Sprintf(q, maniID, upID)
	//fmt.Printf("Query: \n%s\n", query)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	_, err = session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("failed to create link (m:Manifest)-[:UPSTREAM]->(u:Upstream): %v", err)
		return
	}
	return
}

func ManifestFromRecord(res neo4j.Result) (m types.Manifest, err error) {
	r := res.Record()
	v, ok := r.Get("manifestID")
	if !ok {
		err = fmt.Errorf("no manifestID found in record: %v", r)
		return
	}
	m.ID = v.(int64)
	return
}
