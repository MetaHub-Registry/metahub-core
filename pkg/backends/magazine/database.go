package magazine

import (
	"context"

	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	yamlCfg "github.com/metahub-registry/metahub-core/pkg/yaml/config"
)

type Database interface {
	/*******  Misc Tasks ********/
	Setup() error                                                                            // Setup sets up the database backend
	Backup(fname string) error                                                               // Takes a snapshot of the DB
	BootstrapTest(bs utils.Bootstrap) (br utils.BootstrapResult, err error)                  // BootstrapTest sets up the database backend for testing
	BootstrapTestWithBR(b string, bs utils.Bootstrap, br *utils.BootstrapResult) (err error) // BootstrapTest sets up the database backend for testing
	EmptyData(emptyAll bool) error
	TeardownTest(bs utils.Bootstrap)
	/*******  Platform ********/
	Platform_GetInsert(p *types.Platform) error
	Filter_UpdatePlatform(ctx context.Context, plat types.Platform) error
	/******* Upstream ********/
	Upstream_GetInsert_ByRequest(req types.Request) (up types.Upstream, found bool, err error)

	/********* Annotation/Labels *********/
	Config_AddLabels(ctx context.Context, cfgID int64, a *types.Annotations) (err error)
	Manifest_AddAnnotations(ctx context.Context, maniID int64, a *types.Annotations) (err error)

	/********* Layer ***********/
	LayerGetByDigest(dig string) (l types.Layer, found bool, err error)
	LayerDeleteByDigest(dig string) error
	Layer_Insert(ctx context.Context, l types.Layer, up types.Upstream) (lID int64, err error)
	Layer_PlainInsert(ctx context.Context, l types.Layer) (lID int64, err error)

	/********* Config **********/
	ConfigGetByDigest(dig string) (cfg types.Config, found bool, err error)
	ConfigDeleteByDigest(dig string) error
	Config_Insert(ctx context.Context, cfg types.Config) (cfgId int64, err error)
	Config_GetInsert_ByOCIManifest(ociMani types.OCIManifest, label string) (cfg types.Config, found bool, err error)

	/********* Manifest **********/
	Manifest_GetID_ByDigest(dig string) (maniID int64, found bool, err error)
	Manifest_Insert(ctx context.Context, mani types.Manifest) (maniID int64, err error)
	Manifest_GetTypes_ByRequest(req types.Request) (res types.Manifest, found bool, err error)
	Manifest_GetInsert(ctx context.Context, mani *types.Manifest) (err error)
	Manifest_Remove(ctx context.Context, req types.Request) (found bool, err error)
	Manifest_GetManifests_ByManifestListID(mlID int64) (manis []types.Manifest, err error)
	Manifest_GetByCollectionTag(colID, colTagID int64) (manis []types.Manifest, err error)
	/********* Filter ***********/
	Filter_Insert(ctx context.Context, f types.Filter) (fID int64, err error)

	/********* ManifestList **********/
	ManifestList_Insert(ctx context.Context, ml *types.ManifestList) (err error)
	ComputeManifestlists() (err error)
	ComputeManifestlists_ForCollection(ctx context.Context, col types.Collection) (err error)
	ManifestList_Get_ByRequest(req types.Request) (ml types.ManifestList, found bool, err error)

	/********* Collection **********/
	CollectionGetAll() (colls []types.Collection, err error)
	Collection_GetID_ByYaml(ycol yamlCfg.ManifestCollection) (colID, colTagID int64, found bool, err error)
	Collection_Create_ByYaml(ycol yamlCfg.ManifestCollection) (colID, colTagID int64, err error)
	//Collection_GetInsert_ByRequest(req types.Request) (col types.Collection, err error)
	CollectionManifest_ByRequest(req types.Request, mani types.Manifest) (err error)
	Collection_GetTagsListByRepo(repo string) (tags types.TagResponse, err error)
	Collection_GetCreate_ByRepoTag(ctx context.Context, repo, tag string) (c types.Collection, found bool, err error)
	CollectionListing_Get_ByID(colID, colTagID int64) (col types.CollectionListing, err error)
	CollectionListing_Get_ByRepoLike(repo string) (cols []types.CollectionListing, err error)
	Collection_Remove(ctx context.Context, c types.Collection) (found bool, err error)
	/**** CollectionTag ****/
	CollectionTag_CountRelationships(ctID int64) (count int64, err error)

	/********* Links **********/
	LinkCollectionTagToManifest(ctx context.Context, colTagID, maniID int64) error           // Connects a collection tag to a manifest to indicate it is servered by that collection
	LinkCollectionToManifest(ctx context.Context, colID, maniID int64) error                 // Connects a collection to a manifest to indicate it is servered by that collection
	LinkManifestToUpstream(ctx context.Context, maniID, upID int64) error                    // Connects a manifest to an upstream
	LinkManifestToConfig(ctx context.Context, maniID, cfgID int64) error                     //
	LinkManifestToPlatform(ctx context.Context, maniID, platID int64) error                  //
	LinkManifestToLayer(ctx context.Context, maniID, layerID int64, idx int) error           //
	LinkPlatformIDToFilter(ctx context.Context, platID int64, fltr types.Filter) (err error) //
	LinkProfileToFilter(ctx context.Context, prfID, filterID int64) (err error)

	/****** BlobUpload ******/
	BlobUpload_Create_ByOCI(i *types.OCIBlobUpload) (err error)
	BlobUpload_Update_ByUUID(i *types.OCIBlobUpload) (err error)
	BlobUpload_GetOCI_ByUUID(uuid string) (bu types.OCIBlobUpload, found bool, err error)
	BlobUpload_Complete(i *types.OCIBlobUpload) (err error)

	/**** BlobChunks ****/
	UploadChunk_Get_ByUUID(buID int64) (bcs []types.BlobChunks, err error)
	UploadChunk_Create(buID, startByte, lenBytes int64) (bc types.BlobChunks, err error)

	/****** Profiles ********/
	Profile_GetInsert(ctx context.Context, p *types.Profile) (err error)
	Profile_Get_ByName(ctx context.Context, name string) (p types.Profile, found bool, err error)
}
