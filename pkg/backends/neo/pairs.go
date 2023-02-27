package neo

import (
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

func (n Neo) CollectionTagProfilePair_GetAll() (ctpps []types.CollectionTagProfilePair, err error) {
	q := `MATCH (prf:Profile)-[:USES]->(Rule)-[:TRUE]->(Annotation)<-[:HAS_ANNOTATION]-(:ManifestData)<-[:CREATED]-(m:Manifest)`
	q += ` MATCH (c:Collection)-[:UPSTREAM]->(m)<-[:UPSTREAM]-(t:Tag)`
	q += ` RETURN DISTINCT id(prf) AS profileID, prf.name AS profileName, prf.tag AS profileTag, prf.rule AS profileRule,`
	q += ` id(c) AS collectionID, c.repo AS collectionRepo,`
	q += ` id(t) AS tagID, t.name AS tagName`
	res, err := n.Query(q)
	if err != nil {
		logrus.Error(err)
		return
	}
	ctpps = []types.CollectionTagProfilePair{}
	for _, r := range res {
		p, err := n.ProfileFromResult(r)
		if err != nil {
			logrus.Error(err)
			continue
		}
		c, err := CollectionFromResult(r)
		if err != nil {
			logrus.Error(err)
			continue
		}
		t, err := CollectionTagFromResult(r)
		if err != nil {
			logrus.Error(err)
			continue
		}
		ctpp, err := types.NewCollectionTagProfilePair(c, t, p)
		if err != nil {
			logrus.Error(err)
			return ctpps, err
		}

		/// Get Manifests for this Collection/Tag
		mds, err := n.ManifestData_GetByCollectionTag(c.ID, t.ID)
		if err != nil {
			logrus.Error(err)
			return ctpps, err
		}
		for _, md := range mds {
			err = ctpp.AddManifest(md)
			if err != nil {
				logrus.Error(err)
				return ctpps, err
			}
		}
		ctpps = append(ctpps, ctpp)
	}
	return
}
