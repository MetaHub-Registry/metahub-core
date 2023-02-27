package neo

import (
	"context"
	"fmt"

	"github.com/docker/distribution"
	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func (n Neo) Layer_PlainInsert(ctx context.Context, l types.Layer) (layerID int64, err error) {
	layerID, success, err := n.GetLayer(l)
	if err != nil {
		return
	}
	if !success {
		layerID, err = n.CreateLayer(l)
		if err != nil {
			return
		}
	}
	return
}

func (n Neo) Layer_Insert(ctx context.Context, l types.Layer, up types.Upstream) (layerID int64, err error) {
	return
}

func (n Neo) LayerGetByDigest(dig string) (l types.Layer, found bool, err error) {
	q := `MATCH (n:Layer) WHERE n.digest = "%s"`
	q += ` RETURN id(n) AS ID, n.digest AS digest, n.mediaType AS mediaType, n.size AS size`
	query := fmt.Sprintf(q, dig)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	result, err := session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("Failed to run query: %s", err)
		return
	}
	if result.Next() {
		found = true
		v, ok := result.Record().Get("ID")
		if !ok {
			err = fmt.Errorf("LayerGetByDigest: failed to get ID")
			return
		}
		l.ID = v.(int64)
		v, ok = result.Record().Get("size")
		if !ok {
			err = fmt.Errorf("LayerGetByDigest: failed to get size")
			return
		}
		l.Size = v.(int64)
		v, ok = result.Record().Get("digest")
		if !ok {
			err = fmt.Errorf("LayerGetByDigest: failed to get digest")
			return
		}
		l.Digest = v.(string)
		v, ok = result.Record().Get("mediaType")
		if !ok {
			err = fmt.Errorf("LayerGetByDigest: failed to get mediaType")
			return
		}
		l.MediaType = v.(string)
	}
	return
}

func (n Neo) LayerDeleteByDigest(dig string) (err error) {
	q := `MATCH (n:Layer) WHERE n.digest = "%s" DETACH DELETE n`
	query := fmt.Sprintf(q, dig)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	_, err = session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("Failed to run query: %s", err)
		return
	}

	return err
}

func (n Neo) GetLayer(layer types.Layer) (layerID int64, success bool, err error) {
	q := `MATCH (n:Layer) WHERE n.digest = "%s" AND n.mediaType = "%s" AND n.size = %d RETURN id(n) AS ID`
	query := fmt.Sprintf(q, layer.Digest, layer.MediaType, layer.Size)
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
			err = fmt.Errorf("GetLayer: failed to get ID")
			return
		}
		layerID = v.(int64)
		success = true
	}
	return
}

func (n Neo) GetLayersForManifest(m types.ManifestData) (layers []types.Layer, err error) {
	layers = []types.Layer{}
	q := `MATCH (l:Layer)<-[:HAS]-(m:ManifestData) WHERE m.digest = "%s"`
	q += ` RETURN id(l) AS ID, l.digest AS digest, l.mediaType AS mediaType, l.size AS size`
	query := fmt.Sprintf(q, m.Digest)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	result, err := session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("Failed to run query: %s", err)
		return
	}
	for result.Next() {
		layer := types.Layer{}
		v, ok := result.Record().Get("ID")
		if !ok {
			err = fmt.Errorf("GetLayersForManifest: failed to get ID")
			return
		}
		layer.NeoID = v.(int64)
		v, ok = result.Record().Get("digest")
		if !ok {
			err = fmt.Errorf("GetLayersForManifest: failed to get digest")
			return
		}
		layer.Digest = v.(string)
		v, ok = result.Record().Get("mediaType")
		if !ok {
			err = fmt.Errorf("GetLayersForManifest: failed to get mediaType")
			return
		}
		layer.MediaType = v.(string)
		v, ok = result.Record().Get("size")
		if !ok {
			err = fmt.Errorf("GetLayersForManifest: failed to get size")
			return
		}
		layer.Size = v.(int64)
		layers = append(layers, layer)
	}
	return
}

func (n Neo) CreateLayer(layer types.Layer) (layerID int64, err error) {
	label := utils.CreateLabelFromTypeAndDigest(layer.MediaType, layer.Digest)
	q := `CREATE (n:Layer {digest: "%s", mediaType: "%s", size: %d, label: "%s"}) RETURN id(n) AS ID`
	query := fmt.Sprintf(q, layer.Digest, layer.MediaType, layer.Size, label)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	result, err := session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("failed to create layer: %v", err)
		return
	}
	if result.Next() {
		v, ok := result.Record().Get("ID")
		if !ok {
			err = fmt.Errorf("CreateLayer: failed to get ID")
			return
		}
		layerID = v.(int64)
	}
	return
}

func (n Neo) Layer_Create_ByDescriptor(l distribution.Descriptor) (layerID int64, err error) {
	label := utils.CreateLabelFromTypeAndDigest(l.MediaType, l.Digest.String())
	q := `CREATE (n:Layer {digest: "%s", mediaType: "%s", size: %d, label: "%s"}) RETURN id(n) AS ID`
	query := fmt.Sprintf(q, l.Digest.String(), l.MediaType, l.Size, label)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	result, err := session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("failed to create layer: %v", err)
		return
	}
	if result.Next() {
		v, ok := result.Record().Get("ID")
		if !ok {
			err = fmt.Errorf("Layer_Create_ByDescriptor: failed to get ID")
			return
		}
		layerID = v.(int64)
	}
	return
}

func (n Neo) GetLayersForManifestID(maniID int64) (layers []types.Layer, err error) {
	layers = []types.Layer{}
	q := `MATCH (l:Layer)<-[r :HAS]-(m:ManifestData) WHERE id(m) = %d`
	q += ` RETURN id(l) AS layerID, l.digest AS layerDigest, l.mediaType AS layerMediaType, l.size AS layerSize`
	q += ` ORDER BY r.order`
	query := fmt.Sprintf(q, maniID)
	res, err := n.Query(query)
	if err != nil {
		logrus.Errorf("Failed to run query: %s", err)
		return
	}
	for _, r := range res {
		l, e := LayerFromResult(r)
		if e != nil {
			err = errors.Wrap(e, "failed to get layers for manifest id")
			return
		}
		layers = append(layers, l)
	}
	return
}

func LayerFromResult(res types.Result) (l types.Layer, err error) {
	err = res.CheckKeys("layerID", "layerDigest", "layerMediaType", "layerSize")
	if err != nil {
		return
	}
	id, _ := res.GetInt64("layerID")
	size, _ := res.GetInt64("layerSize")
	mt, _ := res.GetStr("layerMediaType")
	dig, _ := res.GetStr("layerDigest")
	l = types.Layer{
		ID:        id,
		Digest:    dig,
		MediaType: mt,
		Size:      size,
	}
	return
}

func (n Neo) Layer_GetByTypeAndDigest(ctx context.Context, mt, dig string) (l types.Layer, found bool, err error) {
	q := `MATCH (n:Layer) WHERE n.digest = "%s" AND n.mediaType = "%s"`
	q += ` RETURN id(n) AS layerID, n.digest AS layerDigest, n.mediaType AS layerMediaType, n.size AS layerSize`
	query := fmt.Sprintf(q, dig, mt)
	//fmt.Printf("query: \n%s\n", query)
	res, found, err := n.QueryOne(query)
	if err != nil {
		return
	}
	if !found {
		return
	}
	l, err = LayerFromResult(res)
	if err != nil {
		err = errors.Wrap(err, "failed to get layer by type and digest")
	}
	return
}

func (n Neo) Layer_DescriporGet(ctx context.Context, desc v1.Descriptor) (layerID int64, found bool, err error) {
	q := `MATCH (n:Layer) WHERE n.digest = "%s" AND n.mediaType = "%s" RETURN id(n) AS ID`
	query := fmt.Sprintf(q, desc.Digest.String(), desc.MediaType)
	res, found, err := n.QueryOne(query)
	if err != nil {
		return
	}
	if !found {
		return
	}
	layerID, err = res.GetInt64("ID")
	if err != nil {
		logrus.Errorf("failed to get layer id: %v", err)
	}
	return
}

func (n Neo) Layer_DescriporGetInsert(ctx context.Context, desc v1.Descriptor) (layerID int64, err error) {
	// check if layer exists
	layerID, found, err := n.Layer_DescriporGet(ctx, desc)
	if err != nil {
		return 0, err
	}
	if found {
		return layerID, nil
	}
	// Create Layer
	label := utils.CreateLabelFromTypeAndDigest(desc.MediaType, desc.Digest.String())
	q := `CREATE (n:Layer {digest: "%s", mediaType: "%s", size: %d, label: "%s"}) RETURN id(n) AS ID`
	query := fmt.Sprintf(q, desc.Digest.String(), desc.MediaType, desc.Size, label)
	res, err := n.Execute(query)
	if err != nil {
		return 0, err
	}
	layerID, err = res.GetInt64("ID")
	if err != nil {
		fmt.Printf("Query: \n%s\n", query)
		logrus.Fatal(err)
		logrus.Errorf("failed to get layer id: %v", err)
	}
	return
}
