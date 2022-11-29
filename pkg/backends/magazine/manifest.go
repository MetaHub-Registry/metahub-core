package magazine

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/backends/minio"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	ycfg "github.com/metahub-registry/metahub-core/pkg/yaml/config"
)

func (mag Magazine) GetManifest(ctx context.Context, m ycfg.Manifest) (mani types.Manifest, err error) {
	up, err := m.ToUpstreamTriple()
	if err != nil {
		logrus.Error(err)
		return
	}
	// The Manifest cached has a Data filed with the original data blob (matching the digest)
	// in addition it has fields populated with the parsed data.
	mani, isCached, err := mag.Minio.GetManifest(ctx, up)
	if err != nil {
		logrus.Error(err)
		return
	}
	if !isCached {
		err = errors.New("manifest is not cached")
		return
	}
	mani.Upstream = types.Upstream{
		Reg:    up.Reg,
		Repo:   up.Repo,
		Tag:    up.Tag,
		Digest: up.Digest,
	}
	return
}

func (mag Magazine) CachManifest(ctx context.Context, m ycfg.Manifest) (mani types.Manifest, err error) {
	up, err := m.ToUpstreamTriple()
	if err != nil {
		logrus.Error(err)
		return
	}
	// The Manifest cached has a Data filed with the original data blob (matching the digest)
	// in addition it has fields populated with the parsed data.
	mani, isCached, err := mag.Minio.GetManifest(ctx, up)
	if err != nil {
		logrus.Error(err)
		return
	}
	logrus.Infof("Caching manifest for %-101s -> %t", up.GetLabel(), isCached)
	if !isCached {
		b, blob, err := upstream.GetManifestByUpstreamTripleByTag(ctx, up)
		if err != nil {
			err = errors.Wrapf(err, "failed to GetManifestByUpstreamTripleByTag")
			logrus.Error(err)
			return mani, err
		}
		rawMani, err := minio.RawManifestFromBlob(b)
		if err != nil {
			logrus.Error(err)
			return mani, err
		}
		/*sB, err := rawMani.Serialize()
		if err != nil {
			logrus.Error(err)
		}
		logrus.Fatal("CachManifest: ", string(sB))
		*/
		switch blob.MediaType {
		case "application/vnd.docker.distribution.manifest.v1+prettyjws":
			err = fmt.Errorf("manifest type is not supported: %s", blob.MediaType)
			return mani, err
		case "application/vnd.docker.distribution.manifest.v2+json":
			up.Digest = rawMani.ConfigInfo().Digest
		default:
			logrus.Fatalf("Unknown media type: %s", blob.MediaType)
		}
		//logrus.Fatalf("blobMeta: %v", blob)
		// We need to make sure that the config-blob is cached
		cfgByte, cfgBlob, err := upstream.GetBlobByUpstreamTriple(ctx, up)
		if err != nil {
			err = errors.Wrapf(err, "failed to GetBlobByUpstreamTriple")
			logrus.Error(err)
			return mani, err
		}
		err = mag.Minio.PutConfig(ctx, cfgByte, cfgBlob.Digest.String())
		if err != nil {
			logrus.Error(err)
			return mani, err
		}
		// The blob 'b' is the original data blob (matching the digest), it contains refrerences to the config and layers
		mani, err = mag.Minio.NewManifestFromBytes(b)
		if err != nil {
			logrus.Error(err)
			return mani, err
		}
		up.Digest = digest.FromBytes(b)
		up.Size = blob.Size
		err = mag.Minio.PutManifest(ctx, b, up)
		if err != nil {
			logrus.Error(err)
			return mani, err
		}
		mani.Data = b
		if err != nil {
			logrus.Error(err)
			return mani, err
		}
	}
	mani.Upstream = types.Upstream{
		Reg:  up.Reg,
		Repo: up.Repo,
		Tag:  up.Tag,
	}
	return
}

func (mag Magazine) NeoInsertManifest(ctx context.Context, m types.Manifest) (maniID int64, err error) {
	if m.Digest == m.Config.Digest {
		logrus.Fatalf("NeoInsertManifest: Manifest.Digest and Manifest.Config.Digest are equal - something went wrong\n Digest:    %s\n ConfigDig: %s ", m.Digest, m.Config.Digest)
	}
	maniID, success, err := mag.Neo.GetManifest(m)
	if err != nil {
		return
	}
	if !success {
		maniID, err = mag.Neo.CreateManifest(m)
		if err != nil {
			return
		}
	}
	return
}

func (mag Magazine) NeoLinkManifestToUpstream(ctx context.Context, maniID, upID int64) (err error) {
	return mag.Neo.LinkManifestToUpstream(ctx, maniID, upID)
}

func (mag Magazine) NeoLinkManifestToConfig(ctx context.Context, maniID, cfgID int64) (err error) {
	return mag.Neo.LinkManifestToConfig(ctx, maniID, cfgID)
}

func (mag Magazine) NeoLinkManifestToLayer(ctx context.Context, maniID, lID int64, idx int) (err error) {
	return mag.Neo.LinkManifestToLayer(ctx, maniID, lID, idx)
}

func (mag Magazine) NeoLinkManifestToPlatform(ctx context.Context, maniID, platID int64) (err error) {
	return mag.Neo.LinkManifestToPlatform(ctx, maniID, platID)
}

func (mag Magazine) Manifest_GetByHTTPRequest(r *http.Request) (m types.Manifest, found bool, err error) {
	req := types.NewRequest(r)
	if req.Profile == "" {
		return
	}
	msg := fmt.Sprintf("%-5s ManifestHandler(%s, ", r.Method, req.Repo)
	if req.Tag == "" {
		msg += fmt.Sprintf("%s)", req.Digest)
	} else {
		msg += fmt.Sprintf("%s)", req.Tag)
	}
	if req.Profile != "generic" {
		msg += fmt.Sprintf(" - profile: %s", req.Profile)
	}
	logrus.Info(msg)
	m, found, err = mag.Manifest_GetByRequest(req)
	if err != nil {
		return
	}
	return
}

func (mag Magazine) Manifest_DeleteByHTTPRequest(r *http.Request) (err error) {
	req := types.NewRequest(r)
	msg := fmt.Sprintf("%-5s DeleteManifestHandler(%s, ", r.Method, req.Repo)
	if req.Tag == "" {
		msg += fmt.Sprintf("%s)", req.Digest)
	} else {
		msg += fmt.Sprintf("%s)", req.Tag)
	}
	logrus.Warn(msg)
	err = mag.RemoveManifestByRequest(req)
	if err != nil {
		return
	}
	return
}

func (mag Magazine) Manifest_GetByRequest(req types.Request) (m types.Manifest, found bool, err error) {
	ctx := context.Background()
	// Fetch Manifest from SQLite
	m, found, err = mag.SQlite.Manifest.GetTypesManifestByRequest(req)
	if err != nil {
		err = errors.Wrap(err, "failed to get Manifest from SQLite")
		return
	}
	if !found {
		return
	}
	m, err = mag.Minio.ManifestGetObj(ctx, m.Digest)
	return
}

func (mag Magazine) StoreManifestUpload(req types.Request, buf []byte) (mani types.Manifest, err error) {
	ctx := context.Background()
	ociMani := types.OCIManifest{
		MediaType: req.ContentType,
	}
	err = json.Unmarshal(buf, &ociMani)
	if err != nil {
		err = errors.Wrapf(err, "failed to unmarshal OCI Manifest '%s'", string(buf))
		return mani, err
	}
	dig := digest.FromBytes(buf)
	req.Digest = dig.String()
	if req.Digest != "" && req.Digest != dig.String() {
		err = fmt.Errorf("digest mismatch: %s != %s", req.Digest, dig.String())
		return
	}
	err = mag.Minio.ManifestsPutObj(ctx, dig.String(), buf)
	if err != nil {
		logrus.Error(err)
		return
	}
	mani = types.Manifest{
		Digest:    dig.String(),
		MediaType: ociMani.MediaType,
		Data:      buf,
		Size:      int64(len(buf)),
	}
	// Fetch Config
	mani.Config, err = mag.SQlite.Config.GetCreateByRequest(ociMani)
	if err != nil {
		err = errors.Wrap(err, "failed to Config.GetCreateByRequest")
		return
	}
	/// Extract Platform
	cfg, found, err := mag.Minio.GetConfig(ctx, mani.Config.Digest)
	if err != nil {
		err = errors.Wrap(err, "failed to get Config from Minio")
		return
	}
	if !found {
		err = errors.New("config not found")
		return
	}
	if cfg.Architecture == "" {
		logrus.Infof("cfg: %v", cfg)
		logrus.Fatal("architecture should not be empty")
	}
	plat := types.Platform{
		Arch:    cfg.Architecture,
		OS:      cfg.OS,
		Variant: cfg.Variant,
	}
	err = mag.SQlite.Platform.Upsert(&plat)
	if err != nil {
		err = errors.Wrap(err, "failed to SQlite.Platform.Upsert")
		return
	}
	mani.Platform = plat
	err = mag.SQlite.UpdatePlatformFilter(plat)
	if err != nil {
		err = errors.Wrap(err, "failed to SQlite.UpdatePlatformFilter")
		return
	}
	req.Platform = plat.GetTag()
	// Fetch/Create Upstream
	mani.Upstream, err = mag.SQlite.Upstream.GetCreateByRequest(req)
	if err != nil {
		err = errors.Wrap(err, "failed to Upstream.GetCreateByRequest")
		return
	}

	// INNER JOIN platform_filter ON platforms.id = platform_filter.platform_id

	// Add Manifest to SQLite
	//// The Manifest is suffixed with the profile to make it unique
	err = mag.SQlite.Manifest.GetInsert(&mani)
	if err != nil {
		err = errors.Wrap(err, "failed to SQlite.Manifest.Insert")
		return
	}

	// Tag && Collection & ManifestList
	/*
		tag, err := mag.SQlite.CollectionTag.GetCreateByRequest(req)
		if err != nil {
			err = errors.Wrap(err, "failed to SQlite.CollectionTag.GetCreateByRequest")
			return
		}
		_ = tag
	*/
	//// Fetch current collection or create new one
	col, err := mag.SQlite.GetInsertCollectionByRepoTag(req.Repo, req.Tag)
	if err != nil {
		err = errors.Wrap(err, "failed to SQlite.GetInsertCollectionByRepo")
		return
	}
	//logrus.Fatalf("col.Tag: %v", col.Tag)
	// Augment ManifestList to collection
	err = mag.AddMLtoCollection(&col)
	if err != nil {
		err = errors.Wrap(err, "failed to AddMLtoCollection")
		return
	}
	// TODO: Add Manifest to ManifestList (which is linked to Collection)
	for _, ml := range col.ManifestLists {
		b := ml.ToOCIBytes()
		logrus.Warnf("ManifestList: %s", string(b))
	}
	ml := types.ManifestList{
		SchemaVersion: 2,
		MediaType:     "application/vnd.docker.distribution.manifest.list.v2+json",
		Manifests: []types.Manifest{
			mani,
		},
	}
	ml.Digest = digest.FromBytes(ml.ToOCIBytes()).String()
	// Add ML to SQLite
	err = mag.SQlite.InsertManifestList(&ml)
	if err != nil {
		err = errors.Wrap(err, "failed to SQlite.ManifestList.Insert")
		return
	}
	col.ManifestLists = append(col.ManifestLists, ml)
	// Update Collection
	err = mag.SQlite.Collection.InsertXTable(col.SqlID, col.Tag.SqlID, ml.SqlID)
	if err != nil {
		err = errors.Wrap(err, "failed to SQlite.Collection.InsertXTable")
		return
	}

	return
}

func (mag Magazine) RemoveManifestByRequest(req types.Request) (err error) {
	ctx := context.Background()
	m := types.Manifest{
		Upstream: types.Upstream{
			Repo: req.Repo,
			Tag:  req.Tag,
		},
		Digest: req.Digest,
	}
	mlIDs, maniDigest, err := mag.SQlite.Manifest.Remove(m)
	if err != nil {
		err = errors.Wrap(err, "failed to SQlite.Manifest.Remove")
		return
	}
	// Remove Manifest Blob
	err = mag.Minio.ManifestRemoveObj(ctx, maniDigest)
	if err != nil {
		err = errors.Wrap(err, "failed to Minio.ManifestRemoveObj")
		return
	}
	for _, mlID := range mlIDs {
		err = mag.SQlite.Collection.ScrubCollectionsByManifestListID(mlID)
		if err != nil {
			err = errors.Wrap(err, "failed to SQlite.Collection.ScrubCollectionsByManifestListID")
			return
		}
	}
	return
}
