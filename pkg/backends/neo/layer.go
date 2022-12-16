package neo

import (
	"context"
	"fmt"

	"github.com/docker/distribution"
	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
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

func (n Neo) GetLayersForManifest(m types.Manifest) (layers []types.Layer, err error) {
	layers = []types.Layer{}
	q := `MATCH (l:Layer)<-[:HAS]-(m:Manifest) WHERE m.digest = "%s"`
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
	q := `CREATE (n:Layer {digest: "%s", mediaType: "%s", size: %d, label: "%s"}) RETURN id(n) AS ID`
	query := fmt.Sprintf(q, layer.Digest, layer.MediaType, layer.Size, utils.ShortenDigest(layer.Digest))
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
	q := `CREATE (n:Layer {digest: "%s", mediaType: "%s", size: %d, label: "%s"}) RETURN id(n) AS ID`
	query := fmt.Sprintf(q, l.Digest.String(), l.MediaType, l.Size, utils.ShortenDigest(l.Digest.String()))
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
