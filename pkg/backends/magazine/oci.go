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
	pkgNameOci                = "magazine/oci.go"
	FailUnwindProfile         = "failed to unwind Profile"
	MediaTypeDockerManifestV2 = "application/vnd.docker.distribution.manifest.v2+json"
	keyManiID                 = "mani.ID"
	attrLogSeverity           = "log.severity"
	attrLogMessage            = "log.message"
	fmtGetProfileForMD        = "failed to get Profile for Manifest.ID: %d"
	fmtOCIv1AlreadyPresent    = "OCIv1 manifest already present for Manifest.ID: %d"
	fmtOCIv1NotPresent        = "OCIv1 manifest NOT present for Manifest.ID: %d"
	errCreateBaseManifest     = "failed to create base manifest"
	errLinkProfileManifest    = "failed to link profile and manifestdata"
)

func (mag Magazine) CreateOCIv1UploadedByPrf(ctx context.Context, mani types.ManifestWrapper, origMani types.Manifest, prf types.Profile) (maniID int64, found bool, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgNameOci,
		"func":    "CreateOCIv1UploadedByPrf",
		keyManiID: mani.ID,
	})
	ctx, span := tracer.Start(ctx, "CreateOCIv1UploadedByPrf")
	defer span.End()
	// Get UPLOADED Manifests
	prf, found, e := mag.Database.ProfileGetUploadedByViaManifestID(ctx, origMani.ID)
	if e != nil {
		err = errors.Wrapf(e, fmtGetProfileForMD, origMani.ID)
		return
	}
	if !found {
		return
	}
	// TODO: Check if OCIv1 is already present
	_, foundOCI, err := mag.Database.ManifestGetByProfileMediaType(ctx, mani.ID, prf.ID, []string{v1.MediaTypeImageManifest, MediaTypeDockerManifestV2})
	if err != nil {
		return
	}
	if foundOCI {
		msg := fmt.Sprintf(fmtOCIv1AlreadyPresent, mani.ID)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String(attrLogSeverity, "debug"),
			attribute.String(attrLogMessage, msg),
		))
		l.Info(msg)
		return
	} else {
		msg := fmt.Sprintf(fmtOCIv1NotPresent, mani.ID)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String(attrLogSeverity, "debug"),
			attribute.String(attrLogMessage, msg),
		))
		l.Info(msg)
	}
	maniID, e = mag.OCICreateBaseManifest(ctx, mani, origMani, prf.ID)
	if e != nil {
		err = errors.Wrap(e, errCreateBaseManifest)
		return
	}
	msg := fmt.Sprintf("Profile '%s' did the upload", prf.Label)
	l.Debug(msg)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String(attrLogSeverity, "debug"),
		attribute.String(attrLogMessage, msg),
	))
	e = mag.Database.LinkManifestToProfile(ctx, maniID, prf.ID, "UPLOADED_BY")
	if e != nil {
		err = errors.Wrap(e, errLinkProfileManifest)
		l.Error(err)
		return
	}
	// TODO: We need to unwind the rules so that annotations are added to the Manifest
	e = mag.Database.ProfileUnwindRules(ctx, prf.ID, maniID)
	if e != nil {
		err = errors.Wrap(e, FailUnwindProfile)
		l.Error(err)
		return

	}
	return
}

func (mag Magazine) CreateOCIv1UploadedBy(ctx context.Context, mani types.ManifestWrapper, origMani types.Manifest) (maniID int64, found bool, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgNameOci,
		"func":    "CreateOCIv1UploadedBy",
		keyManiID: mani.ID,
	})
	ctx, span := tracer.Start(ctx, "CreateOCIv1UploadedBy")
	defer span.End()
	// Get UPLOADED Manifests
	prf, found, e := mag.Database.ProfileGetUploadedByViaManifestID(ctx, origMani.ID)
	if e != nil {
		err = errors.Wrapf(e, fmtGetProfileForMD, origMani.ID)
		return
	}
	if !found {
		return
	}
	// TODO: Check if OCIv1 is already present
	_, foundOCI, err := mag.Database.ManifestGetByProfileMediaType(ctx, mani.ID, prf.ID, []string{v1.MediaTypeImageManifest, MediaTypeDockerManifestV2})
	if err != nil {
		return
	}
	if foundOCI {
		msg := fmt.Sprintf(fmtOCIv1AlreadyPresent, mani.ID)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String(attrLogSeverity, "debug"),
			attribute.String(attrLogMessage, msg),
		))
		l.Info(msg)
		return
	} else {
		msg := fmt.Sprintf(fmtOCIv1NotPresent, mani.ID)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String(attrLogSeverity, "debug"),
			attribute.String(attrLogMessage, msg),
		))
		l.Info(msg)
	}
	prf, prfFound, e := mag.Database.ProfileGetUploadedByViaManifestID(ctx, origMani.ID)
	if e != nil {
		err = errors.Wrapf(e, fmtGetProfileForMD, origMani.ID)
		return
	}
	maniID, e = mag.OCICreateBaseManifest(ctx, mani, origMani, prf.ID)
	if e != nil {
		err = errors.Wrap(e, errCreateBaseManifest)
		return
	}
	if prfFound {
		msg := fmt.Sprintf("Profile '%s' did the upload", prf.Label)
		l.Debug(msg)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String(attrLogSeverity, "debug"),
			attribute.String(attrLogMessage, msg),
		))
		e := mag.Database.LinkManifestToProfile(ctx, maniID, prf.ID, "UPLOADED_BY")
		if e != nil {
			err = errors.Wrap(e, errLinkProfileManifest)
			l.Error(err)
			return
		}
		// TODO: We need to unwind the rules so that annotations are added to the Manifest
		e = mag.Database.ProfileUnwindRules(ctx, prf.ID, maniID)
		if e != nil {
			err = errors.Wrap(e, FailUnwindProfile)
			l.Error(err)
			return
		}
	}
	return
}

func (mag Magazine) CreateOCIv1PulledByPrf(ctx context.Context, mani types.ManifestWrapper, origMani types.Manifest, prf types.Profile) (maniID int64, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameOci,
		"func":    "CreateOCIv1PulledByPrf",
		keyManiID: mani.ID,
	})
	defer span.End()
	// Get Pulled Manifests
	maniID, foundManiData, err := mag.Database.ManifestGetByProfileMediaType(ctx, mani.ID, prf.ID, []string{v1.MediaTypeImageManifest, MediaTypeDockerManifestV2})
	if err != nil {
		utils.ErrorHandler(l, span, err, "failed to get Manifest for Manifest.ID: %d", mani.ID)
		return
	}
	if foundManiData {
		utils.Info(l, span, fmtOCIv1AlreadyPresent, mani.ID)
		return
	} else {
		utils.Debug(l, span, fmtOCIv1NotPresent, mani.ID)
	}
	maniID, e := mag.OCICreateBaseManifest(ctx, mani, origMani, prf.ID)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, errCreateBaseManifest)
		return
	}
	utils.Warn(l, span, "Profile '%s' did the pull", prf.Label)
	e = mag.Database.LinkManifestToProfile(ctx, maniID, prf.ID, "UPLOADED_BY")
	if e != nil {
		err = utils.ErrorHandler(l, span, e, errLinkProfileManifest)
		return
	}
	// TODO: We need to unwind the rules so that annotations are added to the Manifest
	e = mag.Database.ProfileUnwindRules(ctx, prf.ID, maniID)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, FailUnwindProfile)
		return
	}

	return
}

func (mag Magazine) CreateOCIv1ParsedManifest(ctx context.Context, mani types.ManifestWrapper, origMani types.Manifest, prfIDs ...int64) (maniID int64, success bool, err error) {
	prfID := int64(0)
	if len(prfIDs) == 1 {
		prfID = prfIDs[0]
	}
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameOci,
		"func":    "CreateOCIv1ParsedManifest",
		keyManiID: mani.ID,
		"prfID":   prfID,
	})
	defer span.End()
	// -> If not parsed - skip!
	if origMani.Type != "PARSED" && origMani.Type != "ORIGINAL" {
		utils.Warn(l, span, "Manifest.ID: %d is not parsed NOR original (%s), skipping", mani.ID, origMani.Type)
		return
	}

	// Get parsed manifests
	maniID, foundOCI, err := mag.Database.ManifestGetRelIDByMediaType(ctx, mani.ID, []string{v1.MediaTypeImageManifest, MediaTypeDockerManifestV2}, "PARSED")
	if err != nil {
		return
	}
	if foundOCI {
		l.Debugf(fmtOCIv1AlreadyPresent, mani.ID)
		success = true
		return
	}
	maniID, e := mag.OCICreateBaseManifest(ctx, mani, origMani, prfID)
	if e != nil {
		err = errors.Wrap(e, errCreateBaseManifest)
		return
	}
	success = true
	if err = mag.Database.ProfileUnwindRules(ctx, prfID, maniID); err != nil {
		err = errors.Wrap(err, FailUnwindProfile)
		return
	}
	return
}

func (mag Magazine) CreateOCIv1ForProfile(ctx context.Context, mani types.ManifestWrapper, origMani types.Manifest, prf types.Profile) (maniDataID int64, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":       pkgNameOci,
		"func":          "CreateOCIv1ForProfile",
		keyManiID:       mani.ID,
		"maniData.ID":   origMani.ID,
		"maniData.Type": origMani.Type,
		"prf.ID":        prf.ID,
	})
	defer span.End()
	utils.Warn(l, span, "Creating OCIv1 manifest for Manifest.ID: %d // Manifest.type: '%s'", mani.ID, origMani.Type)
	switch origMani.Type {
	case "UPLOADED_BY", "MANIFEST_DERIVED_FROM":
		mdID, success, e := mag.CreateOCIv1UploadedByPrf(ctx, mani, origMani, prf)
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
		mdID, success, e := mag.CreateOCIv1PulledByPrf(ctx, mani, origMani, prf)
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
			attribute.String(attrLogSeverity, "error"),
			attribute.String(attrLogMessage, msg),
		))
		span.RecordError(err)
		l.Error(msg)
		return
	}
	return
}

func (mag Magazine) CreateOCIv1(ctx context.Context, mani types.ManifestWrapper, origMani types.Manifest) (maniDataID int64, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package":       pkgNameOci,
		"func":          "CreateOCIv1",
		keyManiID:       mani.ID,
		"maniData.ID":   origMani.ID,
		"maniData.Type": origMani.Type,
	})
	l.Debugf("Creating OCIv1 manifest for Manifest.ID: %d", mani.ID)
	ctx, span := tracer.Start(ctx, "CreateOCIv1")
	defer span.End()
	// TODO: Check which profiles where used for the manifest
	maniPrf, e := mag.Database.ProfileFromManifestWrapperID(ctx, mani.ID)
	if e != nil {
		err = errors.Wrapf(e, "failed to get profiles for manifest id '%d'", mani.ID)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String(attrLogSeverity, "error"),
			attribute.String(attrLogMessage, err.Error()),
		))
		span.RecordError(err)
		l.Error(err.Error())
		span.SetStatus(codes.Error, err.Error())
		return
	}
	msg := fmt.Sprintf("found %d profiles for manifest: %v", len(maniPrf), maniPrf)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String(attrLogSeverity, "debug"),
		attribute.String(attrLogMessage, msg),
	))
	l.Debug(msg)
	switch origMani.Type {
	case "PARSED", "ORIGINAL":
		// Phase 4 (deprecate-manifest-wrapper): the content-leaf attaches to
		// Upstream via :ORIGINAL, so a leaf-keyed reshape sees Type "ORIGINAL".
		// Treat it like "PARSED" (CreateOCIv1ParsedManifest already accepts
		// both), mirroring CreateOCIv1ForProfile's PARSED/ORIGINAL grouping.
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
	case "UPLOADED_BY", "MANIFEST_DERIVED_FROM":
		mdID, success, e := mag.CreateOCIv1UploadedBy(ctx, mani, origMani)
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
			attribute.String(attrLogSeverity, "error"),
			attribute.String(attrLogMessage, msg),
		))
		span.RecordError(err)
		l.Error(msg)
		return
	}
	return
}

func (mag Magazine) OCIApplyManipulations(ctx context.Context, mani types.ManifestWrapper, origMani *types.Manifest, prfID, ociManiDataID int64) (v1Cfg *v1.Image, ok bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":     pkgNameOci,
		"func":        "OCIApplyManipulations",
		keyManiID:     mani.ID,
		"origMani.ID": origMani.ID,
		"prfID":       prfID,
	})
	defer span.End()
	if ok, err := mag.Database.ManipulationCheckRules(ctx, prfID, mani.ID); err != nil {
		return v1Cfg, false, utils.ErrorHandler(l, span, err, "failing ManipulationCheckRules")
	} else if !ok {
		utils.Warn(l, span, "ManipulationCheckRules reports that the manifest does not match the profile to apply manipulations")
		return v1Cfg, false, nil
	}

	// Once the origMani becomes ociMani, we apply the manipulations
	mplCfgDigest, e := mag.ManipulationUnwind(ctx, mani.ID, ociManiDataID, prfID)
	if e != nil {
		return v1Cfg, false, utils.ErrorHandler(l, span, e, "while in ManipulationUnwind")
	}
	utils.Debug(l, span, "Obtained manipulation config digest: %s", mplCfgDigest)

	// A. Generate `oci.image.config.v1`
	if manipuLayer, err := mag.Database.GetLayersForManifestID(ctx, ociManiDataID); err != nil {
		return v1Cfg, false, utils.ErrorHandler(l, span, err, "while in GetLayersForManifestID")
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
		if e := mag.ApplyManipulationToConfig(ctx, mani.ID, prfID, v1Cfg, mplCfg); e != nil {
			return v1Cfg, false, utils.ErrorHandler(l, span, e, "while in ApplyManipulationToConfig")
		}
	}
	utils.Debug(l, span, "Config.Cmd: %v", v1Cfg.Config.Cmd)
	utils.Debug(l, span, "Config.Entrypoint: %v", v1Cfg.Config.Entrypoint)
	utils.Debug(l, span, "Config.Env: %v", v1Cfg.Config.Env)
	utils.Debug(l, span, "Config.RootFS.DiffIDs: %v", v1Cfg.RootFS.DiffIDs)
	return v1Cfg, true, nil
}

// TODO: When no manipulation are applied, no update is needed
func (mag Magazine) OCICreateBaseManifest(ctx context.Context, mani types.ManifestWrapper, origMani types.Manifest, prfID int64) (ociManiDataID int64, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":     pkgNameOci,
		"func":        "OCICreateBaseManifest",
		keyManiID:     mani.ID,
		"origMani.ID": origMani.ID,
		"prfID":       prfID,
	})
	defer span.End()
	digStr := fmt.Sprintf("%s#%s-%d", origMani.Upstream.GetLabel(), origMani.Digest, prfID)
	tempDigest := digest.FromString(digStr)
	utils.Debug(l, span, "Original Manifest Config Digest: %s", origMani.Config.Digest)
	// Empty Manifest
	plat := &v1.Platform{}
	if origCfg, found, err := mag.Database.ConfigGetByManiDataID(ctx, origMani.ID); err != nil {
		return 0, utils.ErrorHandler(l, span, err, "failed to get original config for manifestdata id %d", origMani.ID)
	} else if !found {
		return 0, utils.NewError(l, span, "failed to find original config for manifestdata id %d", origMani.ID)
	} else {
		cfg, err := mag.Minio.GetConfig(ctx, origCfg.Digest)
		if err != nil {
			return 0, utils.ErrorHandler(l, span, err, "failed to get original config from minio for digest %s", origCfg.Digest)
		}
		utils.Debug(l, span, "Original Manifest Config ID: %d > %s/%s/%s", origCfg.ID, cfg.OS, cfg.Architecture, cfg.Variant)
		plat.Architecture = cfg.Architecture
		plat.OS = cfg.OS
		plat.Variant = cfg.Variant
	}

	m := v1.Manifest{
		Versioned: specs.Versioned{
			SchemaVersion: 2,
		},
		Config: v1.Descriptor{
			MediaType: v1.MediaTypeImageConfig,
			Digest:    tempDigest,
			Platform:  plat,
		},
		MediaType: v1.MediaTypeImageManifest,
	}
	rs, e := mag.Database.ProfileGetAnnoEqRules(ctx, prfID)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to get anno rules for profile %d", prfID)
		return
	}
	// Create a new ociManifest as a copy of origMani (as OCIv1)
	// Graph
	maniDesc, err := ManifestV1ToDescriptor(m)
	if err != nil {
		return
	}
	utils.Debug(l, span, "Temp digest: %s", maniDesc.Digest.String())
	ociManiDataID, _, err = mag.Database.ManifestV1GetInsert(ctx, mani.ID, maniDesc)
	if err != nil {
		return
	}
	v1Cfg, ok, e := mag.OCIApplyManipulations(ctx, mani, &origMani, prfID, ociManiDataID)
	if e != nil {
		return ociManiDataID, utils.ErrorHandler(l, span, e, "failed to apply manipulations")
	} else if !ok {
		v1Cfg, e = mag.GenerateOCIv1Config(ctx, mani, origMani)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to GenerateOCIv1Config after failed manipulation")
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
	e = mag.Minio.PutBlob(ctx, cfgDesc.Data, cfgDesc.Digest.String())
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to PutConfig")
		return
	}
	cfg, _, e := mag.Database.OCIv1ConfigGetInsertDescriptor(ctx, cfgDesc)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to OCIv1ConfigGetInsertDescriptor")
		return
	}
	// Add Labels to Config
	utils.Debug(l, span, "Found %d origMani.Config.Cfg.Config.Labels", len(origMani.Config.Cfg.Config.Labels))
	for k, v := range origMani.Config.Cfg.Config.Labels {
		utils.Warn(l, span, "origMani.Config.Cfg.Config.Labels: %s=%s", k, v)
		a := types.NewAnnotation(k, v)
		err = mag.Database.ConfigAddLabel(ctx, cfg.ID, a)
		if err != nil {
			err = utils.ErrorHandler(l, span, err, "while in ConfigAddLabel")
			return
		}
	}
	// Generate Layers: Create OCI layer media-types:  application/vnd.oci.image.layer.v1.tar[+gzip]
	layerMap, err := mag.LayersCreateOCIv1LayerDescriptors(ctx, origMani.Layers)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "failed to CreateOCIv1LayerDescriptors")
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
		err = utils.ErrorHandler(l, span, err, "failed to marshal manifest")
		return
	}
	maniDesc, err = ManifestV1ToDescriptor(m)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "failed to create manifest descriptor")
		return
	}
	if e := mag.Minio.ManifestsPutObj(ctx, maniDesc.Digest.String(), maniBytes); e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to Put Manifest object to Minio")
		return
	}
	// Graph
	err = mag.Database.ManifestUpdateDesc(ctx, ociManiDataID, maniDesc)
	if err != nil {
		return ociManiDataID, utils.ErrorHandler(l, span, err, "while in ManifestUpdate")
	}
	// Link Manifest to Annotations
	err = mag.Database.ManifestAddAnnotations(ctx, ociManiDataID, &mani.Annotations)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "failed to add Manifest annotations")
		return
	}
	// //// Link Manifest to Config
	err = mag.Database.ManifestLinkToOCIv1Config(ctx, ociManiDataID, cfg)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "failed to link Manifest to OCIv1 Config")
		return
	}
	// //// Link Manifest to Layers
	for _, ld := range layerMap {
		err = mag.Database.ManifestLinkToOCIv1Layer(ctx, ociManiDataID, ld)
		if err != nil {
			err = utils.ErrorHandler(l, span, err, "failed to link Manifest to OCIv1 Layer")
			return
		}
	}
	utils.Info(l, span, "Created OCIv1 Manifest.ID: %d for Manifest.ID: %d", ociManiDataID, mani.ID)
	return
}

func (mag Magazine) GenerateOCIv1Config(ctx context.Context, maniSI types.ManifestWrapper, origMani types.Manifest) (v1Cfg *v1.Image, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":                pkgNameOci,
		"func":                   "mag.GenerateOCIv1Config",
		"maniID":                 maniSI.ID,
		"origManiID":             origMani.ID,
		"origMani.Config.Digest": origMani.Config.Digest,
	})
	defer span.End()
	// Fetch config from Minio
	v1Cfg, err = mag.Minio.GetConfig(ctx, origMani.Config.Digest)
	if err != nil {
		utils.ErrorHandler(l, span, err, "failed to GetConfig")
		return
	}
	if v1Cfg == nil {
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

func (mag Magazine) GenerateOCIv1ConfigDescriptor(ctx context.Context, v1Cfg *v1.Image) (dscr v1.Descriptor, err error) {
	cfgBytes, err := json.Marshal(v1Cfg)
	if err != nil {
		return
	}
	dig := digest.FromBytes(cfgBytes)
	size := len(cfgBytes)
	// Get Labels from Config attached to Manifest and Annotations from Manifest
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
		Platform:    m.Config.Platform,
	}
	return
}
