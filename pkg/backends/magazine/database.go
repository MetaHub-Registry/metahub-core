package magazine

import (
	"context"

	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	yamlCfg "github.com/metahub-registry/metahub-core/pkg/yaml/config"
)

type Database interface {
	/******** Graph *****/
	QueryOne(q string) (res types.Result, found bool, err error)
	Query(q string) (res []types.Result, err error)
	Execute(q string) (res types.Result, err error)

	/*******  Misc Tasks ********/
	Setup(ctx context.Context) error                                                         // Setup sets up the database backend
	Backup(fname string) error                                                               // Takes a snapshot of the DB
	BootstrapTest(bs utils.Bootstrap) (br utils.BootstrapResult, err error)                  // BootstrapTest sets up the database backend for testing
	BootstrapTestWithBR(b string, bs utils.Bootstrap, br *utils.BootstrapResult) (err error) // BootstrapTest sets up the database backend for testing
	EmptyData(emptyAll bool) error
	EmptyData_KeepUpstream() error
	TeardownTest(bs utils.Bootstrap)
	/*******  Platform ********/
	Platform_GetInsert(ctx context.Context, p *types.Platform) error
	/******* Upstream ********/
	Upstream_GetInsert_ByRequest(ctx context.Context, req types.Request) (up types.Upstream, found bool, err error)
	Upstream_Get_ByRequest(ctx context.Context, req types.Request) (up types.Upstream, found bool, err error)

	/********* Annotation/Labels *********/
	Config_AddLabels(ctx context.Context, cfgID int64, a *types.Annotations) (err error)
	Manifest_AddAnnotations(ctx context.Context, maniID int64, as *types.Annotations) (err error)
	Manifest_AddAnnotation(ctx context.Context, maniID int64, a *types.Annotation) (err error)

	/********* Layer ***********/
	LayerGetByDigest(dig string) (l types.Layer, found bool, err error)
	LayerDeleteByDigest(dig string) error
	Layer_PlainInsert(ctx context.Context, l types.Layer) (lID int64, err error)
	Layer_DescriporGetInsert(ctx context.Context, desc v1.Descriptor) (layerID int64, err error)
	Layers_DeleteUnused(ctx context.Context) (err error)
	LayerExists(ctx context.Context, up upstream.UpstreamTriple) (mediaType string, found bool, err error)
	Layer_GetUpstreamRegistry(ctx context.Context, up upstream.UpstreamTriple) (registry string, err error)
	/********* Config **********/
	Config_GetByDigest(ctx context.Context, dig string) (cfg types.Config, found bool, err error)
	//Config_GetByDigestAndType(ctx context.Context, dig, mediaType string) (cfg types.Config, found bool, err error)
	ConfigDeleteByDigest(dig string) error
	Config_Insert(ctx context.Context, cfg types.Config) (cfgId int64, err error)
	Config_GetInsert_ByOCIManifest(ctx context.Context, ociMani types.OCIManifest, label string) (cfg types.Config, found bool, err error)
	Config_Update(ctx context.Context, cfgID int64, cfg types.Config) (err error)
	Config_AddLabel(ctx context.Context, cfgID int64, a types.Annotation) (err error)
	Configs_DeleteUnused(ctx context.Context) (err error)
	OCIv1Config_GetInsertDescriptor(ctx context.Context, desc v1.Descriptor) (cfg types.Config, found bool, err error)
	ConfigExists(ctx context.Context, up upstream.UpstreamTriple) (mediaType string, found bool, err error)
	/********* Manifest **********/
	Manifest_GetInsert(ctx context.Context, mani types.Manifest) (maniID int64, err error)
	Manifest_GetAll(ctx context.Context) (manis []types.Manifest, err error)
	Manifest_GetByLabel(ctx context.Context, colName string) (mani types.Manifest, err error)
	Manifest_GetByUpstreamDigest(ctx context.Context, dig string) (mani types.Manifest, found bool, err error)
	Manifest_GetInsertWithRequest(ctx context.Context, m types.Manifest, req types.Request) (maniID int64, err error)
	Manifest_GetInsertWithProfile(ctx context.Context, upID int64, prfName string) (maniID int64, err error)
	Manifest_Get_CreateWithProfile(ctx context.Context, upID int64, prfName string) (man types.Manifest, err error)
	Manifest_GetByID(ctx context.Context, maniID int64) (mani types.Manifest, found bool, err error)
	Manifest_GetByID_WithProfile(ctx context.Context, maniID, prfID int64) (mani types.Manifest, found bool, err error)
	ManifestExists(ctx context.Context, up upstream.UpstreamTriple) (mediaType string, found bool, err error)
	/// ManifestData
	ManifestData_GetID_ByDigest(ctx context.Context, dig string) (maniID int64, found bool, err error)
	ManifestData_Insert(ctx context.Context, mani types.ManifestData) (maniID int64, err error)
	ManifestData_GetTypes_ByRequest(ctx context.Context, req types.Request) (res types.ManifestData, found bool, err error)
	ManifestData_GetInsert(ctx context.Context, mani *types.ManifestData) (err error)
	ManifestData_Remove(ctx context.Context, req types.Request) (found bool, err error)
	ManifestData_GetByCollectionTag(ctx context.Context, colID, colTagID int64) (manis []types.ManifestData, err error)
	ManifestData_GetAll(ctx context.Context) (manis []types.ManifestData, err error)
	ManifestData_LinkToOCIv1Config(ctx context.Context, maniID int64, cfg types.Config) (err error)
	ManifestData_LinkToOCIv1Layer(ctx context.Context, maniID int64, layer types.LayerDesc) (err error)
	ManifestData_AddAnnotations(ctx context.Context, maniID int64, a *types.Annotations) (err error)
	ManifestData_LinkToAnnotation(ctx context.Context, maniID, annotationID int64) (err error)
	ManifestData_GetByRule(ctx context.Context, rule types.Rule) (mds []types.ManifestData, err error)
	ManifestData_CheckProfile(ctx context.Context, md types.ManifestData, prf types.Profile) (ok bool, err error)
	ManifestData_GetPushed_ByProfile(ctx context.Context, up types.Upstream, prf types.Profile) (m types.ManifestData, found bool, err error)
	//ManifestData_GetPulled_ByProfile(ctx context.Context, up types.Upstream, prf types.Profile) (m types.ManifestData, found bool, err error)
	ManifestData_Delete(ctx context.Context, mani types.ManifestData) (err error)
	/******* Manifest OCI Transformation ********/
	ManifestData_GetID_ByMediaType(ctx context.Context, msID int64, mediaType string) (maniID int64, found bool, err error)
	ManifestData_GetRelID_ByMediaType(ctx context.Context, msID int64, mediaType, relType string) (maniID int64, found bool, err error)
	ManifestData_Get_ByProfileMediaType(ctx context.Context, mID, prfID int64, mediaType string) (maniID int64, found bool, err error)
	/******* ManifestV1 ********/
	ManifestData_V1_GetInsert(ctx context.Context, maniSiID int64, mani v1.Descriptor) (maniID int64, found bool, err error)
	//Manifest_Update(ctx context.Context, maniID int64, mani types.ManifestData) (err error)
	/********* ManifestList **********/
	ManifestList_Insert(ctx context.Context, ml *types.ManifestList) (err error)
	ManifestList_Get_ByRequest(ctx context.Context, req types.Request) (ml types.ManifestList, found bool, err error)
	InsertManifestList(dig digest.Digest, mt string, size int) (mlID int64, err error)
	/********* Collection **********/
	CollectionGetAll(ctx context.Context) (colls []types.Collection, err error)
	Collection_GetID_ByYaml(ctx context.Context, ycol yamlCfg.ManifestCollection) (colID, colTagID int64, found bool, err error)
	Collection_Create_ByYaml(ctx context.Context, ycol yamlCfg.ManifestCollection) (colID, colTagID int64, err error)
	//Collection_GetInsert_ByRequest(req types.Request) (col types.Collection, err error)
	CollectionManifest_ByRequest(req types.Request, mani types.ManifestData) (err error)
	Collection_GetTagsListByRepo(repo string) (tags types.TagResponse, err error)
	Collection_GetCreate_ByRepoTag(ctx context.Context, repo, tag string) (c types.Collection, found bool, err error)
	CollectionListing_Get_ByName(ctx context.Context, repoName, colTagName string) (col types.CollectionListing, err error)
	CollectionListing_Get_ByID(ctx context.Context, colID, colTagID int64) (col types.CollectionListing, err error)
	CollectionListing_Get_ByRepoLike(ctx context.Context, repo string) (cols []types.CollectionListing, err error)
	Collection_Remove(ctx context.Context, c types.Collection) (found bool, err error)
	CollectionExists(ctx context.Context, up upstream.UpstreamTriple) (found bool, err error)
	/**** CollectionTag ****/
	CollectionTag_CountRelationships(ctID int64) (count int64, err error)

	/********* Links **********/
	LinkCollectionTagToManifest(ctx context.Context, colTagID, maniID int64) error       // Connects a collection tag to a manifest to indicate it is servered by that collection
	LinkCollectionToManifest(ctx context.Context, colID, maniID int64) error             // Connects a collection to a manifest to indicate it is servered by that collection
	Manifest_LinkToUpstream(ctx context.Context, maniID, upID int64) error               // Connects a manifest to an upstream
	ManifestData_LinkToConfig(ctx context.Context, maniID int64, cfg types.Config) error //
	LinkManifestToPlatform(ctx context.Context, maniID, platID int64) error              //
	LinkManifestListToCollection(mlID, colID int64) (err error)
	LinkManifestListToCollectionTag(mlID, colTagID int64) (err error)
	LinkManifestListToProfile(mlID, prfID int64) (err error)
	LinkManifestListToManifestData(mlID, mdID int64) (err error)
	ManifestData_LinkToLayer(ctx context.Context, maniID int64, layer types.Layer, idx int) error //
	LinkManifestToManifestData(ctx context.Context, maniID, maniDataID int64, props ...string) error
	LinkManifestDataToProfile(ctx context.Context, mdID, prfID int64, relation string) error
	/****** BlobUpload ******/
	BlobUpload_Create_ByOCI(ctx context.Context, i *types.OCIBlobUpload) (err error)
	BlobUpload_Update_ByUUID(ctx context.Context, i *types.OCIBlobUpload) (err error)
	BlobUpload_GetOCI_ByUUID(ctx context.Context, uuid string) (bu types.OCIBlobUpload, found bool, err error)
	BlobUpload_Complete(ctx context.Context, i *types.OCIBlobUpload) (err error)
	BlobUpload_DeleteUnused(ctx context.Context) (err error)
	/**** BlobChunks ****/
	UploadChunk_Get_ByUUID(ctx context.Context, buID int64) (bcs []types.BlobChunks, err error)
	UploadChunk_Create(ctx context.Context, buID, startByte, lenBytes int64) (bc types.BlobChunks, err error)

	/****** Profiles ********/
	GetProfiles(ctx context.Context) (ps []types.Profile, err error)
	Profile_GetInsert(ctx context.Context, p *types.Profile) (err error)
	Profile_Get_ByNameTag(ctx context.Context, name, tag string) (p types.Profile, found bool, err error)
	Profiles_Get_ByRegexLabel(ctx context.Context, labelRegex string) (ps []types.Profile, err error)
	Profile_Get_ByLabel(ctx context.Context, label string) (p types.Profile, found bool, err error)
	Profile_Get_ByID(ctx context.Context, prfID int64) (p types.Profile, found bool, err error)
	ProfileAuthorizedPushHandler(ctx context.Context, req types.Request) (prf types.Profile, err error)
	Profile_GetUploadedBy_ViaManifestDataID(ctx context.Context, maniID int64) (p types.Profile, found bool, err error)
	Profile_GetPulledBy_ViaManifestDataID(ctx context.Context, maniID int64) (p types.Profile, found bool, err error)
	Profile_UnwindRules(ctx context.Context, prfID, maniID int64) (err error)
	Profile_GetAnnoEqRules(ctx context.Context, prfID int64) (rs []types.Rule, err error)
	Profile_ManifestType(ctx context.Context, maniID int64) (relType string, found bool, err error)
	Profile_FromManifestID(ctx context.Context, maniID int64) (prfs []types.Profile, err error)
	// Rules
	Rule_EvalAnnotationRule(ctx context.Context, r types.Rule) (err error)
	Rule_Get(ctx context.Context, r types.Rule) (ruleID int64, found bool, err error)
	Rule_GetCreate(ctx context.Context, r types.Rule) (ruleID int64, err error)
	Rules_GetAll(ctx context.Context) (rules []types.Rule, err error)
	Rule_EvalAll(ctx context.Context) (err error)
	Rule_GetByLabel(ctx context.Context, label string) (rule types.Rule, found bool, err error)
	/****** CollectionTagProfilePair ********/
	CollectionTagProfilePair_GetAll(ctx context.Context) (ctpps []types.CollectionTagProfilePair, err error)
	CollectionTagProfilePair_GetRegexFiltered(ctx context.Context, repoRegex, tagRegex, prfRegex string) (ctpps []types.CollectionTagProfilePair, err error)

	/**** TEMP ****/
	EvalUploadProfile(ctx context.Context, mani types.Manifest, origMani types.ManifestData) (err error)
}
