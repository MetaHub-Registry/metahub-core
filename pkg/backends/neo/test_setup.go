package neo

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"github.com/metahub-registry/metahub-core/pkg/yaml/config"
)

const (
	neoDb     = "test"
	neoUser   = "neo4j"
	neoPasswd = "test"
)

type Neo4jContainer struct {
	testcontainers.Container
	HttpURI string
	BoltURI string
}

func SetupNeo4j(ctx context.Context) (*Neo4jContainer, error) {
	req := testcontainers.ContainerRequest{
		Image:        "neo4j:4.4.9",
		ExposedPorts: []string{"7474/tcp", "7687/tcp"},
		WaitingFor:   wait.ForHTTP("/").WithPort("7474"),
		Env: map[string]string{
			"NEO4J_AUTH": fmt.Sprintf("%s/%s", neoUser, neoPasswd),
		},
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, err
	}

	ip, err := container.Host(ctx)
	if err != nil {
		return nil, err
	}

	mappedPort, err := container.MappedPort(ctx, "7474")
	if err != nil {
		return nil, err
	}
	boltPort, err := container.MappedPort(ctx, "7687")
	if err != nil {
		return nil, err
	}

	httpUri := fmt.Sprintf("http://%s:%s", ip, mappedPort.Port())
	boltUri := fmt.Sprintf("bolt://%s:%s", ip, boltPort.Port())

	return &Neo4jContainer{Container: container, HttpURI: httpUri, BoltURI: boltUri}, nil
}

func (n Neo) Setup_Filter(bs utils.Bootstrap, br *utils.BootstrapResult, d string) (err error) {
	ctx := context.Background()
	logrus.Warn("Setup_Filter")
	pre := bs.YAMLPrefix
	path := filepath.Join(pre, d)
	prfs, err := config.FindProfiles(ctx, path)
	if err != nil {
		return err
	}
	for _, prf := range prfs {
		f := types.Filter{
			Label: prf.Name,
		}
		feats := []string{}
		for _, act := range prf.Actions {
			for k, v := range act.Args {
				if k == "value" {
					feats = append(feats, v.(string))
				}
			}
		}
		f.Feature = strings.Join(feats, ",")
		msg := fmt.Sprintf("# Filter: %s", f.Label)
		if len(f.Feature) > 0 {
			msg += fmt.Sprintf(" [%s]", f.Feature)
		}
		//logrus.Info(msg)
		fID, err := n.Filter_Insert(ctx, f)
		if err != nil {
			return err
		}
		br.Add(fmt.Sprintf("filter:%s#id", f.Label), fmt.Sprint(fID))
	}
	return
}

func (n Neo) Setup_Profiles(bs utils.Bootstrap, br *utils.BootstrapResult, d string) (err error) {
	ctx := context.Background()
	logrus.Warn("Setup_Profiles")
	pre := bs.YAMLPrefix
	path := filepath.Join(pre, d)
	prfs, err := config.FindProfiles(ctx, path)
	if err != nil {
		return err
	}
	for _, prf := range prfs {
		fID, err := br.GetInt64(fmt.Sprintf("filter:%s#id", prf.Name))
		if err != nil {
			return err
		}
		// Profile
		p := types.Profile{
			Name: prf.Name,
		}
		err = n.Profile_GetInsert(ctx, &p)
		if err != nil {
			return err
		}
		br.Add(fmt.Sprintf("profile:%s#id", p.Name), fmt.Sprint(p.ID))
		//logrus.Infof("Profile '%s' created with ID: %d", p.Name, p.ID)
		err = n.LinkProfileToFilter(ctx, p.ID, fID)
		if err != nil {
			return err
		}
	}
	return
}

func (n Neo) Setup_Platforms(br *utils.BootstrapResult, plts []types.Platform) (err error) {
	//logrus.Info("> Setup_Platforms")
	// Create platforms
	for _, p := range plts {
		err := n.Platform_GetInsert(&p)
		if err != nil {
			return err
		}
		pStr := fmt.Sprintf("%s/%s", p.OS, p.Arch)
		/*if p.Variant != "" {
			pStr = fmt.Sprintf("%s/%s", pStr, p.Variant)
		}*/
		if p.Features != "" {
			pStr += fmt.Sprintf("[%s]", p.Features)
		}
		k := fmt.Sprintf("platform:%s#id", pStr)
		//logrus.Warnf("Add '%s' with ID: %d", k, p.ID)
		br.Add(k, fmt.Sprint(p.ID))
		// Check if filters are already there, if so link
		f := types.Filter{
			Label: "generic",
		}
		if p.Features != "" {
			f.Label = strings.TrimPrefix(p.Features, "cpu:")
		}
		fID, found, err := n.GetFilter(f)
		if err != nil {
			return err
		}

		if found {
			f.ID = fID

			err = n.InsertPlatformFilter(p, f)
			if err != nil {
				return err
			}
		} else {
			logrus.Warnf("No filter for platform '%s'", pStr)
		}

	}
	return
}

func (n Neo) Setup_Config(br *utils.BootstrapResult, cfg types.Config, label string) (err error) {
	ctx := context.Background()
	cfgId, err := n.Config_Insert(ctx, cfg)
	if err != nil {
		return err
	}
	br.Add(fmt.Sprintf("config:%s#id", label), fmt.Sprint(cfgId))
	br.Add(fmt.Sprintf("config:%s#digest", label), cfg.Digest)
	return
}

func (n Neo) Setup_Layer(br *utils.BootstrapResult, ly types.Layer, bootstrapstring string) (err error) {
	ctx := context.Background()
	lyId, err := n.Layer_PlainInsert(ctx, ly)
	if err != nil {
		return err
	}
	br.Add(fmt.Sprintf("%s#id", bootstrapstring), fmt.Sprint(lyId))
	br.Add(fmt.Sprintf("%s#digest", bootstrapstring), ly.Digest)
	return
}

func (n Neo) Setup_Upstream(br *utils.BootstrapResult, up types.Upstream, label string) (err error) {
	id, err := n.CreateUpstream(up)
	if err != nil {
		return err
	}
	br.Add(fmt.Sprintf("upstream:%s#id", label), fmt.Sprint(id))
	return
}

func (n Neo) Setup_Collection_FT_generic(br *utils.BootstrapResult) (err error) {
	ctx := context.Background()
	// Add Collection
	yamlCol := config.FT_yamlCollection_generic()
	colID, colTagID, err := n.CreateCollection(yamlCol)
	if err != nil {
		return err
	}
	br.Add("collection:ft#id", fmt.Sprint(colID))
	br.Add("tag:generic#id", fmt.Sprint(colTagID))
	if err != nil {
		return err
	}
	// Link Collection To Manifests
	for _, mani := range []string{"manifest:ft_arm64#id", "manifest:ft_amd64#id"} {
		mani_id, err := br.GetInt64(mani)
		if err != nil {
			return err
		}
		err = n.LinkCollectionTagToManifest(ctx, colTagID, mani_id)
		if err != nil {
			return err
		}
		err = n.LinkCollectionToManifest(ctx, colID, mani_id)
		if err != nil {
			return err
		}

	}
	return
}
func (n Neo) Setup_Collection_FT_amd64(br *utils.BootstrapResult) (err error) {
	ctx := context.Background()
	// Add Collection
	yamlCol := config.FT_yamlCollection_amd64()
	colID, colTagID, err := n.CreateCollection(yamlCol)
	if err != nil {
		return err
	}
	br.Add("collection:ft#id", fmt.Sprint(colID))
	br.Add("tag:amd64#id", fmt.Sprint(colTagID))
	if err != nil {
		return err
	}
	// Link Collection To Manifests
	for _, mani := range []string{"manifest:ft_amd64#id"} {
		mani_id, err := br.GetInt64(mani)
		if err != nil {
			return err
		}
		err = n.LinkCollectionTagToManifest(ctx, colTagID, mani_id)
		if err != nil {
			return err
		}
		err = n.LinkCollectionToManifest(ctx, colID, mani_id)
		if err != nil {
			return err
		}

	}
	return
}

func (n Neo) Setup_Collection_FT_arm64(br *utils.BootstrapResult) (err error) {
	ctx := context.Background()
	// Add Collection
	yamlCol := config.FT_yamlCollection_arm64()
	colID, colTagID, err := n.CreateCollection(yamlCol)
	if err != nil {
		return err
	}
	br.Add("collection:ft#id", fmt.Sprint(colID))
	br.Add("tag:arm64#id", fmt.Sprint(colTagID))
	if err != nil {
		return err
	}
	// Link Collection To Manifests
	for _, mani := range []string{"manifest:ft_arm64#id"} {
		mani_id, err := br.GetInt64(mani)
		if err != nil {
			return err
		}
		err = n.LinkCollectionTagToManifest(ctx, colTagID, mani_id)
		if err != nil {
			return err
		}
		err = n.LinkCollectionToManifest(ctx, colID, mani_id)
		if err != nil {
			return err
		}

	}
	return
}

func (n Neo) Setup_ManifestList_FT_generic(br *utils.BootstrapResult) (err error) {
	// // Manifest
	arm_ml := types.FT_arm_Manifest()
	arm_ml.ID, err = br.GetInt64("manifest:ft_arm64#id")
	if err != nil {
		return err
	}
	amd_ml := types.FT_arm_Manifest()
	amd_ml.ID, err = br.GetInt64("manifest:ft_amd64#id")
	if err != nil {
		return err
	}
	// // GENERIC ML
	ml_generic := types.FT_generic_Manifestlist()
	ml_generic_id, err := n.CreateManifestList(digest.Digest(ml_generic.Digest), ml_generic.MediaType, int(ml_generic.Size))
	if err != nil {
		return err
	}
	err = n.LinkManifestListToManifests(ml_generic_id, []types.Manifest{arm_ml, amd_ml})
	if err != nil {
		return err
	}
	return
}

func (n Neo) Setup_Manifest(br *utils.BootstrapResult, mani types.Manifest, label string) (err error) {
	ctx := context.Background()
	mani_id, err := n.Manifest_Insert(ctx, mani)
	if err != nil {
		return err
	}
	mani.ID = mani_id
	br.Add(fmt.Sprintf("%s#id", label), fmt.Sprint(mani_id))
	pStr := fmt.Sprintf("%s/%s", mani.Platform.OS, mani.Platform.Arch)
	// Link Platform
	platform_id, err := br.GetInt64(fmt.Sprintf("platform:%s#id", pStr))
	if err != nil {
		logrus.Error(err)
		return err
	}
	//logrus.Warnf("Manifest '%d' linked to platform '%s' with ID: %d", mani_id, pStr, platform_id)
	err = n.LinkManifestToPlatform(ctx, mani_id, platform_id)
	if err != nil {
		logrus.Error(err)
		return err
	}
	// Link Config
	arm_config_id, err := br.GetInt64(fmt.Sprintf("config:ft_%s#id", mani.Platform.Arch))
	if err != nil {
		logrus.Error(err)
		return err
	}
	err = n.LinkManifestToConfig(ctx, mani_id, arm_config_id)
	if err != nil {
		logrus.Error(err)
		return err
	}
	// Link Upstream
	arm_upstream_id, err := br.GetInt64(fmt.Sprintf("upstream:ft_%s#id", mani.Platform.Arch))
	if err != nil {
		logrus.Error(err)
		return err
	}
	err = n.LinkManifestToUpstream(ctx, mani_id, arm_upstream_id)
	if err != nil {
		logrus.Error(err)
		return err
	}
	return
}

func (n Neo) Cypher_Collection_FT_tag1(br *utils.BootstrapResult) (err error) {
	// Seed DB
	// Collection
	q := `MERGE (t:Tag {name: "tag1"})`
	q += ` MERGE (c:Collection {repo: "qnib/featuretest"})`
	q += ` MERGE (c)-[:HAS_TAG]->(t)`
	q += ` RETURN id(c) AS colID, id(t) AS tagID`
	res, err := n.Execute(q)
	if err != nil {
		return err
	}
	colID, err := res.GetInt64("colID")
	if err != nil {
		return err
	}
	br.Add("collection:ft#cypher_id", fmt.Sprint(colID))
	tagID, err := res.GetInt64("tagID")
	if err != nil {
		return err
	}
	br.Add("tag:tag1#cypher_id", fmt.Sprint(tagID))
	return
}

func (n Neo) Cypher_AddCollection_FT_generic(br *utils.BootstrapResult) (err error) {
	// Seed DB
	colID, err := br.GetInt64("collection:ft#cypher_id")
	if err != nil {
		return err
	}
	tagID, err := br.GetInt64("tag:tag1#cypher_id")
	if err != nil {
		return err
	}
	// Collection
	q := `MATCH (t1:Tag), (c1:Collection) WHERE id(c1) = %d AND id(t1) = %d`
	// Platform
	q += ` MERGE (p0:Profile {name: "generic"})`
	q += ` MERGE (f1:Filter {label: "generic", feature: ""})`
	q += ` MERGE (p0)-[:USES]->(f1)`
	q += ` MERGE (pArm:Platform {label: "linux/arm64", arch: "arm64", os: "linux", variant: "", features: ""})`
	q += ` MERGE (f1)-[:USES]->(pArm)`
	q += ` MERGE (mArm:Manifest {digest: "digest: sha256:ecb2919cfe22d74996ed677453f4e6f3bfd2806a7b591697a2fbea9ebee95db5", mediaType: "application/vnd.docker.distribution.manifest.v2+json", size: 735})`
	q += ` MERGE (pArm)<-[:IS]-(mArm)`
	q += ` MERGE (t1)-[:UPSTREAM]->(mArm)`
	q += ` MERGE (c1)-[:UPSTREAM]->(mArm)`
	q += ` MERGE (upArm:Upstream {reg: "metahub-registry.org", repo: "qnib/featuretest", tag: "tag1-linux.arm64"})`
	q += ` MERGE (upArm)<-[:UPSTREAM]-(mArm)`
	q += ` MERGE (cfgArm:Config {digest: "digest: sha256:cf5c26263929e84d7373d23b2692222cceceb3175b3325b183ead2ab6b5a5ce3", mediaType: "application/vnd.docker.container.image.v1+json", size: 2191})`
	q += ` MERGE (mArm)-[:USES]->(cfgArm)`
	q += ` MERGE (pAmd:Platform {label: "linux/amd64", arch: "amd64", os: "linux", variant: "", features: ""})`
	q += ` MERGE (f1)-[:USES]->(pAmd)`
	q += ` MERGE (mAmd:Manifest {digest: "digest: sha256:5bf8eee20fde14eb6e3e924c18fafa4d816e131f315810875136f0438ff7beed", mediaType: "application/vnd.docker.distribution.manifest.v2+json", size: 735})`
	q += ` MERGE (pAmd)<-[:IS]-(mAmd)`
	q += ` MERGE (t1)-[:UPSTREAM]->(mAmd)`
	q += ` MERGE (c1)-[:UPSTREAM]->(mAmd)`
	q += ` MERGE (upAmd:Upstream {reg: "metahub-registry.org", repo: "qnib/featuretest", tag: "tag1-linux.amd64"})`
	q += ` MERGE (upAmd)<-[:UPSTREAM]-(mAmd)`
	q += ` MERGE (cfgAmd:Config {digest: "digest: sha256:83fdbde95f56beb69853240df571a7f085c561b4f8d914c5cc5e0f171dcb86df", mediaType: "application/vnd.docker.container.image.v1+json", size: 2191})`
	q += ` MERGE (mAmd)-[:USES]->(cfgAmd)`
	query := fmt.Sprintf(q, colID, tagID)
	_, err = n.Execute(query)
	return
}

func (n Neo) Cypher_ManifestList_FT_generic(br *utils.BootstrapResult) (err error) {
	// Seed DB
	// Collection
	q := `MERGE (t1:Tag {name: "tag1"})`
	q += ` MERGE (c1:Collection {repo: "qnib/featuretest"})`
	// Platform
	q += ` MERGE (p0:Profile {name: "generic"})`
	q += ` MERGE (f1:Filter {label: "generic", feature: ""})`
	q += ` MERGE (p0)-[:USES]->(f1)`
	q += ` MERGE (pArm:Platform {label: "linux/arm64", arch: "arm64", os: "linux", variant: "", features: ""})`
	q += ` MERGE (f1)-[:USES]->(pArm)`
	q += ` MERGE (mArm:Manifest {digest: "digest: sha256:ecb2919cfe22d74996ed677453f4e6f3bfd2806a7b591697a2fbea9ebee95db5", mediaType: "application/vnd.docker.distribution.manifest.v2+json", size: 735})`
	q += ` MERGE (pArm)<-[:IS]-(mArm)`
	q += ` MERGE (mlGen:ManifestList {digest: "digest: sha256:9d23fb26fef6bd93a9ec12b89d4674b270a7a54cb09dfed6933c0abcacddfc0f", mediaType: "application/vnd.docker.distribution.manifest.list.v2+json", size: 106})`
	q += ` MERGE (mlGen)-[:INCLUDES]->(mArm)`
	q += ` MERGE (t1)-[:UPSTREAM]->(mArm)`
	q += ` MERGE (c1)-[:UPSTREAM]->(mArm)`
	q += ` MERGE (t1)-[:REF]->(mlGen)`
	q += ` MERGE (c1)-[:REF]->(mlGen)`
	q += ` MERGE (t1)-[:HAS_TAG]->(c1)`
	q += ` MERGE (pAmd:Platform {label: "linux/amd64", arch: "amd64", os: "linux", variant: "", features: ""})`
	q += ` MERGE (f1)-[:USES]->(pAmd)`
	q += ` MERGE (mAmd:Manifest {digest: "digest: sha256:5bf8eee20fde14eb6e3e924c18fafa4d816e131f315810875136f0438ff7beed", mediaType: "application/vnd.docker.distribution.manifest.v2+json", size: 735})`
	q += ` MERGE (pAmd)<-[:IS]-(mAmd)`
	q += ` MERGE (t1)-[:UPSTREAM]->(mAmd)`
	q += ` MERGE (c1)-[:UPSTREAM]->(mAmd)`
	q += ` MERGE (mlGen)-[:INCLUDES]->(mAmd)`
	q += ` MERGE (mlGen)-[:IMPLEMENTS]->(f1)`
	q += ` MERGE (upArm:Upstream {reg: "metahub-registry.org", repo: "qnib/featuretest", tag: "tag1-linux.arm64"})`
	q += ` MERGE (upArm)<-[:UPSTREAM]-(mArm)`
	q += ` MERGE (upAmd:Upstream {reg: "metahub-registry.org", repo: "qnib/featuretest", tag: "tag1-linux.amd64"})`
	q += ` MERGE (upAmd)<-[:UPSTREAM]-(mAmd)`
	_, err = n.Execute(q)
	return
}

func (n Neo) Cypher_ManifestList_FT_skylake(br *utils.BootstrapResult) (err error) {
	// Collection
	q := `MERGE (t1:Tag {name: "tag1"})`
	q += ` MERGE (c1:Collection {repo: "qnib/featuretest"})`
	// Platform
	q += ` MERGE (p0:Profile {name: "skylake"})`
	q += ` MERGE (f1:Filter {label: "skylake", feature: "cpu:skylake"})`
	q += ` MERGE (p0)-[:USES]->(f1)`
	q += ` MERGE (pIL:Platform {label: "skylake", arch: "amd64", os: "linux", variant: "", features: "cpu:skylake"})`
	q += ` MERGE (f1)-[:USES]->(pIL)`
	q += ` MERGE (mIL:Manifest {digest: "digest: sha256:bde4b56852850ea948d2b4426fffdcff31bdb8fa1aec6c822af03e93e114fa05", mediaType: "application/vnd.docker.distribution.manifest.v2+json", size: 735})`
	q += ` MERGE (pIL)<-[:IS]-(mIL)`
	/// ManifestList
	q += ` MERGE (mlIL:ManifestList {digest: "digest: sha256:07992d9ce9abfdf556cf553c9be9276c14fcfc325724be67e43364f6a8fa0add", mediaType: "application/vnd.docker.distribution.manifest.list.v2+json", size: 106})`
	q += ` MERGE (mlIL)-[:INCLUDES]->(mIL)`
	q += ` MERGE (mlIL)-[:IMPLEMENTS]->(f1)`
	q += ` MERGE (t1)-[:REF]->(mlIL)`
	q += ` MERGE (c1)-[:REF]->(mlIL)`
	q += ` MERGE (t1)-[:HAS_TAG]->(c1)`
	q += ` MERGE (t1)-[:UPSTREAM]->(mIL)`
	q += ` MERGE (c1)-[:UPSTREAM]->(mIL)`
	q += ` MERGE (upIL:Upstream {reg: "metahub-registry.org", repo: "qnib/featuretest", tag: "tag1-skylake"})<-[:UPSTREAM]-(mIL)`
	_, err = n.Execute(q)
	return
}
