package neo

import (
	"context"
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

func (n Neo) GetManifestsForManifestList(mlID int64) (manifests []types.Manifest, err error) {
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	q := `MATCH (p:Platform)-[:USES]-(f:Filter)<-[:IMPLEMENTS]-(ml:ManifestList)-[:SERVES]->(c:Collection)-[:UPSTREAM]->(m:Manifest)-[:UPSTREAM]->(u:Upstream) WHERE id(ml) = %d`
	q += ` MATCH (cfg:Config)<-[:USES]-(m)-[:IS]-(p)`
	q += ` RETURN id(p) AS platformID, p.arch AS platformArch, p.os AS platformOS, p.variant AS platformVariant, p.features AS platformFeatures,`
	q += ` id(m) AS manifestID, m.digest AS manifestDigest, m.mediaType AS manifestMediaType, m.size AS manifestSize,`
	q += ` id(u) AS upstreamID, u.reg AS upstreamReg, u.repo AS upstreamRepo, u.tag AS upstreamTag,`
	q += ` id(cfg) AS configID, cfg.digest AS configDigest, cfg.mediaType AS configMediaType, cfg.size AS configSize`
	input := map[string]interface{}{}
	query := fmt.Sprintf(q, mlID)
	result, err := session.Run(query, input)
	if err != nil {
		return
	}
	manifests = make([]types.Manifest, 0)
	for result.Next() {
		p, e := PlatformFromRecord(result)
		if e != nil {
			err = errors.Wrap(e, "failed to get platform from record")
			return
		}
		c, e := ConfigFromRecord(result)
		if e != nil {
			err = errors.Wrap(e, "failed to get config from record")
			return
		}
		u, e := UpstreamFromRecord(result)
		if e != nil {
			err = errors.Wrap(e, "failed to get upstream from record")
			return
		}
		m, e := ManifestFromRecord(result)
		if e != nil {
			err = errors.Wrap(e, "failed to get manifest from record")
			return
		}
		m.Config = c
		m.Platform = p
		m.Upstream = u
		manifests = append(manifests, m)
	}
	return
}

func (n Neo) GetAllManifests() (manifests []types.Manifest, err error) {
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	q := `MATCH (p:Platform)-[]-(m:Manifest)-[]-(u:Upstream)`
	q += ` MATCH (m)-[]-(c:Config)`
	q += ` RETURN id(p) AS platformID, p.arch AS platformArch, p.os AS platformOS, p.variant AS platformVariant, p.features AS platformFeatures,`
	q += ` id(m) AS manifestID, m.digest AS manifestDigest, m.mediaType AS manifestMediaType, m.size AS manifestSize,`
	q += ` id(u) AS upstreamID, u.reg AS upstreamReg, u.repo AS upstreamRepo, u.tag AS upstreamTag,`
	q += ` id(c) AS configID, c.digest AS configDigest, c.mediaType AS configMediaType, c.size AS configSize`
	input := map[string]interface{}{}
	result, err := session.Run(q, input)
	if err != nil {
		return
	}
	manifests = make([]types.Manifest, 0)
	for result.Next() {
		p, e := PlatformFromRecord(result)
		if e != nil {
			err = e
			return
		}
		c, e := ConfigFromRecord(result)
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
		m.Config = c
		m.Platform = p
		m.Upstream = u
		manifests = append(manifests, m)
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
	m.NeoID = v.(int64)
	v, ok = r.Get("manifestDigest")
	if !ok {
		err = fmt.Errorf("no manifestDigest found in record: %v", r)
		return
	}
	m.Digest = v.(string)
	v, ok = r.Get("manifestMediaType")
	if !ok {
		err = fmt.Errorf("no manifestMediaType found in record: %v", r)
		return
	}
	m.MediaType = v.(string)
	v, ok = r.Get("manifestSize")
	if !ok {
		err = fmt.Errorf("no manifestSize found in record: %v", r)
		return
	}
	m.Size = v.(int64)
	return
}

func ConfigFromRecord(res neo4j.Result) (c types.Config, err error) {
	r := res.Record()
	v, ok := r.Get("configID")
	if !ok {
		err = fmt.Errorf("no configID found in record: %v", r)
		return
	}
	c.NeoID = v.(int64)
	v, ok = r.Get("configDigest")
	if !ok {
		err = fmt.Errorf("no configDigest found in record: %v", r)
		return
	}
	c.Digest = v.(string)
	v, ok = r.Get("configMediaType")
	if !ok {
		err = fmt.Errorf("no configMediaType found in record: %v", r)
		return
	}
	c.MediaType = v.(string)
	v, ok = r.Get("configSize")
	if !ok {
		err = fmt.Errorf("no configSize found in record: %v", r)
		return
	}
	c.Size = v.(int64)
	return
}

func PlatformFromRecord(res neo4j.Result) (p types.Platform, err error) {
	r := res.Record()
	v, ok := r.Get("platformID")
	if !ok {
		err = fmt.Errorf("no configID found in record: %v", r)
		return
	}
	p.NeoID = v.(int64)
	v, ok = r.Get("platformArch")
	if !ok {
		err = fmt.Errorf("no platformArch found in record: %v", r)
		return
	}
	p.Arch = v.(string)
	v, ok = r.Get("platformOS")
	if !ok {
		err = fmt.Errorf("no platformOS found in record: %v", r)
		return
	}
	p.OS = v.(string)
	v, ok = r.Get("platformVariant")
	if !ok {
		err = fmt.Errorf("no platformVariant found in record: %v", r)
		return
	}
	p.Variant = v.(string)
	v, ok = r.Get("platformFeatures")
	if !ok {
		err = fmt.Errorf("no platformFeatures found in record: %v", r)
		return
	}
	p.Features = v.(string)
	return
}

func UpstreamFromRecord(res neo4j.Result) (u types.Upstream, err error) {
	r := res.Record()
	v, ok := r.Get("platformID")
	if !ok {
		err = fmt.Errorf("no configID found in record: %v", r)
		return
	}
	u.NeoID = v.(int64)
	v, ok = r.Get("upstreamReg")
	if !ok {
		err = fmt.Errorf("no upstreamReg found in record: %v", r)
		return
	}
	u.Reg = v.(string)
	v, ok = r.Get("upstreamRepo")
	if !ok {
		err = fmt.Errorf("no upstreamRepo found in record: %v", r)
		return
	}
	u.Repo = v.(string)
	v, ok = r.Get("upstreamTag")
	if !ok {
		err = fmt.Errorf("no upstreamTag found in record: %v", r)
		return
	}
	u.Tag = v.(string)
	return
}

func (n Neo) GetManifest(mani types.Manifest) (maniID int64, success bool, err error) {
	q := `MATCH (m:Manifest) WHERE m.digest = "%s" AND m.mediaType = "%s" AND m.size = %d RETURN id(m) AS maniID`
	query := fmt.Sprintf(q, mani.Digest, mani.MediaType, mani.Size)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	result, err := session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("Failed to run query: %s", err)
		return
	}
	if result.Next() {
		maniID = result.Record().GetByIndex(0).(int64)
		success = true
	}
	return
}

func (n Neo) CreateManifest(mani types.Manifest) (maniID int64, err error) {
	q := `CREATE (m:Manifest {digest: "%s", mediaType: "%s", size: %d}) RETURN id(m) AS manifestID`
	query := fmt.Sprintf(q, mani.Digest, mani.MediaType, mani.Size)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	result, err := session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("failed to create manifest: %v", err)
		return
	}
	if result.Next() {
		v, ok := result.Record().Get("manifestID")
		if !ok {
			err = fmt.Errorf("failed to get manifestID")
			return
		}
		maniID = v.(int64)
	}
	return
}

func (n Neo) LinkManifestToUpstream(ctx context.Context, maniID, upID int64) (err error) {
	q := `MATCH (m:Manifest),(u:Upstream) WHERE id(m) = %d AND id(u) = %d`
	q += ` MERGE (m)-[:UPSTREAM]->(u)`
	query := fmt.Sprintf(q, maniID, upID)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	_, err = session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("failed to create link (m:Manifest)-[:UPSTREAM]->(u:Upstream): %v", err)
		return
	}
	return
}

func (n Neo) LinkManifestToConfig(ctx context.Context, maniID, cfgID int64) (err error) {
	q := `MATCH (m:Manifest),(c:Config) WHERE id(m) = %d AND id(c) = %d`
	q += ` MERGE (m)-[:USES]->(c)`
	query := fmt.Sprintf(q, maniID, cfgID)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	_, err = session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("failed to create link (m:Manifest)-[:USES]->(c:Config): %v", err)
		return
	}
	return
}

func (n Neo) LinkManifestToPlatform(ctx context.Context, maniID, platID int64) (err error) {
	q := `MATCH (m:Manifest),(p:Platform) WHERE id(m) = %d AND id(p) = %d`
	q += ` MERGE (m)-[:IS]->(p)`
	query := fmt.Sprintf(q, maniID, platID)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	_, err = session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("failed to create link (m:Manifest)-[:IS]->(p:Platform): %v", err)
		return
	}
	return
}

func (n Neo) LinkManifestToLayer(ctx context.Context, maniID, layerID int64, idx int) (err error) {
	q := `MATCH (m:Manifest),(l:Layer) WHERE id(m) = %d AND id(l) = %d`
	q += ` MERGE (m)-[:HAS {order: %d}]->(l)`
	query := fmt.Sprintf(q, maniID, layerID, idx)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	_, err = session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("failed to create link (m:Manifest)-[:HAS {order: %d}]->(l:Layer): %v", idx, err)
		return
	}
	return
}
