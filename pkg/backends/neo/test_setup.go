package neo

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
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

func SetupNeo4j(ctx context.Context, neoPorts string) (*Neo4jContainer, error) {
	ports := []string{"7474/tcp", "7687/tcp"}
	if neoPorts != "" {
		k, v, found := strings.Cut(neoPorts, ":")
		if found {
			ports = []string{
				fmt.Sprintf("%s:7474/tcp", k),
				fmt.Sprintf("%s:7687/tcp", v),
			}
		}
	}
	logrus.Infof("Starting Neo4j with ports: %v", ports)
	image := os.Getenv("METAHUB_NEO4J_IMAGE")
	if image == "" {
		// 4.4.9's JDK 11 crashes reading cgroup v2 on the GitHub runner
		// (CgroupV2Subsystem NPE); the latest 4.4 patch carries the fix.
		image = "neo4j:4.4.44"
	}
	req := testcontainers.ContainerRequest{
		Image:        image,
		ExposedPorts: ports,
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

/*
	func (n Neo) Setup_Profile(bs utils.Bootstrap, br *utils.BootstrapResult, d string) (err error) {
		logrus.Warn("Setup_Filter")
		switch d {
		case "platform-generic":
			f := types.NewEmptyFilter("platform-generic")
			r, err := types.NewRule(types.RuleTypeAnnotation, "eq", "org.supercontainers.hardware.cpu.target", "platform")
			if err != nil {
				return err
			}
			err = f.AddRule(r)
			if err != nil {
				return err
			}
			fID, err := n.FilterInsert(context.Background(), f)
			if err != nil {
				return err
			}
			br.Add(fmt.Sprintf("filter:%s#id", f.Name), fmt.Sprint(fID))
		case "mpich-generic":
			f := types.NewEmptyFilter("mpich-generic")
			r, err := types.NewRule(types.RuleTypeAnnotation, "eq", "org.supercontainers.hardware.cpu.target", "platform")
			if err != nil {
				return err
			}
			err = f.AddRule(r)
			if err != nil {
				return err
			}
			rMpich, err := types.NewRule(types.RuleTypeAnnotation, "eq", "org.supercontainers.mpi.implementation", "mpich")
			if err != nil {
				return err
			}
			err = f.AddRule(rMpich)
			if err != nil {
				return err
			}
			fID, err := n.FilterInsert(context.Background(), f)
			if err != nil {
				return err
			}
			br.Add(fmt.Sprintf("filter:%s#id", f.Name), fmt.Sprint(fID))

		}
		return
	}
*/

func (n Neo) Setup_Annotation(ctx context.Context, bs utils.Bootstrap, br *utils.BootstrapResult, k, v string) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"func": "Setup_Annotation",
	})
	_ = l
	defer span.End()
	a := types.NewAnnotation(k, v)
	aID, err := n.AnnotationGetInsert(ctx, a)
	if err != nil {
		return err
	}
	br.Add(fmt.Sprintf("annotation:%s:%s#id", k, v), fmt.Sprint(aID))
	return
}

func (n Neo) Setup_ManipuLayer(bs utils.Bootstrap, br *utils.BootstrapResult, d string) (err error) {
	logrus.Warnf("Setup_ManipuLayer: %s", d)
	ctx := context.Background()
	_ = ctx
	img, lyr, found := strings.Cut(d, "#")
	if !found {
		return fmt.Errorf("did not find '#': %s", d)
	}
	mplID, err := br.GetInt64(fmt.Sprintf("manipulayer:%s#id", img))
	if err != nil {
		return fmt.Errorf("failed to get ManipuLayer ID: %v", err)
	}
	lyrIDs, err := br.GetInt64RegEx(fmt.Sprintf(`layer:\d+:%s#id`, lyr))
	if err != nil {
		return fmt.Errorf("failed to get Layer ID: %v", err)
	}
	orderIDs, err := br.GetIntRegEx(fmt.Sprintf(`layer:\d+:%s#order`, lyr))
	if err != nil {
		return fmt.Errorf("failed to get Layer ID: %v", err)
	}
	mediaType, err := br.Get(fmt.Sprintf("layer:%s#mediaType", lyr))
	if err != nil {
		return fmt.Errorf("failed to get Layer MediaType: %v", err)
	}
	for k, lyrID := range lyrIDs {
		ordrKey := strings.Replace(k, "#id", "#order", 1)
		order := orderIDs[ordrKey]
		err = n.ManipuLayerLinkToLayer(ctx, mplID, lyrID, mediaType, order)
		if err != nil {
			return err
		}
	}
	return
}

func (n Neo) Setup_Profiles(ctx context.Context, bs utils.Bootstrap, br *utils.BootstrapResult, d string) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"func": "Setup_Profiles",
	})
	_ = l
	defer span.End()
	fname := d
	pre := bs.YAMLPrefix
	if pre == "" {
		err = fmt.Errorf(`the YAMLPrefix is empty, please use "bs.SetYAMLPrefix(<relative path to yaml root>)"`)
		return
	}
	path := filepath.Join(pre, fname)
	cPrfs, e := config.FindProfiles(path)
	if e != nil {
		err = errors.Wrap(e, fmt.Sprintf("failed to find profiles in %s", path))
		return err
	}
	for _, cPrf := range cPrfs {
		p, _ := n.ProfileFromYAML(ctx, cPrf)
		err = n.ProfileGetInsert(ctx, &p)
		if err != nil {
			return err
		}
		label := fmt.Sprintf("%s:%s", p.Name, p.Tag)
		if p.Name == "" {
			label = p.Tag
		}
		br.Add(fmt.Sprintf("profile:%s#id", label), fmt.Sprint(p.ID))

		msg := fmt.Sprintf("Profile '%s' created with ID: %d", label, p.ID)
		if p.Rule != "" {
			msg += fmt.Sprintf(" and Rule: %s", p.Rule)
		}
		logrus.Info(msg)
	}
	return
}

func (n Neo) Setup_Rules(ctx context.Context, bs utils.Bootstrap, br *utils.BootstrapResult, d string) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"func": "Setup_Rules",
	})
	_ = l
	defer span.End()
	pre := bs.YAMLPrefix
	path := filepath.Join(pre, d)
	rls, err := config.FindRules(path)
	if err != nil {
		return err
	}
	for _, rl := range rls {
		r := rl.ToTypesRule()
		rID, err := n.RuleGetCreate(ctx, r)
		if err != nil {
			return err
		}
		label := fmt.Sprintf("%s:%s", r.Name, r.Tag)
		br.Add(fmt.Sprintf("rule:%s#id", label), fmt.Sprint(rID))
		logrus.Infof("Rule '%s' created with ID: %d", label, rID)
		err = n.RuleEvalAll(ctx)
		if err != nil {
			err = errors.Wrap(err, "failed to eval all rules")
			return err
		}
	}
	return
}

func (n Neo) Setup_Platforms(br *utils.BootstrapResult, plts []types.Platform) (err error) {
	//logrus.Info("> Setup_Platforms")
	ctx := context.Background()
	// Create platforms
	for _, p := range plts {
		err := n.PlatformGetInsert(ctx, &p)
		if err != nil {
			return err
		}
		pStr := fmt.Sprintf("%s/%s", p.OS, p.Arch)
		if p.Variant != "" {
			pStr = fmt.Sprintf("%s/%s", pStr, p.Variant)
		}
		k := fmt.Sprintf("platform:%s#id", pStr)
		logrus.Warnf("Add '%s' with ID: %d", k, p.ID)
		br.Add(k, fmt.Sprint(p.ID))
		/*
			// Check if filters are already there, if so link
			f := types.Filter{
				Label: "generic",
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
		*/
	}
	return
}

func (n Neo) Setup_Config(ctx context.Context, br *utils.BootstrapResult, cfg types.Config, label string) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"func":  "Setup_Config",
		"label": label,
	})
	_ = l
	defer span.End()
	cfgId, err := n.ConfigInsert(ctx, cfg)
	if err != nil {
		return err
	}
	br.Add(fmt.Sprintf("config:%s#id", label), fmt.Sprint(cfgId))
	br.Add(fmt.Sprintf("config:%s#digest", label), cfg.Digest)
	br.Add(fmt.Sprintf("config:%s#mediaType", label), cfg.MediaType)
	return
}

func (n Neo) Setup_Layer(ctx context.Context, br *utils.BootstrapResult, ly types.Layer, bootstrapstring string) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"func":            "Setup_Layer",
		"bootstrapstring": bootstrapstring,
	})
	_ = l
	defer span.End()
	lyId, err := n.LayerPlainInsert(ctx, ly)
	if err != nil {
		return err
	}
	br.Add(fmt.Sprintf("layer:%s#id", bootstrapstring), fmt.Sprint(lyId))
	br.Add(fmt.Sprintf("layer:%s#digest", bootstrapstring), ly.Digest)
	br.Add(fmt.Sprintf("layer:%s#mediaType", bootstrapstring), ly.MediaType)
	spl := strings.Split(bootstrapstring, ":")
	if _, err := strconv.Atoi(spl[0]); err != nil {
		return fmt.Errorf("bootstrapstring '%s' is expected to have the order up front", bootstrapstring)
	} else {
		br.Add(fmt.Sprintf("layer:%s#order", bootstrapstring), spl[0])
	}
	return
}

func (n Neo) Setup_Upstream(br *utils.BootstrapResult, up types.Upstream, label string) (err error) {
	id, err := n.CreateUpstream(context.Background(), up)
	if err != nil {
		return err
	}
	br.Add(fmt.Sprintf("upstream:%s#id", label), fmt.Sprint(id))
	br.Add(fmt.Sprintf("upstream:%s#digest", label), up.Digest.String())
	br.Add(fmt.Sprintf("upstream:%s#registry", label), up.Reg)
	br.Add(fmt.Sprintf("upstream:%s#repo", label), up.Repo)
	br.Add(fmt.Sprintf("upstream:%s#tag", label), up.Tag)
	return
}

func (n Neo) Setup_Collection_FT_amd64(ctx context.Context, br *utils.BootstrapResult) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"func": "Setup_Collection_FT_amd64",
	})
	_ = l
	defer span.End()
	// Add Repo
	yamlCol := config.FTyamlCollectionamd64()
	colID, colTagID, err := n.CreateCollection(ctx, yamlCol)
	if err != nil {
		return err
	}
	br.Add("collection:ft#id", fmt.Sprint(colID))
	br.Add("tag:amd64#id", fmt.Sprint(colTagID))
	// Link Repo To content-leaf Manifest (Phase 4: wrapper node removed)
	return n.linkCollectionToLeaves(ctx, br, colID, colTagID, "manifestdata:ft_amd64#id")
}

// linkCollectionToLeaves links the given Repo + Tag to each content-leaf
// :Manifest identified by the supplied BootstrapResult keys. Linking is
// skipped (non-fatal) for keys whose content-leaf is not yet present — some
// fixtures bootstrap "collection:" before "manifestdata:" and perform their
// own linking afterwards.
func (n Neo) linkCollectionToLeaves(ctx context.Context, br *utils.BootstrapResult, colID, colTagID int64, keys ...string) (err error) {
	for _, key := range keys {
		maniID, e := br.GetInt64(key)
		if e != nil {
			logrus.Warnf("linkCollectionToLeaves: content-leaf %q not present yet, skipping link (non-fatal)", key)
			continue
		}
		if err = n.LinkCollectionToManifest(ctx, colID, maniID); err != nil {
			return err
		}
		if err = n.LinkCollectionTagToManifest(ctx, colTagID, maniID); err != nil {
			return err
		}
	}
	return
}

func (n Neo) Setup_Collection_FT_arm64(ctx context.Context, br *utils.BootstrapResult) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"func": "Setup_Collection_FT_arm64",
	})
	_ = l
	defer span.End()
	// Add Repo
	yamlCol := config.FTyamlCollectionArm64()
	colID, colTagID, err := n.CreateCollection(ctx, yamlCol)
	if err != nil {
		return err
	}
	br.Add("collection:ft#id", fmt.Sprint(colID))
	br.Add("tag:arm64#id", fmt.Sprint(colTagID))
	// Link Repo To content-leaf Manifest (Phase 4: wrapper node removed)
	return n.linkCollectionToLeaves(ctx, br, colID, colTagID, "manifestdata:ft_arm64#id")
}

func (n Neo) Setup_Collection_FT_generic(ctx context.Context, br *utils.BootstrapResult) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"func": "Setup_Collection_FT_generic",
	})
	_ = l
	defer span.End()
	// Add Repo
	yamlCol := config.FTyamlCollectionGeneric()
	colID, colTagID, err := n.CreateCollection(ctx, yamlCol)
	if err != nil {
		return err
	}
	br.Add("collection:ft#id", fmt.Sprint(colID))
	br.Add("tag:generic#id", fmt.Sprint(colTagID))
	// Link Repo To content-leaf Manifest (Phase 4: wrapper node removed)
	return n.linkCollectionToLeaves(ctx, br, colID, colTagID, "manifestdata:ft_arm64#id", "manifestdata:ft_amd64#id")
}

func (n Neo) Setup_Collection_FT_oras_cyclonedx(ctx context.Context, br *utils.BootstrapResult) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"func": "Setup_Collection_FT_oras_cyclonedx",
	})
	_ = l
	defer span.End()
	yamlCol := config.FTyamlCollectionCycloneDX()
	colID, colTagID, err := n.CreateCollection(ctx, yamlCol)
	if err != nil {
		return err
	}
	br.Add("collection:ft_oras#id", fmt.Sprint(colID))
	br.Add("tag:cyclonedx#id", fmt.Sprint(colTagID))
	// Link Repo to ORAS content-leaf Manifest (Phase 4: wrapper node removed)
	return n.linkCollectionToLeaves(ctx, br, colID, colTagID, "manifestdata:ft_oras_cyclonedx#id")
}

func (n Neo) Setup_Collection_FT_zen3(ctx context.Context, br *utils.BootstrapResult) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"func": "Setup_Collection_FT_zen3",
	})
	_ = l
	defer span.End()
	// Add Repo
	yamlCol := config.FTyamlCollectionZen3()
	colID, colTagID, err := n.CreateCollection(ctx, yamlCol)
	if err != nil {
		return err
	}
	br.Add("collection:ft#id", fmt.Sprint(colID))
	br.Add("tag:zen3#id", fmt.Sprint(colTagID))

	// Link Repo To content-leaf Manifest (Phase 4: wrapper node removed)
	return n.linkCollectionToLeaves(ctx, br, colID, colTagID, "manifestdata:ft_zen3#id")
}

func (n Neo) Setup_ManifestList_FT_generic(br *utils.BootstrapResult) (err error) {
	// // Manifest
	arm_ml := types.FTarmManifest()
	arm_ml.ID, err = br.GetInt64("manifestdata:ft_arm64#id")
	if err != nil {
		return err
	}
	amd_ml := types.FTarmManifest()
	amd_ml.ID, err = br.GetInt64("manifestdata:ft_amd64#id")
	if err != nil {
		return err
	}
	// // GENERIC ML
	ml_generic := types.FTgenericManifestlist()
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

func (n Neo) Cypher_Collection_FT_tag1(br *utils.BootstrapResult) (err error) {
	// Seed DB
	// Repo
	q := `MERGE (t:Tag {name: "tag1", repo: "qnib/featuretest"})`
	q += ` MERGE (c:Repo {repo: "qnib/featuretest"})`
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

func (n Neo) BootstrapTest(ctx context.Context, bs utils.Bootstrap) (br utils.BootstrapResult, err error) {
	br = utils.NewBootstrapResult()
	for _, b := range bs.Bootstraps {
		err = n.BootstrapTestWithBR(ctx, b, bs, &br)
		if err != nil {
			err = errors.Wrapf(err, whileFmrt, b)
			return
		}
	}
	return
}

func (n Neo) TeardownTest(bs utils.Bootstrap, br utils.BootstrapResult) {
	logrus.Info("> TeardownTest")
	if bs.PostTestSleep > 0 {
		s := strings.Split(n.neoConfig.URL, ":")
		if len(s) != 3 {
			logrus.Warnf("Could not parse URL: %s", n.neoConfig.URL)
			return
		}
		bPort := s[2]
		jHttp, ok := n.neoConfig.Args["TEST_JAEGER_HTTP"]
		if ok {
			logrus.Warnf("Tracing: http://%s", jHttp)
		}

		logrus.Warnf("http: %s // bolt: %s // BoltPort: %s", n.neoConfig.HTTP, n.neoConfig.URL, bPort)
		logrus.Warnf("User: %s / Passwd: %s", n.neoConfig.User, n.neoConfig.Passwd)
		time.Sleep(time.Duration(bs.PostTestSleep) * time.Second)
	}
	if bs.EmptyAll {
		err := n.EmptyData(true)
		if err != nil {
			logrus.Fatal(err)
		}
	}

}

func (n Neo) BootstrapTestWithBR(ctx context.Context, b string, bs utils.Bootstrap, br *utils.BootstrapResult) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"func":  "BootstrapTestWithBR",
		"layer": b,
	})
	_ = l
	defer span.End()
	before, rest, found := strings.Cut(b, ":")
	if !found {
		err = fmt.Errorf("no Setup for '%s'", b)
		return err
	}
	switch before {
	case "empty":
		emptyAll := false
		if rest == "all" {
			emptyAll = true
		}
		err = n.EmptyData(emptyAll)
		if err != nil {
			err = errors.Wrapf(err, whileFmrt, b)
			return
		}
		return
	case "annotation":
		err = n.Bootstrap_Annotation(ctx, bs, br, rest)
		if err != nil {
			err = errors.Wrapf(err, whileFmrt, b)
			return
		}
		return
	case "manipulation":
		err = n.Bootstrap_Manipulation(ctx, bs, br, rest)
		if err != nil {
			err = errors.Wrapf(err, whileFmrt, b)
			return
		}
		return
	case "rules":
		err = n.Bootstrap_Rules(ctx, bs, br, rest)
		if err != nil {
			err = errors.Wrapf(err, whileFmrt, b)
			return
		}
		return
	case "profile":
		err = n.Bootstrap_Profiles(ctx, bs, br, rest)
		if err != nil {
			err = errors.Wrapf(err, whileFmrt, b)
			return
		}
		return
	case "config":
		err = n.Bootstrap_Config(ctx, bs, br, rest)
		if err != nil {
			err = errors.Wrapf(err, whileFmrt, b)
			return
		}
		return
	case "upstream":
		err = n.Bootstrap_Upstream(ctx, bs, br, rest)
		if err != nil {
			err = errors.Wrapf(err, whileFmrt, b)
			return
		}
		return
	case "layer":
		if err = n.Bootstrap_Layer(ctx, bs, br, rest); err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
		return

	case "collection":
		if err = n.Bootstrap_Collection(ctx, bs, br, rest); err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
		return
	case "manifestdata":
		if err = n.Bootstrap_Manifest(ctx, bs, br, rest); err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
		return
	case "manifest":
		if err = n.Bootstrap_ManifestWrapper(ctx, bs, br, rest); err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
		return
	case "scenario":
		if err = n.Bootstrap_Scenario(ctx, bs, br, rest); err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
		return
	}
	// FIXME: Old way
	switch b {
	case "platforms:generic":
		err = n.Setup_Platforms(br, types.Example_PlatformGeneric)
		if err != nil {
			err = errors.Wrap(err, "Setup_Platforms:Generic")
			return
		}
	case "platforms:amd64":
		err = n.Setup_Platforms(br, types.Example_PlatformAMD64)
		if err != nil {
			err = errors.Wrap(err, "Setup_Platforms:AMD64")
			return
		}
	case "platforms:arm64":
		err = n.Setup_Platforms(br, types.Example_PlatformARM64)
		if err != nil {
			err = errors.Wrap(err, "Setup_Platforms:AMD64")
			return
		}
	case "manifestlist:ft_generic":
		err = n.Setup_ManifestList_FT_generic(br)
		if err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
	case "cypher:collection:ft_tag1":
		err = n.Cypher_Collection_FT_tag1(br)
		if err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}

	// Path Z task 6 phase C2: the former `cypher:manifestlist:ft_generic`,
	// `cypher:collection_only:ft_generic`, and `cypher:manifestlist:ft_skylake`
	// bootstrap cases were removed together with their helper functions
	// (`Cypher_ManifestList_FT_generic`, `Cypher_AddCollection_FT_generic`,
	// `Cypher_ManifestList_FT_skylake`). Those helpers seeded legacy `:ManifestWrapper`
	// wrapper nodes via `MERGE (pArm)<-[:HAS_PLATFORM]-(mArm:ManifestWrapper {label: ...})`.
	// Their only callers were the commented-out `TestBasic` in
	// `pkg/backends/magazine/catalog_test.go` and two commented-out tests in
	// `pkg/backends/neo/index_test.go:136-137`. No live test used them.
	default:
		err = fmt.Errorf("no Setup for '%s'", b)
		return err
	}
	return
}

func (n Neo) Bootstrap_Rules(ctx context.Context, bs utils.Bootstrap, br *utils.BootstrapResult, path string) (err error) {
	err = n.Setup_Rules(ctx, bs, br, path)
	if err != nil {
		err = errors.Wrapf(err, "Setup_Rules(%s)", path)
		return
	}

	return
}

func (n Neo) Bootstrap_Profiles(ctx context.Context, bs utils.Bootstrap, br *utils.BootstrapResult, path string) (err error) {
	err = n.Setup_Profiles(ctx, bs, br, path)
	if err != nil {
		err = errors.Wrapf(err, "Setup_Profiles(%s)", path)
		return
	}

	return
}

func (n Neo) Bootstrap_Annotation(ctx context.Context, bs utils.Bootstrap, br *utils.BootstrapResult, b string) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"func": "Bootstrap_Annotation",
	})
	_ = l
	defer span.End()
	sl := strings.Split(b, ":")
	if len(sl) != 2 {
		err = errors.Errorf("invalid bootstrap annotation: %s", b)
		return
	}
	err = n.Setup_Annotation(ctx, bs, br, sl[0], sl[1])
	if err != nil {
		err = errors.Wrapf(err, "Bootstrap_Annotation(%s, %s)", sl[0], sl[1])
		return
	}

	return
}

func (n Neo) Bootstrap_ManipuLayer(bs utils.Bootstrap, br *utils.BootstrapResult, b string) (err error) {
	err = n.Setup_ManipuLayer(bs, br, b)
	if err != nil {
		err = errors.Wrapf(err, "Setup_ManipuLayer(%s)", b)
		return
	}

	return
}

func (n Neo) Bootstrap_Config(ctx context.Context, bs utils.Bootstrap, br *utils.BootstrapResult, b string) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"func": "Bootstrap_Config",
		"b":    b,
	})
	_ = l
	defer span.End()
	switch b {
	case "ft_arm64":
		err = n.Setup_Config(ctx, br, types.FTarmConfig(), b)
		if err != nil {
			err = errors.Wrapf(err, whileFmrt, b)
			return
		}
	case "ft_amd64":
		err = n.Setup_Config(ctx, br, types.FTamd64Config(), b)
		if err != nil {
			err = errors.Wrapf(err, whileFmrt, b)
			return
		}
	case "ft_amd64_83f":
		err = n.Setup_Config(ctx, br, types.ExmplConfigAmd64x83f(), b)
		if err != nil {
			err = errors.Wrapf(err, whileFmrt, b)
			return
		}
	case "ft_zen3":
		err = n.Setup_Config(ctx, br, types.FTzen3Config(), b)
		if err != nil {
			err = errors.Wrapf(err, whileFmrt, b)
			return
		}
	case "ft_zen":
		err = n.Setup_Config(ctx, br, types.ConfigAmd64ZenConfig(), b)
		if err != nil {
			err = errors.Wrapf(err, whileFmrt, b)
			return
		}
	case "glow150arm64":
		err = n.Setup_Config(ctx, br, types.ConfigGlow150ARM64(), b)
		if err != nil {
			err = errors.Wrapf(err, whileFmrt, b)
			return
		}
	case "glow151arm64":
		err = n.Setup_Config(ctx, br, types.ConfigGlow151ARM64(), b)
		if err != nil {
			err = errors.Wrapf(err, whileFmrt, b)
			return
		}
	case "ft_oras_cyclonedx":
		err = n.Setup_Config(ctx, br, types.FTOrasConfig(), b)
		if err != nil {
			err = errors.Wrapf(err, whileFmrt, b)
			return
		}
	}

	return
}

func (n Neo) Bootstrap_Collection(ctx context.Context, bs utils.Bootstrap, br *utils.BootstrapResult, b string) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"func": "Bootstrap_Collection",
		"b":    b,
	})
	_ = l
	defer span.End()
	switch b {
	case "ft_generic":
		err = n.Setup_Collection_FT_generic(ctx, br)
		if err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
	case "ft_zen3":
		err = n.Setup_Collection_FT_zen3(ctx, br)
		if err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
	case "ft_arm64":
		err = n.Setup_Collection_FT_arm64(ctx, br)
		if err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
	case "ft_amd64":
		err = n.Setup_Collection_FT_amd64(ctx, br)
		if err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
	case "ft_oras_cyclonedx":
		err = n.Setup_Collection_FT_oras_cyclonedx(ctx, br)
		if err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
	default:
		return fmt.Errorf("no Setup for '%s'", b)
	}
	return
}

func (n Neo) Bootstrap_Upstream(ctx context.Context, bs utils.Bootstrap, br *utils.BootstrapResult, b string) (err error) {
	switch b {
	case "ft_arm64":
		err = n.Setup_Upstream(br, types.FTarm64Upstream(), b)
		if err != nil {
			err = errors.Wrapf(err, whileFmrt, b)
			return
		}
	case "ft_amd64":
		err = n.Setup_Upstream(br, types.FTamd64Upstream(), b)
		if err != nil {
			err = errors.Wrapf(err, whileFmrt, b)
			return
		}
	case "ft_zen3":
		err = n.Setup_Upstream(br, types.FTzen3Upstream(), b)
		if err != nil {
			err = errors.Wrapf(err, whileFmrt, b)
			return
		}
	case "ft_zen":
		err = n.Setup_Upstream(br, types.ManifestAmd64Zen.Upstream, b)
		if err != nil {
			err = errors.Wrapf(err, whileFmrt, b)
			return
		}
	case "glow150arm64":
		err = n.Setup_Upstream(br, types.UpstreamGlow150ARM64, b)
		if err != nil {
			err = errors.Wrapf(err, whileFmrt, b)
			return
		}
	case "glow151arm64":
		err = n.Setup_Upstream(br, types.UpstreamGlow151ARM64, b)
		if err != nil {
			err = errors.Wrapf(err, whileFmrt, b)
			return
		}
	case "glow150amd64":
		err = n.Setup_Upstream(br, types.UpstreamGlow150AMD64, b)
		if err != nil {
			err = errors.Wrapf(err, whileFmrt, b)
			return
		}
	case "ft_oras_cyclonedx":
		err = n.Setup_Upstream(br, types.FTOrasUpstream(), b)
		if err != nil {
			err = errors.Wrapf(err, whileFmrt, b)
			return
		}
	default:
		return fmt.Errorf("no Setup for '%s'", b)
	}
	return
}

func (n Neo) Bootstrap_Layer(ctx context.Context, bs utils.Bootstrap, br *utils.BootstrapResult, b string) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"func": "Bootstrap_Layer",
		"b":    b,
	})
	_ = l
	defer span.End()
	switch b {
	case "0:ft_arm64":
		err = n.Setup_Layer(ctx, br, types.FTarm64Layer0(), b)
		if err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
	case "1:ft_arm64":
		err = n.Setup_Layer(ctx, br, types.FTarm64Layer1(), b)
		if err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
	case "0:ft_amd64":
		err = n.Setup_Layer(ctx, br, types.FTamd64Layer0(), b)
		if err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
	case "1:ft_amd64":
		err = n.Setup_Layer(ctx, br, types.FTamd64Layer1(), b)
		if err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
	case "0:ft_zen_f144":
		err = n.Setup_Layer(ctx, br, types.LayerFeaturetestLf144(), b)
		if err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
	case "1:ft_zen_4c85":
		err = n.Setup_Layer(ctx, br, types.LayerFeaturetestL4c85(), b)
		if err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
	case "0:ft_zen3":
		err = n.Setup_Layer(ctx, br, types.FTzen3Layer0(), b)
		if err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
	case "1:ft_zen3":
		err = n.Setup_Layer(ctx, br, types.FTzen3Layer1(), b)
		if err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
	case "0:glow150arm64_2660":
		err = n.Setup_Layer(ctx, br, types.Layer0Glow150ARM64, b)
		if err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
	case "1:glow150arm64_3571":
		err = n.Setup_Layer(ctx, br, types.Layer1Glow150ARM64, b)
		if err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
	case "2:glow150arm64_d54e":
		err = n.Setup_Layer(ctx, br, types.Layer2Glow150ARM64, b)
		if err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
	case "0:glow151arm64_7c57":
		err = n.Setup_Layer(ctx, br, types.Layer0Glow151ARM64, b)
		if err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
	case "1:glow151arm64_8a17":
		err = n.Setup_Layer(ctx, br, types.Layer1Glow151ARM64, b)
		if err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
	case "2:glow151arm64_a24c":
		err = n.Setup_Layer(ctx, br, types.Layer2Glow151ARM64, b)
		if err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
	default:
		return fmt.Errorf("no Setup for '%s'", b)
	}
	return
}
