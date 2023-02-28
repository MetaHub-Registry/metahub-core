package neo

import (
	"context"
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func (n Neo) GetManifestList(dig digest.Digest, mt string, size int) (mlID int64, success bool, err error) {
	q := `MATCH (n:ManifestList) WHERE n.digest = "%s" AND n.mediaType = "%s" AND n.size = %d RETURN id(n) AS ID`
	query := fmt.Sprintf(q, dig, mt, size)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	result, err := session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("Failed to run query: %s", err)
		return
	}
	if result.Next() {
		v, ok := result.Record().Get("ID")
		if !ok {
			err = fmt.Errorf("GetManifestList: failed to get ID")
			return
		}
		mlID = v.(int64)
		success = true
	}
	return
}

func (n Neo) CreateManifestList(dig digest.Digest, mt string, size int) (mlID int64, err error) {
	q := `CREATE (n:ManifestList {digest: "%s", mediaType: "%s", size: %d, label: "%s"}) RETURN id(n) AS ID`
	query := fmt.Sprintf(q, dig, mt, size, utils.ShortenDigest(dig.String()))
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	result, err := session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("failed to create conifg: %v", err)
		return
	}
	if result.Next() {
		v, ok := result.Record().Get("ID")
		if !ok {
			err = fmt.Errorf("CreateManifestList: failed to get ID")
			return
		}
		mlID = v.(int64)
	}
	return
}

func (n Neo) UnLinkManifestListToCollection(mlID, colID int64) (err error) {
	q := `MATCH (c:Collection)-[r:REF]->(ml:ManifestList) WHERE id(c) = %d AND id(ml) = %d`
	q += ` DELETE r`
	query := fmt.Sprintf(q, colID, mlID)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	_, err = session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("failed to create link (ml:ManifestList)-[:REF]->(c:Collection): %v", err)
		return
	}
	return
}

func (n Neo) LinkManifestListToCollection(mlID, colID int64) (err error) {
	q := `MATCH (c:Collection), (ml:ManifestList) WHERE id(c) = %d AND id(ml) = %d`
	q += ` MERGE (ml)<-[:REF]-(c)`
	query := fmt.Sprintf(q, colID, mlID)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	_, err = session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("failed to create link (ml:ManifestList)-[:REF]->(c:Collection): %v", err)
		return
	}
	return
}

func (n Neo) LinkManifestListToManifestData(mlID, mdID int64) (err error) {
	q := `MATCH (ml:ManifestList), (md:ManifestData) WHERE id(ml) = %d AND id(md) = %d`
	q += ` MERGE (ml)-[:INCLUDES]-(md)`
	query := fmt.Sprintf(q, mlID, mdID)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	_, err = session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("failed to create link (ml)-[:INCLUDES]-(md): %v", err)
		return
	}
	return
}

func (n Neo) UnLinkManifestListToCollectionTag(mlID, colTagID int64) (err error) {
	q := `MATCH (ct:Tag)-[r:REF]->(ml:ManifestList) WHERE id(ct) = %d AND id(ml) = %d`
	q += ` DELETE r`
	query := fmt.Sprintf(q, colTagID, mlID)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	_, err = session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("failed to create link (ml:ManifestList)-[:REF]->(t:Tag): %v", err)
		return
	}
	return
}

func (n Neo) LinkManifestListToCollectionTag(mlID, colTagID int64) (err error) {
	q := `MATCH (ct:Tag), (ml:ManifestList) WHERE id(ct) = %d AND id(ml) = %d`
	q += ` MERGE (ml)<-[:REF]-(ct)`
	query := fmt.Sprintf(q, colTagID, mlID)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	_, err = session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("failed to create link (ml:ManifestList)-[:REF]->(t:Tag): %v", err)
		return
	}
	return
}

func (n Neo) LinkManifestListToProfile(mlID, pID int64) (err error) {
	q := `MATCH (p:Profile), (ml:ManifestList) WHERE id(p) = %d AND id(ml) = %d`
	q += ` MERGE (ml)<-[:REF]-(p)`
	query := fmt.Sprintf(q, pID, mlID)
	_, err = n.Execute(query)
	return
}

func (n Neo) LinkManifestListToFilter(mlID, filterID int64) (err error) {
	q := `MATCH (ml:ManifestList),(f:Filter) WHERE id(ml) = %d AND id(f) = %d`
	q += ` MERGE (ml)-[:IMPLEMENTS]->(f)`
	query := fmt.Sprintf(q, mlID, filterID)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	_, err = session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("failed to create link (ml:ManifestList)-[:IMPLEMENTS]->(f:Filter): %v", err)
		return
	}
	return
}

func (n Neo) LinkManifestListToManifests(mlID int64, manifests []types.ManifestData) (err error) {
	for _, m := range manifests {
		q := `MATCH (ml:ManifestList),(m:ManifestData) WHERE id(ml) = %d AND id(m) = %d`
		q += ` MERGE (ml)-[:INCLUDES]->(m)`
		query := fmt.Sprintf(q, mlID, m.ID)
		session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
		defer session.Close()
		_, err = session.Run(query, map[string]interface{}{})
		if err != nil {
			logrus.Errorf("failed to create link (ml:ManifestList)-[:INCLUDES]->(m:ManifestData): %v\n%s", err, query)
			return
		}
	}
	return
}

func (n Neo) ManifestList_Insert(ctx context.Context, ml *types.ManifestList) (err error) {
	dig := digest.Digest(ml.Digest)
	mlID, success, err := n.GetManifestList(dig, ml.MediaType, 0)
	if err != nil {
		logrus.Errorf("failed to get manifest list: %v", err)
		return
	}
	if success {
		ml.ID = mlID
		return
	}
	mlID, err = n.CreateManifestList(dig, ml.MediaType, 0)
	if err != nil {
		logrus.Errorf("failed to create manifest list: %v", err)
		return
	}
	ml.ID = mlID
	return
}

func (n Neo) InsertManifestList(dig digest.Digest, mt string, size int) (mlID int64, err error) {
	mlID, success, err := n.GetManifestList(dig, mt, size)
	if err != nil {
		return
	}
	if !success {
		mlID, err = n.CreateManifestList(dig, mt, size)
		if err != nil {
			return
		}
	}
	return
}

func (n Neo) ManifestList_Get_ByRequest(req types.Request) (ml types.ManifestList, found bool, err error) {
	/* TODO: Need to implement ManifestList_Get_ByRequest */
	// TODO: Do we have a tag or just a digest?
	logrus.Infof("Query for ML of name '%s' and tag '%s' / dig:'%s'", req.Repo, req.Tag, utils.ShortenDigest(req.Digest))
	q := fmt.Sprintf(`MATCH (c:Collection)-[:REF]->(ml)<-[:REF]-(t:Tag) WHERE c.repo = "%s"`, req.Repo)
	if req.Tag != "" {
		q += fmt.Sprintf(` AND t.name ="%s"`, req.Tag)
	} else {
		q += fmt.Sprintf(` AND ml.digest ="%s"`, req.Digest)
	}
	q += fmt.Sprintf(` MATCH (ml)<-[:REF]-(prf:Profile) WHERE prf.label = "%s"`, req.Profile)
	q += ` RETURN id(ml) as manifestlistID, ml.digest as manifestlistDigest, ml.mediaType as manifestlistMediaType, ml.size as manifestlistSize`
	//query := fmt.Sprintf(q, req.Repo, req.Tag, req.Profile)
	res, found, err := n.QueryOne(q)
	if err != nil {
		logrus.Error(err)
		return
	}
	if !found {
		logrus.Errorf("ManifestList not found")
		fmt.Printf(" >>> \n%s\n", q)
		return
	}
	ml, err = ManifestListFromResult(res)
	if err != nil {
		logrus.Error(err)
		return
	}
	// TODO: Fetch all INCLUDES for ManifestList
	q = fmt.Sprintf(`MATCH (ml:ManifestList)-[:INCLUDES]->(md:ManifestData) WHERE id(ml) = %d`, ml.ID)
	q += ` MATCH (md)<-[]-(m:Manifest)-[:IS]->(p:Platform)`
	q += ` RETURN id(md) as manifestID, md.digest as manifestDigest, md.mediaType as manifestMediaType,`
	q += ` md.size as manifestSize,`
	q += ` id(p) as platformID, p.os as platformOS, p.arch as platformArch, p.variant as platformVariant`
	fmt.Printf("Query: \n%s\n", q)
	rs, err := n.Query(q)
	for _, r := range rs {
		m, err := n.ManifestDataFromResult(r)
		if err != nil {
			logrus.Error(err)
			continue
		}
		p, err := PlatformFromResult(r)
		if err != nil {
			logrus.Error(err)
			continue
		}
		m.Platform = p
		ml.Manifests = append(ml.Manifests, m)
	}
	return
}

func ManifestListFromResult(res types.Result) (ml types.ManifestList, err error) {
	err = res.CheckKeys("manifestlistID", "manifestlistDigest", "manifestlistMediaType", "manifestlistSize")
	if err != nil {
		return
	}
	id, _ := res.GetInt64("manifestlistID")
	size, _ := res.GetInt("manifestlistSize")
	manifestlistDigest, _ := res.GetStr("manifestlistDigest")
	manifestlistMediaType, _ := res.GetStr("manifestlistMediaType")
	ml = types.ManifestList{
		ID:            id,
		SchemaVersion: 2,
		Digest:        manifestlistDigest,
		MediaType:     manifestlistMediaType,
		Size:          size,
	}
	return
}
