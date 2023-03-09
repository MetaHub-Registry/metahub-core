package magazine

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func (mag Magazine) CreateOCIv1(ctx context.Context, mani types.Manifest, origMani types.ManifestData) (err error) {
	ctx, span := tracer.Start(ctx, "CreateOCIv1")
	defer span.End()
	// TODO: Check if OCIv1 is already present
	msID := mani.ID
	_, found, err := mag.Database.ManifestData_GetID_ByMediaType(ctx, msID, "application/vnd.oci.image.manifest.v1+json")
	if err != nil {
		return
	}
	if found {
		logrus.Debugf("OCIv1 manifest already present for Manifest.ID: %d", msID)
		return
	}
	// Empty Manifest
	m := v1.Manifest{
		Versioned: specs.Versioned{
			SchemaVersion: 2,
		},
		MediaType: v1.MediaTypeImageManifest,
	}
	/////// A. Generate `oci.image.config.v1`
	cfg, err := mag.GenerateOCIv1Config(ctx, mani, origMani)
	if err != nil {
		return
	}
	cfgDesc, err := mag.GenerateOCIv1ConfigDescriptor(ctx, cfg)
	if err != nil {
		return
	}

	m.Config = cfgDesc
	// Upload Config Blob to Minio
	err = mag.Minio.PutConfig(ctx, cfgDesc.Data, cfgDesc.Digest.String())
	if err != nil {
		return
	}
	// Put Config into Graph
	cfgID, _, err := mag.Database.OCIv1Config_GetInsertDescriptor(ctx, cfgDesc)
	if err != nil {
		return
	}
	// Add Labels to Config
	for k, v := range origMani.Config.Cfg.Config.Labels {
		logrus.Warnf("origMani.Config.Cfg.Config.Labels: %s=%s", k, v)
		a := types.NewAnnotation(k, v)
		err = mag.Database.Config_AddLabel(ctx, cfgID, a)
		if err != nil {
			return
		}
	}
	// Generate Layers: Create OCI layer media-types:  application/vnd.oci.image.layer.v1.tar[+gzip]
	layerMap, err := mag.Layers_CreateOCIv1LayerDescriptors(ctx, origMani.Layers)
	if err != nil {
		return
	}
	for _, l := range layerMap {
		m.Layers = append(m.Layers, l.Descriptor)
	}
	//TODO: Generate Manifest: `oci.image.manifest.v1`
	////// Add Annotations
	m.Annotations = mani.Annotations.ToMap()

	// TODO: Put Manifest into Graph and Minio
	maniBytes, err := json.Marshal(m)
	if err != nil {
		return
	}
	// Minio
	maniDigest := digest.FromBytes(maniBytes)
	err = mag.Minio.ManifestsPutObj(ctx, maniDigest.String(), maniBytes)
	if err != nil {
		return
	}
	// Graph
	maniDesc, err := ManifestV1ToDescriptor(m)
	if err != nil {
		return
	}
	maniID, _, err := mag.Database.ManifestData_V1_GetInsert(ctx, msID, maniDesc)
	if err != nil {
		return
	}
	// Link ManifestData to Annotations
	err = mag.Database.ManifestData_AddAnnotations(ctx, maniID, &mani.Annotations)
	if err != nil {
		return
	}
	////// Link ManifestData to Config
	err = mag.Database.ManifestData_LinkToOCIv1Config(ctx, maniID, cfgID)
	if err != nil {
		return
	}
	////// Link ManifestData to Layers
	for _, ld := range layerMap {
		err = mag.Database.ManifestData_LinkToOCIv1Layer(ctx, maniID, ld.LayerID)
		if err != nil {
			logrus.Error(err)
			return
		}
	}

	return
}

func (mag Magazine) GenerateOCIv1Config(ctx context.Context, maniSI types.Manifest, origMani types.ManifestData) (v1Cfg v1.Image, err error) {
	// TODO: Fetch config from Minio
	v1Cfg, found, err := mag.Minio.GetConfig(ctx, origMani.Config.Digest)
	if err != nil {
		return
	}
	if !found {
		err = fmt.Errorf("config with digest '%s' not found in object store", origMani.Config.Digest)
		return
	}
	es := utils.NewEnvStore()
	// Put Labels into Env
	for k, v := range v1Cfg.Config.Labels {
		es.Add(k, v)
	}
	// Add original labels from Config
	anno := origMani.Config.Labels
	for k, v := range anno.ToMap() {
		cur, ok := v1Cfg.Config.Labels[k]
		if ok {
			if cur != v {
				logrus.Warnf("Label '%s' already present in config: '%s'", k, cur)
			}
		} else {
			logrus.Debugf("origMani.Config.Labels: Add label '%s' with value '%s'", k, v)
			v1Cfg.Config.Labels[k] = v
			es.Add(k, v)
		}
	}

	// and Annotations from Manifest
	for k, v := range maniSI.Annotations.ToMap() {
		// Only add labels with reverse DNS notations
		if utils.IsReverseDNS(k) {
			cur, ok := v1Cfg.Config.Labels[k]
			if ok {
				if cur != v {
					logrus.Warnf("Label '%s' already present in config: '%s'", k, cur)
				}
			} else {
				logrus.Debugf("maniSI.Annotations: Add label '%s' with value '%s'", k, v)
				v1Cfg.Config.Labels[k] = v
				es.Add(k, v)

			}
		}
	}
	v1Cfg.Config.Env = es.ToSlice()
	/*now := time.Now()
	mhHist := v1.History{
		Created:    &now,
		CreatedBy:  "org.metahub-registry.v1",
		Comment:    "Merged annotations into config labels and add labels as ENV",
		EmptyLayer: true,
	}
	v1Cfg.History = append(v1Cfg.History, mhHist)
	*/
	// TODO: Manipulate Config
	// -> ENV, CMD, ENTRYPOINT, VOLUME, EXPOSE, USER, WORKDIR, HEALTHCHECK, SHELL
	return
}

func (mag Magazine) GenerateOCIv1ConfigDescriptor(ctx context.Context, v1Cfg v1.Image) (dscr v1.Descriptor, err error) {
	cfgBytes, err := json.Marshal(v1Cfg)
	if err != nil {
		return
	}
	dig := digest.FromBytes(cfgBytes)
	size := len(cfgBytes)
	// Get Labels from Config attached to ManifestData and Annotations from Manifest
	dscr = v1.Descriptor{
		MediaType: v1.MediaTypeImageConfig,
		Data:      cfgBytes,
		Size:      int64(size),
		Digest:    dig,
	}
	logrus.Warnf("Generated ConfigDescriptor: %s", utils.ShortenDigest(dig.String()))
	return
}

func ManifestV1ToDescriptor(m v1.Manifest) (dscr v1.Descriptor, err error) {
	b, err := json.Marshal(m)
	if err != nil {
		return
	}
	dscr = v1.Descriptor{
		MediaType:   m.MediaType,
		Size:        int64(len(b)),
		Digest:      digest.FromBytes(b),
		Annotations: m.Annotations,
	}
	return
}
