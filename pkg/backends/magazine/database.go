package magazine

import (
	"context"

	"github.com/docker/distribution/manifest/manifestlist"
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
	Setup(ctx context.Context) error                                                                              // Setup sets up the database backend
	Backup(fname string) error                                                                                    // Takes a snapshot of the DB
	BootstrapTest(ctx context.Context, bs utils.Bootstrap) (br utils.BootstrapResult, err error)                  // BootstrapTest sets up the database backend for testing
	BootstrapTestWithBR(ctx context.Context, b string, bs utils.Bootstrap, br *utils.BootstrapResult) (err error) // BootstrapTest sets up the database backend for testing
	EmptyData(emptyAll bool) error
	EmptyData_KeepUpstream() error
	EmptyDataKeepUpstreamConfigLayer() error
	TeardownTest(bs utils.Bootstrap, br utils.BootstrapResult)
	/*******  Platform ********/
	Platform_GetInsert(ctx context.Context, p *types.Platform) error
	/******* Upstream ********/
	Upstream_GetInsert_ByRequest(ctx context.Context, req types.Request) (up types.Upstream, found bool, err error)
	Upstream_Get_ByRequest(ctx context.Context, req types.Request) (up types.Upstream, found bool, err error)
	UpstreamGet(ctx context.Context, up types.Upstream) (upID int64, success bool, err error)
	UpstreamGetByID(ctx context.Context, upID int64) (up types.Upstream, found bool, err error)
	UpstreamGetByLayerDigest(ctx context.Context, dig string) (up types.Upstream, found bool, err error)
	/********* Annotation/Labels *********/
	ConfigAddLabels(ctx context.Context, cfgID int64, a *types.Annotations) (err error)
	Manifest_AddAnnotations(ctx context.Context, maniID int64, as *types.Annotations) (err error)
	Manifest_AddAnnotation(ctx context.Context, maniID int64, a *types.Annotation) (err error)
	/********* Blob **********/
	BlobExists(ctx context.Context, up upstream.UpstreamTriple) (found bool, err error)
	BlobGetByDigest(ctx context.Context, dig string) (b *types.Blob, found bool, err error)
	BlobDeleteByDigest(ctx context.Context, dig string) error
	/********* Layer ***********/
	LayerGetByDigest(ctx context.Context, dig string) (l types.Layer, found bool, err error)
	LayerDeleteByDigest(ctx context.Context, dig string) error
	Layer_PlainInsert(ctx context.Context, l types.Layer) (lID int64, err error)
	LayerDescriptorGetInsert(ctx context.Context, desc v1.Descriptor) (layerID int64, err error)
	Layers_DeleteUnused(ctx context.Context) (err error)
	LayerExists(ctx context.Context, up upstream.UpstreamTriple) (found bool, err error)
	LayerGetUpstream(ctx context.Context, lID int64) (up types.Upstream, found bool, err error)
	GetLayersForManifestDataID(ctx context.Context, maniDataID int64) (layers []types.Layer, err error)
	LayerGetByLayerAndManifestDataID(ctx context.Context, layerID, mdID int64) (lyr types.Layer, found bool, err error)
	/********* Config **********/
	ConfigGetByDigest(ctx context.Context, dig string) (cfg types.Config, found bool, err error)
	ConfigGetByID(ctx context.Context, cfgID int64) (cfg types.Config, found bool, err error)

	//ConfigGetByDigestAndType(ctx context.Context, dig, mediaType string) (cfg types.Config, found bool, err error)
	ConfigDeleteByDigest(ctx context.Context, dig string) error
	ConfigInsert(ctx context.Context, cfg types.Config) (cfgId int64, err error)
	ConfigGetInsertByOCIManifest(ctx context.Context, ociMani types.OCIManifest, label string) (cfg types.Config, found bool, err error)
	ConfigUpdate(ctx context.Context, cfgID int64, cfg types.Config) (err error)
	ConfigAddLabel(ctx context.Context, cfgID int64, a types.Annotation) (err error)
	ConfigsDeleteUnused(ctx context.Context) (err error)
	ConfigGetByManiDataID(ctx context.Context, mdID int64) (cfg types.Config, found bool, err error)

	OCIv1ConfigGetInsertDescriptor(ctx context.Context, desc v1.Descriptor) (cfg types.Config, found bool, err error)
	ConfigExists(ctx context.Context, up upstream.UpstreamTriple) (mediaType string, found bool, err error)
	/********* Manifest **********/
	Manifest_Get(ctx context.Context, upID int64, suffix ...string) (msID int64, found bool, err error)
	ManifestGetInsert(ctx context.Context, up types.Upstream, as ...types.Annotation) (maniID int64, err error)
	Manifest_GetAll(ctx context.Context) (manis []types.Manifest, err error)
	ManifestGetByLabel(ctx context.Context, colName string) (mani types.Manifest, found bool, err error)
	Manifest_GetByUpstreamDigest(ctx context.Context, dig string) (mani types.Manifest, found bool, err error)
	Manifest_GetInsertWithRequest(ctx context.Context, m types.Manifest, req types.Request) (maniID int64, err error)
	Manifest_GetInsert_WithDesc(ctx context.Context, up types.Upstream, maniIn manifestlist.ManifestDescriptor) (maniID int64, err error)
	Manifest_Get_CreateWithProfile(ctx context.Context, upID int64, prfName string) (man types.Manifest, err error)
	Manifest_Get_ByRepoTag(ctx context.Context, repo, tag string) (mani types.Manifest, found bool, err error)
	ManifestGetByID(ctx context.Context, maniID int64) (mani types.Manifest, found bool, err error)
	ManifestGetByIDWithProfile(ctx context.Context, maniID, prfID int64) (mani types.Manifest, found bool, err error)
	ManifestExists(ctx context.Context, up upstream.UpstreamTriple) (mediaType string, found bool, err error)
	ManifestExists_ForTagProfile(ctx context.Context, up upstream.UpstreamTriple) (mediaType string, found bool, err error)
	/// ManifestData
	ManifestDataInsert(ctx context.Context, maniData *types.ManifestData) (err error)
	ManifestDataGetPulledByRepoTag(ctx context.Context, repo, tag string) (maniData types.ManifestData, found bool, err error)
	ManifestData_GetID_ByDigest(ctx context.Context, dig string) (maniID int64, found bool, err error)
	ManifestData_Insert(ctx context.Context, mani types.ManifestData) (maniID int64, err error)
	ManifestDataGetTypesByRequest(ctx context.Context, req types.Request) (res types.ManifestData, found bool, err error)
	ManifestData_GetInsert(ctx context.Context, mani *types.ManifestData) (err error)
	ManifestDataRemove(ctx context.Context, req types.Request) (found bool, err error)
	ManifestData_GetByCollectionTag(ctx context.Context, colID, colTagID int64) (manis []types.ManifestData, err error)
	ManifestData_GetAll(ctx context.Context) (manis []types.ManifestData, err error)
	ManifestDataGetByID(ctx context.Context, mdID int64) (m types.ManifestData, found bool, err error)
	ManifestData_LinkToOCIv1Config(ctx context.Context, maniID int64, cfg types.Config) (err error)
	ManifestDataLinkToOCIv1Layer(ctx context.Context, maniID int64, layer types.LayerDesc) (err error)
	ManifestData_AddAnnotations(ctx context.Context, maniID int64, a *types.Annotations) (err error)
	ManifestData_LinkToAnnotation(ctx context.Context, maniID, annotationID int64) (err error)
	ManifestData_GetByRule(ctx context.Context, rule types.Rule) (mds []types.ManifestData, err error)
	ManifestDataCheckProfile(ctx context.Context, md types.ManifestData, prf types.Profile) (ok bool, err error)
	ManifestData_GetPushed_ByProfile(ctx context.Context, up types.Upstream, prf types.Profile) (m types.ManifestData, found bool, err error)
	ManifestDataDelete(ctx context.Context, mani types.ManifestData) (err error)
	ManifestDataGetByDigest(ctx context.Context, dig string) (m types.ManifestData, found bool, err error)
	ManifestDataLinkToUpstream(ctx context.Context, maniID, upID int64) (err error)
	////// OS

	ManifestGetOS(ctx context.Context, maniDataID int64) (os types.OS, found bool, err error)
	ManifestUpsertOS(ctx context.Context, maniDataID int64, os types.OS) (err error)
	/******* Manifest OCI Transformation ********/
	ManifestDataGetIDByMediaType(ctx context.Context, msID int64, mediaType string) (maniID int64, found bool, err error)
	ManifestDataGetRelIDByMediaType(ctx context.Context, msID int64, mediaType []string, relType string) (maniID int64, found bool, err error)
	ManifestDataGetByProfileMediaType(ctx context.Context, mID, prfID int64, mediaTypes []string) (maniID int64, found bool, err error)
	/******* ManifestV1 ********/
	ManifestDataV1GetInsert(ctx context.Context, maniSiID int64, mani v1.Descriptor) (maniID int64, found bool, err error)
	ManifestDataUpdateDesc(ctx context.Context, maniDataID int64, maniDataV1 v1.Descriptor) (err error)
	//Manifest_Update(ctx context.Context, maniID int64, mani types.ManifestData) (err error)
	/********* ManifestList **********/
	ManifestList_Insert(ctx context.Context, ml *types.ManifestList) (err error)
	ManifestList_Get_ByRequest(ctx context.Context, req types.Request) (ml types.ManifestList, found bool, err error)
	InsertManifestList(dig digest.Digest, mt string, size int) (mlID int64, err error)
	/********* Collection **********/
	CollectionGetAll(ctx context.Context) (colls []types.Collection, err error)
	Collection_GetID_ByYaml(ctx context.Context, ycol yamlCfg.ManifestCollection) (colID, colTagID int64, found bool, err error)
	Collection_Create_ByYaml(ctx context.Context, ycol yamlCfg.ManifestCollection) (colID, colTagID int64, err error)
	CollectionGetCreateUpstreamTriple(ctx context.Context, up types.Upstream, maniIn manifestlist.ManifestDescriptor) (upID, maniID int64, prf types.Profile, err error)
	Collection_Get_ByRepoTag_woManifests(ctx context.Context, repo, tag string) (c types.Collection, found bool, err error)
	Collection_Get_ByRepoTag(ctx context.Context, repo, tag string) (c types.Collection, found bool, err error)
	CollectionGetCreateByRepoTag(ctx context.Context, repo, tag string) (c types.Collection, found bool, err error)
	CollectionListing_Get_ByName(ctx context.Context, repoName, colTagName string) (col types.CollectionListing, err error)
	CollectionListing_Get_ByID(ctx context.Context, colID, colTagID int64) (col types.CollectionListing, err error)
	CollectionListing_Get_ByRepoLike(ctx context.Context, repo string) (cols []types.CollectionListing, err error)
	CollectionRemove(ctx context.Context, c types.Collection) (found bool, err error)
	CollectionExists(ctx context.Context, up upstream.UpstreamTriple) (found bool, err error)
	/**** CollectionTag ****/
	CollectionTag_CountRelationships(ctID int64) (count int64, err error)
	CollectionGetTagsListByRepo(ctx context.Context, repo string) (tags types.TagResponse, err error)

	/********* Links **********/
	LinkCollectionTagToManifest(ctx context.Context, colTagID, maniID int64) error      // Connects a collection tag to a manifest to indicate it is servered by that collection
	LinkCollectionToManifest(ctx context.Context, colID, maniID int64) error            // Connects a collection to a manifest to indicate it is servered by that collection
	ManifestLinkToUpstream(ctx context.Context, maniID, upID int64) error               // Connects a manifest to an upstream
	ManifestDataLinkToConfig(ctx context.Context, maniID int64, cfg types.Config) error //
	LinkManifestToPlatform(ctx context.Context, maniID, platID int64) error             //
	LinkManifestListToCollection(mlID, colID int64) (err error)
	LinkManifestListToCollectionTag(mlID, colTagID int64) (err error)
	LinkManifestListToProfile(mlID, prfID int64) (err error)
	LinkManifestListToManifestData(mlID, mdID int64) (err error)
	ManifestDataLinkToLayer(ctx context.Context, maniDataID int64, layer types.Layer, idx int) error //
	ManifestDataLinkToProfile(ctx context.Context, maniDataID, profileID int64) error                //
	LinkManifestToManifestData(ctx context.Context, maniID, maniDataID int64, props ...string) error
	LinkManifestDataToProfile(ctx context.Context, mdID, prfID int64, relation string) error
	/****** BlobUpload ******/
	BlobUploadCreateByOCI(ctx context.Context, i *types.OCIBlobUpload) (err error)
	BlobUploadUpdateByUUID(ctx context.Context, i *types.OCIBlobUpload) (err error)
	BlobUploadGetOCIByUUID(ctx context.Context, uuid string) (bu types.OCIBlobUpload, found bool, err error)
	BlobUploadComplete(ctx context.Context, i *types.OCIBlobUpload) (err error)
	BlobUploadDeleteUnused(ctx context.Context) (err error)
	/**** BlobChunks ****/
	UploadChunk_Get_ByUUID(ctx context.Context, buID int64) (bcs []types.BlobChunk, err error)
	UploadChunkCreate(ctx context.Context, bu *types.OCIBlobUpload, startByte, lenBytes int64) (bc types.BlobChunk, err error)
	BlobUploadGetByBlobDigest(ctx context.Context, dig string) (bu types.OCIBlobUpload, found bool, err error)
	CreateBlobFromChunk(ctx context.Context, bu *types.OCIBlobUpload, chunk types.BlobChunk) (err error)
	/****** Profiles ********/
	GetProfiles(ctx context.Context) (ps []types.Profile, err error)
	Profile_GetInsert(ctx context.Context, p *types.Profile) (err error)
	Profile_Get_ByNameTag(ctx context.Context, name, tag string) (p types.Profile, found bool, err error)
	Profiles_Get_ByRegexLabel(ctx context.Context, labelRegex string) (ps []types.Profile, err error)
	Profile_Get_ByLabel(ctx context.Context, label string) (p types.Profile, found bool, err error)
	ProfileGetByID(ctx context.Context, prfID int64) (p types.Profile, found bool, err error)
	ProfileAuthorizedPushHandler(ctx context.Context, req types.Request) (prf types.Profile, err error)
	Profile_GetUploadedBy_ViaManifestDataID(ctx context.Context, maniID int64) (p types.Profile, found bool, err error)
	Profile_GetPulledBy_ViaManifestDataID(ctx context.Context, maniID int64) (p types.Profile, found bool, err error)
	ProfileUnwindRules(ctx context.Context, prfID, maniID int64) (err error)
	ProfileGetAnnoEqRules(ctx context.Context, prfID int64) (rs []types.Rule, err error)
	Profile_ManifestType(ctx context.Context, maniID int64) (relType string, found bool, err error)
	ProfileFromManifestID(ctx context.Context, maniID int64) (prfs []types.Profile, err error)
	ProfileFromYAML(ctx context.Context, p yamlCfg.Profile) (prf types.Profile, err error)
	// Rules
	Rule_EvalAnnotationRule(ctx context.Context, r types.Rule) (err error)
	Rule_Get(ctx context.Context, r types.Rule) (ruleID int64, found bool, err error)
	Rule_GetCreate(ctx context.Context, r types.Rule) (ruleID int64, err error)
	Rules_GetAll(ctx context.Context) (rules []types.Rule, err error)
	RuleEvalAll(ctx context.Context) (err error)
	RuleGetByLabel(ctx context.Context, label string) (rule types.Rule, found bool, err error)
	RulesGetByManifestID(ctx context.Context, maniID int64) (rules types.Rules, err error)
	RulesGetByManipuLayerID(ctx context.Context, mplID int64) (rules types.Rules, err error)
	/****** CollectionTagProfilePair ********/
	CollectionTagProfilePairGetAll(ctx context.Context) (ctpps []types.CollectionTagProfilePair, err error)
	CollectionTagProfilePairGetRegexFiltered(ctx context.Context, repoRegex, tagRegex, prfRegex string) (ctpps []types.CollectionTagProfilePair, err error)

	/**** TEMP ****/
	EvalUploadProfile(ctx context.Context, mani types.Manifest, origMani types.ManifestData) (err error)
	/***** BaseLayer *****/
	ManifestGetAllSystemAnnos(ctx context.Context, maniDataID int64) (as types.Annotations, err error)
	/***** Manipulations *****/
	ManipulationGetCreate(ctx context.Context, mp *types.Manipulation) (err error)
	ManipulationsGetByProfileID(ctx context.Context, prfID int64) (mps []types.Manipulation, err error)
	ManipuLayerFromConfigYAML(ctx context.Context, cmpl yamlCfg.ManipuLayer) (mpl types.ManipuLayer, err error)
	ManipulationFromYAML(ctx context.Context, cmp yamlCfg.Manipulation) (mp types.Manipulation, err error)
	ManinpulationCheckRules(ctx context.Context, prfID, maniID int64) (ok bool, err error)
	ManipuCmdsGetByManipulayerID(ctx context.Context, mplyrID int64) (mpcs []types.ManipuCmd, err error)
	ManipuEnvsGetByManipulayerID(ctx context.Context, mplyrID int64) (mpes []types.ManipuEnv, err error)
}
