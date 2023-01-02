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

func (n Neo) LinkManifestListToManifests(mlID int64, manifests []types.Manifest) (err error) {
	for _, m := range manifests {
		q := `MATCH (ml:ManifestList),(m:Manifest) WHERE id(ml) = %d AND id(m) = %d`
		q += ` MERGE (ml)-[:INCLUDES]->(m)`
		query := fmt.Sprintf(q, mlID, m.ID)
		session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
		defer session.Close()
		_, err = session.Run(query, map[string]interface{}{})
		if err != nil {
			logrus.Errorf("failed to create link (ml:ManifestList)-[:INCLUDES]->(m:Manifest): %v\n%s", err, query)
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

func (n Neo) ComputeManifestlists() (err error) {
	colFilPairs, err := n.GetCollectionFilterPairs()
	if err != nil {
		return
	}
	return n.computeManifestlists(colFilPairs)
}

func (n Neo) ComputeManifestlists_ForCollection(ctx context.Context, col types.Collection) (err error) {
	colFilPairs, err := n.GetCollectionFilterPairs_ForCollection(col)
	if err != nil {
		return
	}
	return n.computeManifestlists(colFilPairs)
}

func (n Neo) computeManifestlists(colFilPairs []types.CollectionFilterPair) (err error) {
	for _, colFilPair := range colFilPairs {
		col := colFilPair.Collection
		colTagID := col.ColTag.ID
		filter := colFilPair.Filter
		logrus.Warnf("Collection:%s Tag:%s - %s (%d manifests)", col.Repo, col.Tag, filter.Label, len(colFilPair.Manifests))
		ml := types.ManifestList{
			SchemaVersion: 2,
			MediaType:     "application/vnd.docker.distribution.manifest.list.v2+json",
			Manifests:     []types.Manifest{},
		}

		for _, mani := range colFilPair.Manifests {
			logrus.Warnf("  Manifest %s:%s", mani.Upstream.Repo, mani.Upstream.Tag)
			ml.Manifests = append(ml.Manifests, mani)
		}
		mlBlob := ml.ToOCIBytes()
		mlDigest := digest.FromBytes(mlBlob)
		//label := fmt.Sprintf("%s:%s-%s", colFilPair.Collection.Repo, colFilPair.Collection.Tag, colFilPair.Filter.Label)
		mlID, err := n.InsertManifestList(mlDigest, ml.MediaType, len(mlBlob))
		if err != nil {
			logrus.Error(err)
			continue
		}
		// Link ManifestList to Collection
		err = n.LinkManifestListToCollection(mlID, col.ID)
		if err != nil {
			logrus.Error(err)
			continue
		}
		// Link ManifestList to CollectionTag
		err = n.LinkManifestListToCollectionTag(mlID, colTagID)
		if err != nil {
			logrus.Error(err)
			continue
		}
		// Link ManifestList to Filter
		err = n.LinkManifestListToFilter(mlID, filter.ID)
		if err != nil {
			logrus.Error(err)
			continue
		}
		// Link ManifestList to Manifests
		err = n.LinkManifestListToManifests(mlID, colFilPair.Manifests)
		if err != nil {
			logrus.Error(err)
			continue
		}
	}
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
	q := `MATCH (prf:Profile)-[:USES]->(f:Filter)<-[:IMPLEMENTS]-(ml:ManifestList) WHERE prf.name = "%s"`
	q += ` MATCH (c:Collection)-[:REF]->(ml)<-[:REF]-(t:Tag) WHERE c.repo = "%s" AND t.name = "%s"`
	q += ` RETURN id(ml) as manifestlistID, ml.digest as manifestlistDigest, ml.mediaType as manifestlistMediaType, ml.size as manifestlistSize,`
	q += ` id(f) as filterID, f.label as filterLabel, f.feature AS filterFeature, id(t) as tagID`
	query := fmt.Sprintf(q, req.Profile, req.Repo, req.Tag)
	//fmt.Printf("Query: ManifestList_Get_ByRequest\n%s\n", query)
	res, found, err := n.QueryOne(query)
	if err != nil || !found {
		return
	}
	tagID, err := res.GetInt64("tagID")
	if err != nil {
		return
	}
	_ = tagID
	ml, err = ManifestListFromResult(res)
	if err != nil {
		return
	}
	f, err := FilterFromResult(res)
	if err != nil {
		return
	}

	q = `MATCH (f:Filter)-[:USES]->(p:Platform)<-[:IS]-(m:Manifest)<-[:INCLUDES]-(ml:ManifestList)`
	q += ` WHERE id(ml) = %d AND id(f) = %d`
	q += ` RETURN id(m) as manifestID, m.digest as manifestDigest, m.mediaType as manifestMediaType, m.size as manifestSize,`
	q += ` id(p) as platformID, p.os as platformOS, p.arch as platformArch, p.variant as platformVariant, p.features as platformFeatures`
	query = fmt.Sprintf(q, ml.ID, f.ID)
	//fmt.Printf("Query: Add Manifests\n%s\n", query)
	rs, err := n.Query(query)
	for _, r := range rs {
		m, err := ManifestFromResult(r)
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

func ManifestListFromResult(res Result) (ml types.ManifestList, err error) {
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
