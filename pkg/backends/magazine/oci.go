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
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	pkgNameOci        = "magazine/oci.go"
	FailUnwindProfile = "failed to unwind Profile"
)

func (mag Magazine) CreateOCIv1_UploadedByPrf(ctx context.Context, mani types.Manifest, origMani types.ManifestData, prf types.Profile) (maniID int64, found bool, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgNameOci,
		"func":    "CreateOCIv1_UploadedByPrf",
		"mani.ID": mani.ID,
	})
	ctx, span := tracer.Start(ctx, "CreateOCIv1_UploadedByPrf")
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
	_, foundOCI, err := mag.Database.ManifestData_Get_ByProfileMediaType(ctx, mani.ID, prf.ID, v1.MediaTypeImageManifest)
	if err != nil {
		return
	}
	if foundOCI {
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
	maniID, e = mag.OCI_CreateBaseManifest(ctx, mani, origMani, prf.ID)
	if e != nil {
		err = errors.Wrap(e, "failed to create base manifest")
		return
	}
	msg := fmt.Sprintf("Profile '%s' did the upload", prf.Label)
	l.Debug(msg)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.message", msg),
	))
	e = mag.Database.LinkManifestDataToProfile(ctx, maniID, prf.ID, "UPLOADED_BY")
	if e != nil {
		err = errors.Wrap(e, "failed to link profile and manifestdata")
		l.Error(err)
		return
	}
	// TODO: We need to unwind the rules so that annotations are added to the ManifestData
	e = mag.Database.ProfileUnwindRules(ctx, prf.ID, maniID)
	if e != nil {
		err = errors.Wrap(e, FailUnwindProfile)
		l.Error(err)
		return

	}
	return
}

func (mag Magazine) CreateOCIv1_UploadedBy(ctx context.Context, mani types.Manifest, origMani types.ManifestData) (maniID int64, found bool, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgNameOci,
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
	_, foundOCI, err := mag.Database.ManifestData_Get_ByProfileMediaType(ctx, mani.ID, prf.ID, v1.MediaTypeImageManifest)
	if err != nil {
		return
	}
	if foundOCI {
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
	prf, prfFound, e := mag.Database.Profile_GetUploadedBy_ViaManifestDataID(ctx, origMani.ID)
	if e != nil {
		err = errors.Wrapf(e, "failed to get Profile for ManifestData.ID: %d", origMani.ID)
		return
	}
	maniID, e = mag.OCI_CreateBaseManifest(ctx, mani, origMani, prf.ID)
	if e != nil {
		err = errors.Wrap(e, "failed to create base manifest")
		return
	}
	if prfFound {
		msg := fmt.Sprintf("Profile '%s' did the upload", prf.Label)
		l.Debug(msg)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "debug"),
			attribute.String("log.message", msg),
		))
		e := mag.Database.LinkManifestDataToProfile(ctx, maniID, prf.ID, "UPLOADED_BY")
		if e != nil {
			err = errors.Wrap(e, "failed to link profile and manifestdata")
			l.Error(err)
			return
		}
		// TODO: We need to unwind the rules so that annotations are added to the ManifestData
		e = mag.Database.ProfileUnwindRules(ctx, prf.ID, maniID)
		if e != nil {
			err = errors.Wrap(e, FailUnwindProfile)
			l.Error(err)
			return
		}
	}
	return
}

func (mag Magazine) CreateOCIv1_PulledByPrf(ctx context.Context, mani types.Manifest, origMani types.ManifestData, prf types.Profile) (maniID int64, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameOci,
		"func":    "CreateOCIv1_PulledByPrf",
		"mani.ID": mani.ID,
	})
	defer span.End()
	// Get Pulled Manifests
	maniID, foundManiData, err := mag.Database.ManifestData_Get_ByProfileMediaType(ctx, mani.ID, prf.ID, v1.MediaTypeImageManifest)
	if err != nil {
		utils.ErrorHandler(l, span, err, "failed to get ManifestData for Manifest.ID: %d", mani.ID)
		return
	}
	if foundManiData {
		utils.Info(l, span, "OCIv1 manifest already present for Manifest.ID: %d", mani.ID)
		return
	} else {
		utils.Debug(l, span, "OCIv1 manifest NOT present for Manifest.ID: %d", mani.ID)
	}
	maniID, e := mag.OCI_CreateBaseManifest(ctx, mani, origMani, prf.ID)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to create base manifest")
		return
	}
	utils.Warn(l, span, "Profile '%s' did the pull", prf.Label)
	e = mag.Database.LinkManifestDataToProfile(ctx, maniID, prf.ID, "UPLOADED_BY")
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to link profile and manifestdata")
		return
	}
	// TODO: We need to unwind the rules so that annotations are added to the ManifestData
	e = mag.Database.ProfileUnwindRules(ctx, prf.ID, maniID)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, FailUnwindProfile)
		return
	}

	return
}

func (mag Magazine) CreateOCIv1ParsedManifest(ctx context.Context, mani types.Manifest, origMani types.ManifestData, prfIDs ...int64) (maniID int64, success bool, err error) {
	prfID := int64(0)
	if len(prfIDs) == 1 {
		prfID = prfIDs[0]
	}
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameOci,
		"func":    "CreateOCIv1ParsedManifest",
		"mani.ID": mani.ID,
		"prfID":   prfID,
	})
	defer span.End()
	// TODO: Is the manifest in question uploaded or parsed?
	// -> If not parsed - skip!
	if origMani.Type != "PARSED" && origMani.Type != "ORIGINAL" {
		utils.Warn(l, span, "Manifest.ID: %d is not parsed (%s), skipping", mani.ID, origMani.Type)
		return
	}

	// Get parsed manifests
	maniID, foundOCI, err := mag.Database.ManifestData_GetRelID_ByMediaType(ctx, mani.ID, v1.MediaTypeImageManifest, "PARSED")
	if err != nil {
		return
	}
	if foundOCI {
		l.Debugf("OCIv1 manifest already present for Manifest.ID: %d", mani.ID)
		success = true
		return
	}
	maniID, e := mag.OCI_CreateBaseManifest(ctx, mani, origMani, prfID)
	if e != nil {
		err = errors.Wrap(e, "failed to create base manifest")
		return
	}
	success = true
	if err = mag.Database.ProfileUnwindRules(ctx, prfID, maniID); err != nil {
		err = errors.Wrap(err, FailUnwindProfile)
		return
	}
	return
}

func (mag Magazine) CreateOCIv1ForProfile(ctx context.Context, mani types.Manifest, origMani types.ManifestData, prf types.Profile) (maniDataID int64, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":       pkgNameOci,
		"func":          "CreateOCIv1ForProfile",
		"mani.ID":       mani.ID,
		"maniData.ID":   origMani.ID,
		"maniData.Type": origMani.Type,
		"prf.ID":        prf.ID,
	})
	defer span.End()
	utils.Warn(l, span, "Creating OCIv1 manifest for Manifest.ID: %d // ManifestData.type: '%s'", mani.ID, origMani.Type)
	switch origMani.Type {
	case "UPLOADED_BY":
		mdID, success, e := mag.CreateOCIv1_UploadedByPrf(ctx, mani, origMani, prf)
		if e != nil {
			err = errors.Wrapf(e, "failed to create OCIv1 manifest for uploaded Manifest.ID: %d", mani.ID)
			return
		}
		if success {
			l.Warnf("Manifest.ID: %d was uploaded by a user, created new OCI manifest", mani.ID)
			maniDataID = mdID
			return
		}
	case "PULLED_BY":
		mdID, success, e := mag.CreateOCIv1_PulledByPrf(ctx, mani, origMani, prf)
		if e != nil {
			err = errors.Wrapf(e, "failed to create OCIv1 manifest for pulled Manifest.ID: %d", mani.ID)
			return
		}
		if success {
			l.Warnf("Manifest.ID: %d was pulled by a user, created new OCI manifest", mani.ID)
			maniDataID = mdID
			return
		}
	case "PARSED", "ORIGINAL":
		mdID, success, e := mag.CreateOCIv1ParsedManifest(ctx, mani, origMani, prf.ID)
		if e != nil {
			err = errors.Wrapf(e, "failed to create OCIv1 manifest for parsed Manifest.ID: %d", mani.ID)
			return
		}
		if success {
			l.Warnf("Manifest.ID: %d was parsed, created new OCI manifest", mani.ID)
			maniDataID = mdID
			return
		}
	default:
		msg := fmt.Sprintf("unknown manifest data type: %s", origMani.Type)
		err = errors.New(msg)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "error"),
			attribute.String("log.message", msg),
		))
		span.RecordError(err)
		l.Error(msg)
		return
	}
	return
}

func (mag Magazine) CreateOCIv1(ctx context.Context, mani types.Manifest, origMani types.ManifestData) (maniDataID int64, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package":       pkgNameOci,
		"func":          "CreateOCIv1",
		"mani.ID":       mani.ID,
		"maniData.ID":   origMani.ID,
		"maniData.Type": origMani.Type,
	})
	l.Debugf("Creating OCIv1 manifest for Manifest.ID: %d", mani.ID)
	ctx, span := tracer.Start(ctx, "CreateOCIv1")
	defer span.End()
	// TODO: Check which profiles where used for the manifest
	maniPrf, e := mag.Database.Profile_FromManifestID(ctx, mani.ID)
	if e != nil {
		err = errors.Wrapf(e, "failed to get profiles for manifest id '%d'", mani.ID)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "error"),
			attribute.String("log.message", err.Error()),
		))
		span.RecordError(err)
		l.Error(err.Error())
		span.SetStatus(codes.Error, err.Error())
		return
	}
	msg := fmt.Sprintf("found %d profiles for manifest: %v", len(maniPrf), maniPrf)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.message", msg),
	))
	l.Debug(msg)
	switch origMani.Type {
	case "PARSED":
		mdID, success, e := mag.CreateOCIv1ParsedManifest(ctx, mani, origMani)
		if e != nil {
			err = errors.Wrapf(e, "failed to create OCIv1 manifest for parsed Manifest.ID: %d", mani.ID)
			return
		}
		if success {
			l.Warnf("Manifest.ID: %d was parsed, created new OCI manifest", mani.ID)
			maniDataID = mdID
			return
		}
	case "UPLOADED_BY":
		mdID, success, e := mag.CreateOCIv1_UploadedBy(ctx, mani, origMani)
		if e != nil {
			err = errors.Wrapf(e, "failed to create OCIv1 manifest for uploaded Manifest.ID: %d", mani.ID)
			return
		}
		if success {
			l.Warnf("Manifest.ID: %d was uploaded by a user, created new OCI manifest", mani.ID)
			maniDataID = mdID
			return
		}
	default:
		msg := fmt.Sprintf("unknown manifest data type: %s", origMani.Type)
		err = errors.New(msg)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "error"),
			attribute.String("log.message", msg),
		))
		span.RecordError(err)
		l.Error(msg)
		return
	}
	return
}

func (mag Magazine) OCIApplyManipulations(ctx context.Context, mani types.Manifest, origMani *types.ManifestData, prfID, ociManiDataID int64) (v1Cfg v1.Image, ok bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":     pkgNameOci,
		"func":        "OCIApplyManipulations",
		"mani.ID":     mani.ID,
		"origMani.ID": origMani.ID,
		"prfID":       prfID,
	})
	defer span.End()
	if ok, err := mag.Database.ManinpulationCheckRules(ctx, prfID, mani.ID); err != nil {
		return v1Cfg, false, utils.ErrorHandler(l, span, err, "failing ManinpulationCheckRules")
	} else if !ok {
		utils.Warn(l, span, "ManinpulationCheckRules reports that the manifest does not match the profile to apply manipulations")
		return v1Cfg, false, nil
	}

	// Once the origMani becomes ociMani, we apply the manipulations
	mplCfgDigest, e := mag.ManipulationUnwind(ctx, mani.ID, ociManiDataID, prfID)
	if e != nil {
		return v1Cfg, false, utils.ErrorHandler(l, span, e, "while in ManipulationUnwind")
	}
	// ///// A. Generate `oci.image.config.v1`
	if manipuLayer, err := mag.Database.GetLayersForManifestDataID(ctx, ociManiDataID); err != nil {
		return v1Cfg, false, utils.ErrorHandler(l, span, err, "while in GetLayersForManifestDataID")
	} else {
		origMani.Layers = append(origMani.Layers, manipuLayer...)
	}
	v1Cfg, e = mag.GenerateOCIv1Config(ctx, mani, *origMani)
	if e != nil {
		return v1Cfg, false, utils.ErrorHandler(l, span, e, "failed to GenerateOCIv1Config")
	}
	// Add Config from ociManifest to config from original
	if mplCfgDigest != "" {
		mplCfg, err := mag.Minio.ConfigGetObj(ctx, mplCfgDigest)
		if err != nil {
			return v1Cfg, false, utils.ErrorHandler(l, span, err, "while in ConfigGetObj")
		}
		for _, diffID := range mplCfg.Cfg.RootFS.DiffIDs {
			utils.Info(l, span, "Adding diffID: %s", diffID.String())
			v1Cfg.RootFS.DiffIDs = append(v1Cfg.RootFS.DiffIDs, diffID)
		}
		if e := mag.ApplyManipulationToConfig(ctx, mani.ID, prfID, &v1Cfg, mplCfg); e != nil {
			return v1Cfg, false, utils.ErrorHandler(l, span, e, "while in ApplyManipulationToConfig")
		}
	}
	return v1Cfg, true, nil
}

// TODO: When no manipulation are applied, no update is needed
func (mag Magazine) OCI_CreateBaseManifest(ctx context.Context, mani types.Manifest, origMani types.ManifestData, prfID int64) (ociManiDataID int64, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameOci,
		"func":    "OCI_CreateBaseManifest",
		"mani.ID": mani.ID,
		"prfID":   prfID,
	})
	defer span.End()
	digStr := fmt.Sprintf("%s#%s-%d", origMani.Upstream.GetLabel(), origMani.Digest, prfID)
	tempDigest := digest.FromString(digStr)
	// Empty Manifest
	m := v1.Manifest{
		Versioned: specs.Versioned{
			SchemaVersion: 2,
		},
		Config: v1.Descriptor{
			MediaType: v1.MediaTypeImageConfig,
			Digest:    tempDigest,
		},
		MediaType: v1.MediaTypeImageManifest,
	}
	rs, e := mag.Database.ProfileGetAnnoEqRules(ctx, prfID)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to get anno rules for profile %d", prfID)
		return
	}
	// Create a new ociManifestData as a copy of origMani (as OCIv1)
	// Graph
	maniDesc, err := ManifestV1ToDescriptor(m)
	if err != nil {
		return
	}
	utils.Warn(l, span, "Temp digest: %s", maniDesc.Digest.String())
	ociManiDataID, _, err = mag.Database.ManifestData_V1_GetInsert(ctx, mani.ID, maniDesc)
	if err != nil {
		return
	}
	v1Cfg, ok, e := mag.OCIApplyManipulations(ctx, mani, &origMani, prfID, ociManiDataID)
	if e != nil {
		return ociManiDataID, utils.ErrorHandler(l, span, e, "failed to apply manipulations")
	} else if !ok {
		v1Cfg, e = mag.GenerateOCIv1Config(ctx, mani, origMani)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to GenerateOCIv1Config")
			return
		}
	}
	utils.Debug(l, span, "Found %d AnnoEqRules", len(rs))
	for idx, r := range rs {
		if r.Type == types.RuleTypeAnnotation && r.Op == "eq" {
			utils.Debug(l, span, "  AnnoEqRule %d > %s", idx, r.ToString())
			v1Cfg.Config.Labels[r.Key] = r.Value
			k := utils.UpLabelKey(r.Key)
			v1Cfg.Config.Env = append(v1Cfg.Config.Env, fmt.Sprintf("%s=%s", k, r.Value))
		}
		utils.Debug(l, span, "  [SKIP] NOT AnnoEqRule %d > %s", idx, r.ToString())
	}

	cfgDesc, e := mag.GenerateOCIv1ConfigDescriptor(ctx, v1Cfg)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to GenerateOCIv1ConfigDescriptor")
		return
	}
	m.Config = cfgDesc
	// Upload Config Blob to Minio
	e = mag.Minio.PutConfig(ctx, cfgDesc.Data, cfgDesc.Digest.String())
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to PutConfig")
		return
	}
	cfg, _, e := mag.Database.OCIv1Config_GetInsertDescriptor(ctx, cfgDesc)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to OCIv1Config_GetInsertDescriptor")
		return
	}
	// Add Labels to Config
	utils.Debug(l, span, "Found %d origMani.Config.Cfg.Config.Labels", len(origMani.Config.Cfg.Config.Labels))
	for k, v := range origMani.Config.Cfg.Config.Labels {
		utils.Warn(l, span, "origMani.Config.Cfg.Config.Labels: %s=%s", k, v)
		a := types.NewAnnotation(k, v)
		err = mag.Database.Config_AddLabel(ctx, cfg.ID, a)
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
	utils.Warn(l, span, "Package %d Layers", len(m.Layers))
	// //// Add Annotations
	m.Annotations = mani.Annotations.ToMap()

	/// Finalize the Manifest and the Config
	// Put Manifest into Graph and Minio
	maniBytes, err := json.Marshal(m)
	if err != nil {
		return
	}
	maniDesc, err = ManifestV1ToDescriptor(m)
	if err != nil {
		return
	}
	mag.Minio.ManifestsPutObj(ctx, maniDesc.Digest.String(), maniBytes)
	// Graph
	err = mag.Database.ManifestDataUpdateDesc(ctx, ociManiDataID, maniDesc)
	if err != nil {
		return ociManiDataID, utils.ErrorHandler(l, span, err, "while in ManifestDataUpdate")
	}
	// Link ManifestData to Annotations
	err = mag.Database.ManifestData_AddAnnotations(ctx, ociManiDataID, &mani.Annotations)
	if err != nil {
		return
	}
	// //// Link ManifestData to Config
	err = mag.Database.ManifestData_LinkToOCIv1Config(ctx, ociManiDataID, cfg)
	if err != nil {
		return
	}
	// //// Link ManifestData to Layers
	for _, ld := range layerMap {
		err = mag.Database.ManifestData_LinkToOCIv1Layer(ctx, ociManiDataID, ld)
		if err != nil {
			l.Error(err)
			return
		}
	}
	return
}

func (mag Magazine) GenerateOCIv1Config(ctx context.Context, maniSI types.Manifest, origMani types.ManifestData) (v1Cfg v1.Image, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameOci,
		"func":       "mag.GenerateOCIv1Config",
		"maniID":     maniSI.ID,
		"origManiID": origMani.ID,
	})
	defer span.End()
	// Fetch config from Minio
	v1Cfg, found, err := mag.Minio.GetConfig(ctx, origMani.Config.Digest)
	if err != nil {
		utils.ErrorHandler(l, span, err, "failed to GetConfig")
		return
	}
	if !found {
		err = utils.NewError(l, span, "config with digest '%s' not found in object store", origMani.Config.Digest)
		return
	}
	utils.Debug(l, span, "cfg.Labels: %d", len(v1Cfg.Config.Labels))
	for k, v := range v1Cfg.Config.Labels {
		utils.Debug(l, span, "cfg.Labels-%s=%s", k, v)
	}
	utils.Debug(l, span, "origMani.Config.Labels: %d", len(origMani.Config.Labels.ToMap()))
	for k, v := range origMani.Config.Labels.ToMap() {
		utils.Debug(l, span, "origMani.Config.Labels-%s=%s", k, v)
	}
	utils.Debug(l, span, "origMani.Annotations: %d", len(origMani.Annotations.ToMap()))
	for k, v := range origMani.Annotations.ToMap() {
		utils.Debug(l, span, "origMani.Annotations-%s=%s", k, v)
	}
	utils.Debug(l, span, "maniSI.Annotations: %d", len(maniSI.Annotations.ToMap()))
	for k, v := range maniSI.Annotations.ToMap() {
		utils.Debug(l, span, "maniSI.Annotations-%s=%s", k, v)
	}
	es := utils.NewEnvStore()
	// Put env in env
	for _, e := range v1Cfg.Config.Env {
		err = es.AddEnv(e)
		if err != nil {
			err = utils.ErrorHandler(l, span, err, "failed to AddEnv")
			return
		}
	}
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
				utils.Warn(l, span, "Label '%s' already present in config: '%s'", k, cur)
			}
		} else {
			utils.Debug(l, span, "origMani.Config.Labels: Add label '%s' with value '%s'", k, v)
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
					utils.Warn(l, span, "Label '%s' already present in config: '%s'", k, cur)
				}
			} else {
				utils.Debug(l, span, "maniSI.Annotations: Add label '%s' with value '%s'", k, v)
				v1Cfg.Config.Labels[k] = v
				es.Add(k, v)

			}
		}
	}
	v1Cfg.Config.Env = es.ToSlice()
	// TODO: Check if Environment violates
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
