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
	EmptyDataKeepUpstream() error
	EmptyDataKeepUpstreamConfigLayer() error
	TeardownTest(bs utils.Bootstrap, br utils.BootstrapResult)
	/*******  Platform ********/
	PlatformGetInsert(ctx context.Context, p *types.Platform) error
	/******* Upstream ********/
	UpstreamGetInsertByRequest(ctx context.Context, req types.Request) (up types.Upstream, found bool, err error)
	UpstreamGetByRequest(ctx context.Context, req types.Request) (up types.Upstream, found bool, err error)
	UpstreamGet(ctx context.Context, up types.Upstream) (upID int64, success bool, err error)
	UpstreamGetByID(ctx context.Context, upID int64) (up types.Upstream, found bool, err error)
	UpstreamGetByLayerDigest(ctx context.Context, dig string) (up types.Upstream, found bool, err error)
	UpstreamGetByManifestID(ctx context.Context, maniDataID int64) (up types.Upstream, relType string, found bool, err error)
	/********* Annotation/Labels *********/
	ConfigAddLabels(ctx context.Context, cfgID int64, a *types.Annotations) (err error)
	// ManifestAddAnnotation / ManifestAddAnnotations were dead wrapper-era
	// helpers removed in Path Z Phase C3 step 2 — they were only ever called
	// by their own definitions. Annotation attachment to content-leaf
	// manifests goes through ManifestLinkToAnnotation / the per-push
	// annotation wiring in ManifestInsert instead.
	/********* Blob **********/
	BlobExists(ctx context.Context, up upstream.UpstreamTriple) (found bool, err error)
	BlobGetByDigest(ctx context.Context, dig string) (b *types.Blob, found bool, err error)
	BlobDeleteByDigest(ctx context.Context, dig string) error
	/********* Layer ***********/
	LayerGetByDigest(ctx context.Context, dig string) (l types.Layer, found bool, err error)
	LayerDeleteByDigest(ctx context.Context, dig string) error
	LayerPlainInsert(ctx context.Context, l types.Layer) (lID int64, err error)
	LayerDescriptorGetInsert(ctx context.Context, desc v1.Descriptor) (layerID int64, err error)
	LayersDeleteUnused(ctx context.Context) (err error)
	LayerExists(ctx context.Context, up upstream.UpstreamTriple) (found bool, err error)
	LayerGetUpstream(ctx context.Context, lID int64) (up types.Upstream, found bool, err error)
	GetLayersForManifestID(ctx context.Context, maniDataID int64) (layers []types.Layer, err error)
	LayerGetByLayerAndManifestID(ctx context.Context, layerID, mdID int64) (lyr types.Layer, found bool, err error)
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
	// ManifestGet was deleted in Path Z Phase C3 step 3. It walked the
	// wrapper :ManifestWrapper node looking for the one attached to a given
	// :Upstream, a query that is not meaningful in the slot-centred
	// model. Only callers were the wrapper-era tests TestManifestCR
	// and TestManifestWrapperGetInsert, both deleted in the same commit.
	ManifestWrapperGetInsert(ctx context.Context, up types.Upstream, as ...types.Annotation) (maniID int64, err error)
	ManifestWrapperGetAll(ctx context.Context) (manis []types.ManifestWrapper, err error)
	ManifestWrapperGetByLabel(ctx context.Context, colName string) (mani types.ManifestWrapper, found bool, err error)
	// ManifestGetByUpstreamDigest, ManifestWrapperGetInsertWithUpstream, and
	// ManifestGetCreateWithProfile were dead wrapper-era helpers with zero
	// external callers across metalib + metahub-api. Removed in Path Z
	// Phase C3 step 2. ManifestWrapperGetInsertWithUpstream in particular was a
	// no-op stub returning (0, nil) from before the wrapper refactor.
	ManifestWrapperGetInsertWithDesc(ctx context.Context, up types.Upstream, maniIn manifestlist.ManifestDescriptor) (maniID int64, err error)
	ManifestWrapperGetByRepoTag(ctx context.Context, repo, tag string) (mani types.ManifestWrapper, found bool, err error)
	ManifestWrapperGetByID(ctx context.Context, maniID int64) (mani types.ManifestWrapper, found bool, err error)
	ManifestWrapperGetByIDWithProfile(ctx context.Context, maniID, prfID int64) (mani types.ManifestWrapper, found bool, err error)
	// ManifestExists and ManifestExistsForTagProfile are Slot-agnostic
	// existence checks — they walk :Index and :Manifest directly
	// without consulting the legacy :ManifestWrapper wrapper. Renamed from
	// ManifestWrapperExists / ManifestWrapperExistsForTagProfile in Path Z Phase C3.
	ManifestExists(ctx context.Context, up upstream.UpstreamTriple) (mediaType string, found bool, err error)
	ManifestExistsForTagProfile(ctx context.Context, up upstream.UpstreamTriple) (mediaType string, found bool, err error)
	/// Manifest
	ManifestGetPulledByRepoTag(ctx context.Context, repo, tag string) (maniData types.Manifest, found bool, err error)
	ManifestGetIDByDigest(ctx context.Context, dig string) (maniID int64, found bool, err error)
	ManifestInsert(ctx context.Context, mani *types.Manifest) (maniID int64, err error)
	ManifestGetTypesByRequest(ctx context.Context, req types.Request) (res types.Manifest, found bool, err error)
	ManifestGetInsert(ctx context.Context, mani *types.Manifest) (err error)
	ManifestRemove(ctx context.Context, req types.Request) (found bool, err error)
	ManifestGetByCollectionTag(ctx context.Context, colID, colTagID int64) (manis []types.Manifest, err error)
	// ManifestGetRawByRepoTag returns every Manifest ever pushed to
	// (repo, tag), walking via the Upstream node's `(repo, tag)` compound
	// key so that multi-arch push history is fully visible even when the
	// :ManifestWrapper wrapper's outgoing edge is replaced on subsequent pushes.
	// This is the raw push-time lookup used by
	// ComputeManifestlistsForRepoTag — the returned rows only have ID,
	// Digest, MediaType, Size, Label and the relationship type populated.
	// An empty result is returned as `([], nil)`; callers treat empty as
	// "nothing to compute".
	ManifestGetRawByRepoTag(ctx context.Context, repo, tag string) (manis []types.Manifest, err error)
	ManifestGetAll(ctx context.Context) (manis []types.Manifest, err error)
	ManifestGetByID(ctx context.Context, mdID int64) (m types.Manifest, found bool, err error)
	ManifestLinkToOCIv1Config(ctx context.Context, maniID int64, cfg types.Config) (err error)
	ManifestLinkToOCIv1Layer(ctx context.Context, maniID int64, layer types.LayerDesc) (err error)
	ManifestAddAnnotations(ctx context.Context, maniID int64, a *types.Annotations) (err error)
	ManifestLinkToAnnotation(ctx context.Context, maniID, annotationID int64) (err error)
	// ManifestsMatchingRule is the umbrella rule-match getter that
	// unions the three per-flavour walks (annotation shortcut, platform
	// shortcut, direct anno-null edge) and returns every Manifest
	// matching the given Rule, deduplicated by Manifest ID. See the
	// rule-shortcut taxonomy in design decision D5. Path Z task 7.4 —
	// renamed from ManifestGetByRule.
	ManifestsMatchingRule(ctx context.Context, rule types.Rule) (mds []types.Manifest, err error)
	// Phase C3: ManifestGetByRule deprecated shim deleted. Use ManifestsMatchingRule.
	// AssertRuleShortcutInvariants runs the four cypher queries from
	// Path Z design decision D5 and returns a slice of human-readable
	// violation messages — one per way the rule evaluator has drifted
	// from the three-flavour shortcut taxonomy. An empty slice means
	// all four invariants hold. Path Z task 1.5.
	AssertRuleShortcutInvariants(ctx context.Context) (violations []string, err error)
	ManifestCheckProfile(ctx context.Context, md types.Manifest, prf types.Profile) (ok bool, err error)
	ManifestCheckProfileAnnotations(ctx context.Context, md types.Manifest, prf types.Profile) (ok bool, err error)
	ManifestGetPushedByProfile(ctx context.Context, up types.Upstream, prf types.Profile) (m types.Manifest, found bool, err error)
	ManifestDelete(ctx context.Context, mani types.Manifest) (err error)
	ManifestGetByDigest(ctx context.Context, dig string) (m types.Manifest, found bool, err error)
	ManifestLinkToUpstream(ctx context.Context, maniID, upID int64) (err error)
	////// OS

	ManifestWrapperGetOS(ctx context.Context, maniDataID int64) (os types.OS, found bool, err error)
	ManifestWrapperUpsertOS(ctx context.Context, maniDataID int64, os types.OS) (err error)
	/******* Manifest OCI Transformation ********/
	ManifestGetIDByMediaType(ctx context.Context, msID int64, mediaType string) (maniID int64, found bool, err error)
	ManifestGetRelIDByMediaType(ctx context.Context, msID int64, mediaType []string, relType string) (maniID int64, found bool, err error)
	ManifestGetByProfileMediaType(ctx context.Context, mID, prfID int64, mediaTypes []string) (maniID int64, found bool, err error)
	/******* ManifestV1 ********/
	ManifestV1GetInsert(ctx context.Context, maniSiID int64, mani v1.Descriptor) (maniID int64, found bool, err error)
	ManifestUpdateDesc(ctx context.Context, maniDataID int64, maniDataV1 v1.Descriptor) (err error)
	//Manifest_Update(ctx context.Context, maniID int64, mani types.Manifest) (err error)
	/********* Index **********/
	ManifestListInsert(ctx context.Context, ml *types.Index) (err error)
	IndexGetByRequest(ctx context.Context, req types.Request) (ml types.Index, found bool, err error)
	ManifestListGetByDigest(ctx context.Context, dig string) (ml *types.Index, err error)
	ManifestListAddAnnotation(ctx context.Context, mlID int64, a *types.Annotation) (err error)
	ManifestListLinkToAnnotation(ctx context.Context, mlID, annotationID int64) (err error)
	InsertManifestList(ctx context.Context, dig digest.Digest, mt string, size int) (mlID int64, err error)
	/********* Repo **********/
	CollectionGetAll(ctx context.Context) (colls []types.Repo, err error)
	CollectionGetIDByYaml(ctx context.Context, ycol yamlCfg.ManifestCollection) (colID, colTagID int64, found bool, err error)
	CollectionCreateByYaml(ctx context.Context, ycol yamlCfg.ManifestCollection) (colID, colTagID int64, err error)
	CollectionGetCreateUpstreamTriple(ctx context.Context, up types.Upstream, maniIn manifestlist.ManifestDescriptor) (upID, maniID int64, prf types.Profile, err error)
	CollectionGetByRepoTagWoManifests(ctx context.Context, repo, tag string) (c types.Repo, found bool, err error)
	CollectionGetByRepoTag(ctx context.Context, repo, tag string) (c types.Repo, found bool, err error)
	CollectionGetCreateByRepoTag(ctx context.Context, repo, tag string) (c types.Repo, found bool, err error)
	CollectionListingGetByName(ctx context.Context, repoName, colTagName string) (col types.CollectionListing, err error)
	CollectionListingGetByID(ctx context.Context, colID, colTagID int64) (col types.CollectionListing, err error)
	CollectionListingGetByRepoLike(ctx context.Context, repo string) (cols []types.CollectionListing, err error)
	CollectionRemove(ctx context.Context, c types.Repo) (found bool, err error)
	CollectionExists(ctx context.Context, up upstream.UpstreamTriple) (found bool, err error)
	/**** CollectionTag ****/
	CollectionTagCountRelationships(ctID int64) (count int64, err error)
	CollectionGetTagsListByRepo(ctx context.Context, repo string) (tags types.TagResponse, err error)

	/********* Links **********/
	LinkCollectionTagToManifest(ctx context.Context, colTagID, maniID int64) error // Connects a collection tag to a manifest to indicate it is servered by that collection
	LinkCollectionToManifest(ctx context.Context, colID, maniID int64) error       // Connects a collection to a manifest to indicate it is servered by that collection
	ManifestWrapperLinkToUpstream(ctx context.Context, maniID, upID int64) error   // Connects a manifest to an upstream
	ManifestLinkToConfig(ctx context.Context, maniID int64, cfg types.Config) error
	LinkManifestListToCollection(mlID, colID int64) (err error)
	LinkManifestListToCollectionTag(mlID, colTagID int64) (err error)
	LinkManifestListToProfile(mlID, prfID int64) (err error)
	LinkManifestListToManifest(ctx context.Context, mlID, mdID int64) (err error)
	ManifestLinkToLayer(ctx context.Context, maniDataID int64, layer types.Layer, idx int) error
	ManifestLinkToProfile(ctx context.Context, maniDataID, profileID int64) error
	LinkManifestToPlatform(ctx context.Context, maniDataID, platID int64) error
	LinkManifestWrapperToManifest(ctx context.Context, maniID, maniDataID int64, props ...string) error
	LinkManifestToProfile(ctx context.Context, mdID, prfID int64, relation string) error
	/****** BlobUpload ******/
	BlobUploadCreateByOCI(ctx context.Context, i *types.OCIBlobUpload) (err error)
	BlobUploadUpdateByUUID(ctx context.Context, i *types.OCIBlobUpload) (err error)
	BlobUploadGetOCIByUUID(ctx context.Context, uuid string) (bu types.OCIBlobUpload, found bool, err error)
	BlobUploadComplete(ctx context.Context, i *types.OCIBlobUpload) (err error)
	BlobUploadDeleteUnused(ctx context.Context) (err error)
	BlobUploadGetByLocation(ctx context.Context, location string) (bu types.OCIBlobUpload, found bool, err error)
	/**** BlobChunks ****/
	UploadChunkGetByUUID(ctx context.Context, buID int64) (bcs []types.BlobChunk, err error)
	UploadChunkGetByDigest(ctx context.Context, dig string) (bcs []types.BlobChunk, err error)
	UploadChunkGetByBU(ctx context.Context, bu types.OCIBlobUpload) (bcs []types.BlobChunk, err error)
	UploadChunkCreate(ctx context.Context, bu *types.OCIBlobUpload, startByte, lenBytes int64) (bc types.BlobChunk, err error)
	BlobUploadGetByBlobDigest(ctx context.Context, dig string) (bu types.OCIBlobUpload, found bool, err error)
	CreateBlobFromChunk(ctx context.Context, bu *types.OCIBlobUpload, chunk types.BlobChunk) (err error)
	// BlobUpdateSizeByDigest sets blob size from manifest descriptor when the
	// current size is negative (placeholder from chunked upload).
	BlobUpdateSizeByDigest(ctx context.Context, digest string, size int64) error
	/****** Profiles ********/
	GetProfiles(ctx context.Context) (ps []types.Profile, err error)
	ProfileGetInsert(ctx context.Context, p *types.Profile) (err error)
	ProfileGetByNameTag(ctx context.Context, name, tag string) (p types.Profile, found bool, err error)
	ProfilesGetByRegexLabel(ctx context.Context, labelRegex string) (ps []types.Profile, err error)
	ProfileGetByLabel(ctx context.Context, label string) (p types.Profile, found bool, err error)
	ProfileGetByID(ctx context.Context, prfID int64) (p types.Profile, found bool, err error)
	ProfileAuthorizedPushHandler(ctx context.Context, req types.Request) (prf types.Profile, err error)
	ProfileGetUploadedByViaManifestID(ctx context.Context, maniID int64) (p types.Profile, found bool, err error)
	ProfileGetPulledByViaManifestID(ctx context.Context, maniID int64) (p types.Profile, found bool, err error)
	ProfileUnwindRules(ctx context.Context, prfID, maniID int64) (err error)
	ProfileGetAnnoEqRules(ctx context.Context, prfID int64) (rs []types.Rule, err error)
	// ProfileInjectDeterministicAnnotations links every deterministic
	// ("anno eq") rule of the given profile as an annotation on the given
	// Manifest. Non-deterministic rules (regex, ne, null, platform rules,
	// etc.) are silently skipped; the returned counts let the caller observe
	// how many rules were injected vs skipped. It is the push-time cousin of
	// ProfileUnwindRules used by Magazine.ComputeManifestlistsForRepoTag.
	ProfileInjectDeterministicAnnotations(ctx context.Context, prfID, maniDataID int64) (injected int, skipped int, err error)
	ProfileManifestType(ctx context.Context, maniID int64) (relType string, found bool, err error)
	ProfileFromManifestID(ctx context.Context, maniID int64) (prfs []types.Profile, err error)
	ProfileFromManifestWrapperID(ctx context.Context, maniID int64) (prfs []types.Profile, err error)
	ProfileFromYAML(ctx context.Context, p yamlCfg.Profile) (prf types.Profile, err error)
	// Rules
	RuleEvalAnnotationRule(ctx context.Context, r types.Rule) (err error)
	RuleGet(ctx context.Context, r types.Rule) (ruleID int64, found bool, err error)
	RuleGetCreate(ctx context.Context, r types.Rule) (ruleID int64, err error)
	RulesGetAll(ctx context.Context) (rules []types.Rule, err error)
	RuleEvalAll(ctx context.Context) (err error)
	RuleGetByLabel(ctx context.Context, label string) (rule types.Rule, found bool, err error)
	RulesGetByManifestID(ctx context.Context, maniID int64) (rules types.Rules, err error)
	RulesGetByManipuLayerID(ctx context.Context, mplID int64) (rules types.Rules, err error)
	/****** CollectionTagProfilePair ********/
	CollectionTagProfilePairGetAll(ctx context.Context) (ctpps []types.CollectionTagProfilePair, err error)
	CollectionTagProfilePairGetRegexFiltered(ctx context.Context, repoRegex, tagRegex, prfRegex string) (ctpps []types.CollectionTagProfilePair, err error)

	/**** TEMP ****/
	EvalUploadProfile(ctx context.Context, mani types.ManifestWrapper, origMani types.Manifest) (err error)
	/***** BaseLayer *****/
	ManifestWrapperGetAllSystemAnnos(ctx context.Context, maniDataID int64) (as types.Annotations, err error)
	/***** Manipulations *****/
	ManipulationGetCreate(ctx context.Context, mp *types.Manipulation) (err error)
	ManipulationsGetByProfileID(ctx context.Context, prfID int64) (mps []types.Manipulation, err error)
	ManipuLayerFromConfigYAML(ctx context.Context, cmpl yamlCfg.ManipuLayer) (mpl types.ManipuLayer, err error)
	ManipulationFromYAML(ctx context.Context, cmp yamlCfg.Manipulation) (mp types.Manipulation, err error)
	ManipulationCheckRules(ctx context.Context, prfID, maniID int64) (ok bool, err error)
	ManipuCmdsGetByManipulayerID(ctx context.Context, mplyrID int64) (mpcs []types.ManipuCmd, err error)
	ManipuEnvsGetByManipulayerID(ctx context.Context, mplyrID int64) (mpes []types.ManipuEnv, err error)
	ManifestLinkManipulatedFrom(ctx context.Context, manipulatedID, originalID int64) (err error)
	ManifestGetManipulatedByOriginalAndLabel(ctx context.Context, originalDigest, manipulationLabel string) (md types.Manifest, found bool, err error)
	/**** Artefact ********/
	ManifestLinkToArtifactConfig(ctx context.Context, mdDigest string, cfg v1.Descriptor) (err error)
	LinkManifestToSubject(ctx context.Context, mdDigest string, subId int64) (err error)
	LinkSubjectByDigestMediaType(ctx context.Context, refID int64, subDigest, subMediaType string) (err error)
	ReferrersGetBySubjectDigest(ctx context.Context, dig string) (refs types.ReferrersResponse, err error)
	/**** ORAS ********/
	OrasArtifactGetByRepoLike(ctx context.Context, repo string) ([]types.OrasListing, error)
	OrasArtifactGetByName(ctx context.Context, repo, artifactType string) (types.OrasArtifact, error)
	OrasArtifactGetByID(ctx context.Context, manifestID int64) (types.OrasArtifact, error)
	/**** Profiles (all with rules) ********/
	ProfileGetAll(ctx context.Context) ([]types.Profile, error)
	// ProfileGetByLabelWithStats returns a single profile by label together
	// with the count of distinct collections (Slots) and manifests served
	// under it. found is false when no profile with that label exists.
	ProfileGetByLabelWithStats(ctx context.Context, label string) (ps types.ProfileStats, found bool, err error)
	// ManifestGetProfileLabels resolves which profile(s) serve each manifest
	// in the given digest list. Returns a map of digest → profile labels.
	ManifestGetProfileLabels(ctx context.Context, digests []string) (map[string][]string, error)
	// CollectionListingGetByRepoLikeForProfile filters collection listings
	// by repo prefix AND profile label. When profileLabel is empty it falls
	// back to CollectionListingGetByRepoLike.
	CollectionListingGetByRepoLikeForProfile(ctx context.Context, repo string, profileLabel string) ([]types.CollectionListing, error)
	// RepoLinkTag ensures a Repo -[:HAS_TAG]-> Tag edge exists. Idempotent.
	RepoLinkTag(ctx context.Context, repo, tag string) error
	/******* Slot (Path Z slot-centred model) ********/
	// SlotGetCreate MERGEs a Slot for the given triple and its three join
	// edges to Repo/Tag/Profile. Idempotent on the compound (repo, tag,
	// profile) key.
	SlotGetCreate(ctx context.Context, repo, tag, profile string) (types.Slot, error)
	// SlotGetByTriple is a read-only lookup; found is false when no Slot
	// exists for the triple.
	SlotGetByTriple(ctx context.Context, repo, tag, profile string) (s types.Slot, found bool, err error)
	// SlotGetAllForRepoTag returns every Slot for a (repo, tag) pair
	// across all profiles. Used by the compute path.
	SlotGetAllForRepoTag(ctx context.Context, repo, tag string) ([]types.Slot, error)
	// SlotGetAllForRepo returns every Slot for a repo across all (tag,
	// profile) pairs. Used by the catalog endpoint.
	SlotGetAllForRepo(ctx context.Context, repo string) ([]types.Slot, error)
	// SlotLinkPointsTo replaces the Slot's current POINTS_TO edge with a
	// new one targeting indexID, archiving the previous target into
	// HAS_HISTORY atomically. Idempotent: re-linking the same Index is a
	// no-op.
	SlotLinkPointsTo(ctx context.Context, slotID, indexID int64) error
	// SlotAppendPushedManifest appends a timestamped HAS_PUSHED edge from
	// the Slot to the Manifest. Append-only, never deduplicated.
	SlotAppendPushedManifest(ctx context.Context, slotID, manifestID int64) error
	// SlotAppendPushedIndex appends a timestamped HAS_PUSHED_INDEX edge
	// from the Slot to the Index. Append-only.
	SlotAppendPushedIndex(ctx context.Context, slotID, indexID int64) error
	// ManifestGetAllForSlot walks HAS_PUSHED edges from the given Slot
	// and returns every content-leaf Manifest ever pushed to it,
	// deduplicated by manifest ID. Used by the compute path as the
	// Slot-first raw-md fetch. Returns an empty slice if the Slot
	// exists but has no pushes yet. See design D7.
	ManifestGetAllForSlot(ctx context.Context, slotID int64) ([]types.Manifest, error)
	// SetIndexOrigin sets the `origin` property on an :Index node with
	// first-writer-wins semantics. Path Z task 8.2 / design D8.
	// Valid origin values are neo.IndexOriginClientPushed and
	// neo.IndexOriginComputed.
	SetIndexOrigin(ctx context.Context, indexID int64, origin string) error
	// LinkIndexReferenceWithOrder MERGEs a :REFERENCES edge with an
	// `order` property, used by client-pushed indexes to preserve the
	// descriptor order from the original OCI manifest list. Idempotent.
	// Path Z task 8.2.
	LinkIndexReferenceWithOrder(ctx context.Context, indexID, mdID int64, order int) error
}
