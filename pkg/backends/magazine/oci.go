package magazine

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func (mag Magazine) CreateOCIv1_UploadedBy(ctx context.Context, mani types.Manifest, origMani types.ManifestData) (err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "magazine/oci.go",
		"func":    "CreateOCIv1_UploadedBy",
		"mani.ID": mani.ID,
	})
	ctx, span := tracer.Start(ctx, "CreateOCIv1_UploadedBy")
	defer span.End()
	// Get UPLOADED Manifests
	prf, found, e := mag.Database.Profile_GetUploadedBy_ViaManifestDataID(ctx, origMani.ID)
	if e != nil {
		err = errors.Wrapf(e, "failed to get Profile for ManifestData.ID: %d", origMani.ID)
		return
	}
	if !found {
		return
	}
	// TODO: Check if OCIv1 is already present
	_, found, err = mag.Database.ManifestData_Get_ByProfileMediaType(ctx, mani.ID, prf.ID, v1.MediaTypeImageManifest)
	if err != nil {
		return
	}
	if found {
		msg := fmt.Sprintf("OCIv1 manifest already present for Manifest.ID: %d", mani.ID)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "debug"),
			attribute.String("log.message", msg),
		))
		l.Info(msg)
		return
	} else {
		msg := fmt.Sprintf("OCIv1 manifest NOT present for Manifest.ID: %d", mani.ID)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "debug"),
			attribute.String("log.message", msg),
		))
		l.Info(msg)
	}
	prf, found, e = mag.Database.Profile_GetUploadedBy_ViaManifestDataID(ctx, origMani.ID)
	if e != nil {
		err = errors.Wrapf(e, "failed to get Profile for ManifestData.ID: %d", origMani.ID)
		return
	}

	maniID, e := mag.OCI_CreateBaseManifest(ctx, mani, origMani, prf.ID)
	if e != nil {
		err = errors.Wrap(e, "failed to create base manifest")
		return
	}

	if found {
		msg := fmt.Sprintf("Profile '%s' did the upload", prf.Label)
		l.Debug(msg)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "debug"),
			attribute.String("log.message", msg),
		))
		e := mag.Database.LinkManifestDataToProfile(ctx, maniID, prf.ID)
		if e != nil {
			err = errors.Wrap(e, "failed to link profile and manifestdata")
			l.Error(err)
			return
		}
		// TODO: We need to unwind the rules so that annotations are added to the ManifestData
		e = mag.Database.Profile_UnwindRules(ctx, prf.ID, maniID)
		if e != nil {
			err = errors.Wrap(e, "failed to unwind Profile")
			l.Error(err)
			return
		}
	}
	return
}

func (mag Magazine) OCI_CreateBaseManifest(ctx context.Context, mani types.Manifest, origMani types.ManifestData, prfID int64) (maniID int64, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "magazine/oci.go",
		"func":    "OCI_CreateBaseManifest",
		"mani.ID": mani.ID,
	})
	ctx, span := tracer.Start(ctx, "OCI_CreateBaseManifest")
	defer span.End()
	// Empty Manifest
	m := v1.Manifest{
		Versioned: specs.Versioned{
			SchemaVersion: 2,
		},
		MediaType: v1.MediaTypeImageManifest,
	}
	rs, e := mag.Database.Profile_GetAnnoEqRules(ctx, prfID)
	if e != nil {
		err = errors.Wrapf(e, "failed to get anno rules for profile %d", prfID)
		return
	}
	// ///// A. Generate `oci.image.config.v1`
	cfg, err := mag.GenerateOCIv1Config(ctx, mani, origMani)
	if err != nil {
		return
	}
	for _, r := range rs {
		if r.Type == types.RuleTypeAnnotation && r.Op == "eq" {
			cfg.Config.Labels[r.Key] = r.Value
			k := utils.UpLabelKey(r.Key)
			cfg.Config.Env = append(cfg.Config.Env, fmt.Sprintf("%s=%s", k, r.Value))
		}
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
		l.Warnf("origMani.Config.Cfg.Config.Labels: %s=%s", k, v)
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
	// //// Add Annotations
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
	maniID, _, err = mag.Database.ManifestData_V1_GetInsert(ctx, mani.ID, maniDesc)
	if err != nil {
		return
	}
	// Link ManifestData to Annotations
	err = mag.Database.ManifestData_AddAnnotations(ctx, maniID, &mani.Annotations)
	if err != nil {
		return
	}
	// //// Link ManifestData to Config
	err = mag.Database.ManifestData_LinkToOCIv1Config(ctx, maniID, cfgID)
	if err != nil {
		return
	}
	// //// Link ManifestData to Layers
	for _, ld := range layerMap {
		err = mag.Database.ManifestData_LinkToOCIv1Layer(ctx, maniID, ld.LayerID)
		if err != nil {
			l.Error(err)
			return
		}
	}
	return
}

func (mag Magazine) CreateOCIv1_ParsedManifest(ctx context.Context, mani types.Manifest, origMani types.ManifestData) (err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "magazine/oci.go",
		"func":    "CreateOCIv1_ParsedManifest",
		"mani.ID": mani.ID,
	})
	l.Infof("Creating OCIv1 manifest for Manifest.ID")
	ctx, span := tracer.Start(ctx, "CreateOCIv1_ParsedManifest")
	defer span.End()
	// TODO: Is the manifest in question uploaded or parsed?
	// -> If not parsed - skip!
	if origMani.Type != "PARSED" {
		l.Warnf("Manifest.ID: %d is not parsed (%s), skipping", mani.ID, origMani.Type)
		return
	}
	// TODO: Get parsed manifests
	_, found, err := mag.Database.ManifestData_GetParsedID_ByMediaType(ctx, mani.ID, v1.MediaTypeImageManifest, "PARSED")
	if err != nil {
		return
	}
	if found {
		l.Debugf("OCIv1 manifest already present for Manifest.ID: %d", mani.ID)
		return
	}
	maniID, e := mag.OCI_CreateBaseManifest(ctx, mani, origMani, 0)
	if e != nil {
		err = errors.Wrap(e, "failed to create base manifest")
		return
	}
	_ = maniID
	// TODO: Check which profile was used to upload the config
	//    -> For now we only generate one OCI manifest(data) per manifest, this needs to be extended in case
	//       someone pused different manifest for different profiles
	/*
		err = mag.Database.EvalUploadProfile(ctx, mani, origMani)
		if err != nil {
			return
		}
	*/

	return
}

func (mag Magazine) CreateOCIv1(ctx context.Context, mani types.Manifest, origMani types.ManifestData) (err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "magazine/oci.go",
		"func":    "CreateOCIv1",
		"mani.ID": mani.ID,
	})
	l.Debugf("Creating OCIv1 manifest for Manifest.ID: %d", mani.ID)
	ctx, span := tracer.Start(ctx, "CreateOCIv1")
	defer span.End()
	err = mag.CreateOCIv1_UploadedBy(ctx, mani, origMani)
	if err != nil {
		err = errors.Wrapf(err, "failed to create OCIv1 manifest for uploaded Manifest.ID: %d", mani.ID)
		return
	}
	err = mag.CreateOCIv1_ParsedManifest(ctx, mani, origMani)
	if err != nil {
		err = errors.Wrapf(err, "failed to create OCIv1 manifest for parsed Manifest.ID: %d", mani.ID)
		return
	}
	return
}

func (mag Magazine) GenerateOCIv1Config(ctx context.Context, maniSI types.Manifest, origMani types.ManifestData) (v1Cfg v1.Image, err error) {
	ctx, span := tracer.Start(ctx, "GenerateOCIv1Config")
	defer span.End()
	// TODO: Fetch config from Minio
	v1Cfg, found, err := mag.Minio.GetConfig(ctx, origMani.Config.Digest)
	if err != nil {
		return
	}
	if !found {
		err = fmt.Errorf("config with digest '%s' not found in object store", origMani.Config.Digest)
		return
	}
	msg := fmt.Sprintf("cfg.Labels: %d", len(v1Cfg.Config.Labels))
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.message", msg),
	))
	for k, v := range v1Cfg.Config.Labels {
		msg := fmt.Sprintf("cfg.Labels-%s=%s", k, v)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "debug"),
			attribute.String("log.message", msg),
		))
	}
	msg = fmt.Sprintf("origMani.Config.Labels: %d", len(origMani.Config.Labels.ToMap()))
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.message", msg),
	))
	for k, v := range origMani.Config.Labels.ToMap() {
		msg := fmt.Sprintf("origMani.Config.Labels-%s=%s", k, v)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "debug"),
			attribute.String("log.message", msg),
		))
	}
	msg = fmt.Sprintf("origMani.Annotations: %d", len(origMani.Annotations.ToMap()))
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.message", msg),
	))
	for k, v := range origMani.Annotations.ToMap() {
		msg := fmt.Sprintf("origMani.Annotations-%s=%s", k, v)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "debug"),
			attribute.String("log.message", msg),
		))
	}
	msg = fmt.Sprintf("maniSI.Annotations: %d", len(maniSI.Annotations.ToMap()))
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.message", msg),
	))
	for k, v := range maniSI.Annotations.ToMap() {
		msg := fmt.Sprintf("maniSI.Annotations-%s=%s", k, v)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "debug"),
			attribute.String("log.message", msg),
		))
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
				msg := fmt.Sprintf("Label '%s' already present in config: '%s'", k, cur)
				span.AddEvent("log", trace.WithAttributes(
					attribute.String("log.severity", "warn"),
					attribute.String("log.message", msg),
				))
				logrus.Warn(msg)
			}
		} else {
			msg := fmt.Sprintf("origMani.Config.Labels: Add label '%s' with value '%s'", k, v)
			span.AddEvent("log", trace.WithAttributes(
				attribute.String("log.severity", "debug"),
				attribute.String("log.message", msg),
			))
			logrus.Debug(msg)
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
					msg := fmt.Sprintf("Label '%s' already present in config: '%s'", k, cur)
					span.AddEvent("log", trace.WithAttributes(
						attribute.String("log.severity", "warn"),
						attribute.String("log.message", msg),
					))
					logrus.Warn(msg)
				}
			} else {
				msg := fmt.Sprintf("maniSI.Annotations: Add label '%s' with value '%s'", k, v)
				span.AddEvent("log", trace.WithAttributes(
					attribute.String("log.severity", "debug"),
					attribute.String("log.message", msg),
				))
				logrus.Debug(msg)
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
