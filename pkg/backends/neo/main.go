package neo

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mitchellh/go-homedir"
	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/metahub-registry/metahub-core/pkg/tracing"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"go.opentelemetry.io/otel"
)

var (
	tracer = otel.Tracer("neo")
)

type NeoConfig struct {
	DbName  string
	User    string
	Passwd  string
	URL     string
	HTTP    string
	Args    map[string]string
	Tracing tracing.TraceCfg
}

type Neo struct {
	Enabled   bool
	neoDriver neo4j.Driver
	neoConfig NeoConfig
	Args      map[string]string
}

func NewNeo(ncfg NeoConfig) (n Neo) {
	return Neo{
		Enabled:   true,
		neoConfig: ncfg,
		Args:      map[string]string{},
	}
}

func New(cfgFile string) (Neo, error) {
	viper.SetConfigType("yaml")
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".cobra-example" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".metahub-admin")
		viper.SetConfigType("yaml")
	}
	err := viper.ReadInConfig()
	if err != nil {
		logrus.Fatalf("Error reading config file, %s", err)
	}
	n := Neo{}
	nUrl := viper.GetString("neo4j.url")
	if nUrl == "" {
		logrus.Warnf("No neo4j.url set")
		n.Enabled = false
		return n, nil
	}
	ncfg := NeoConfig{
		DbName: viper.GetString("neo4j.dbname"),
		User:   viper.GetString("neo4j.user"),
		Passwd: viper.GetString("neo4j.password"),
		URL:    nUrl,
	}
	n = Neo{
		Enabled:   true,
		neoConfig: ncfg,
		Args:      map[string]string{},
	}
	err = n.Connect()
	if err != nil {
		logrus.Fatal(err)
	}
	return n, nil
}

func (n Neo) Backup(fname string) (err error) {
	return fmt.Errorf("'Backup' not implemented")
}

func (n *Neo) Connect() (err error) {
	n.neoDriver, err = neo4j.NewDriver(n.neoConfig.URL, neo4j.BasicAuth(n.neoConfig.User, n.neoConfig.Passwd, ""))
	if err != nil {
		logrus.Errorf("Error creating neo driver: %s\n", err)
		return
	}
	return
}

func (n Neo) Setup(ctx context.Context) (err error) {
	_, span := tracer.Start(ctx, "Neo.Setup")
	defer span.End()
	n.neoDriver, err = neo4j.NewDriver(n.neoConfig.URL, neo4j.BasicAuth(n.neoConfig.User, n.neoConfig.Passwd, ""))
	if err != nil {
		logrus.Errorf("Error creating neo driver: %s\n", err)
		return
	}
	logrus.Debug("Set database constraints (if not already present)")
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	if err != nil {
		logrus.Errorf("Error creating neo session: %s\n", err)
		return
	}
	defer session.Close()
	input := map[string]interface{}{}
	type pair struct {
		name string
		attr []string
	}
	nodes := []pair{
		{
			name: "Profile",
			attr: []string{"label"},
			//}, {
			//	name: "Upstream",
			//	attr: []string{"digest", "label"},
		}, {
			name: "Rule",
			attr: []string{"label"},
		}, {
			name: "Collection",
			attr: []string{"label"},
		}, {
			name: "Platform",
			attr: []string{"label"},
		}, {
			name: "Manifest",
			attr: []string{"digest", "mediaType"},
			//}, {
			//	name: "ManifestList",
			//	attr: "digest",
		}, {
			name: "Config",
			attr: []string{"digest", "mediaType"},
		}, {
			name: "Layer",
			attr: []string{"digest", "mediaType"},
		},
	}
	for _, t := range nodes {
		q := fmt.Sprintf(`CREATE CONSTRAINT constraint_node_%s IF NOT EXISTS FOR (p:%s) REQUIRE (p.`, strings.ToLower(t.name), t.name)
		q += strings.Join(t.attr, ", p.")
		q += `) IS UNIQUE`
		logrus.Debug(q)
		_ = input
		_, err = session.Run(q, input)
		if err != nil {
			logrus.Fatal(err.Error())
		}
	}

	logrus.Info("Neo4j Setup Done")
	return
}

func (n Neo) EmptyData(all bool) (err error) {
	return n.EmptyGraph()
}

func (n Neo) EmptyData_KeepUpstream() (err error) {
	logrus.Warn("Neo.EmptyData_KeepUpstream")
	q := `MATCH (n) WHERE NOT "Upstream" IN LABELS(n) DETACH DELETE n`
	_, err = n.Execute(q)
	return
}

func (n Neo) EmptyGraph() (err error) {
	logrus.Warn("Neo.EmptyGraph")
	q := `MATCH (n) DETACH DELETE n`
	_, err = n.Execute(q)
	return
}

func (n Neo) Execute(q string) (res types.Result, err error) {
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	if err != nil {
		logrus.Errorf("Error creating neo session: %s\n", err)
		return
	}
	defer session.Close()
	input := map[string]interface{}{}
	r, err := session.Run(q, input)
	if err != nil {
		logrus.Errorf("Error running query: %s\n", err)
		return
	}
	res = types.NewResult()
	for r.Next() {
		record := r.Record()
		keys, err := r.Keys()
		if err != nil {
			logrus.Errorf("Error getting keys: %s\n", err)
			return res, err
		}
		for idx, key := range keys {
			v := record.Values[idx]
			e := res.Add(key, v)
			if e != nil {
				err = errors.Wrap(e, "Error adding key to result")
				return res, err
			}
		}
	}
	return
}

func (n Neo) QueryOne(q string) (res types.Result, found bool, err error) {
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	if err != nil {
		logrus.Errorf("Error creating neo session: %s\n", err)
		return
	}
	defer session.Close()
	input := map[string]interface{}{}
	r, err := session.Run(q, input)
	if err != nil {
		logrus.Errorf("Error running query: %s\n", err)
		return
	}
	if r.Next() {
		record := r.Record()
		keys, err := r.Keys()
		if err != nil {
			logrus.Errorf("Error getting keys: %s\n", err)
			return res, found, err
		}
		res = types.NewResult()
		for idx, key := range keys {
			v := record.Values[idx]
			e := res.Add(key, v)
			if e != nil {
				err = errors.Wrap(e, "Error adding key to result")
				return res, found, err
			}
		}
		found = true
	}
	return
}

func (n Neo) Query(q string) (res []types.Result, err error) {
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	if err != nil {
		logrus.Errorf("Error creating neo session: %s\n", err)
		return
	}
	defer session.Close()
	input := map[string]interface{}{}
	r, err := session.Run(q, input)
	if err != nil {
		logrus.Errorf("Error running query: %s\n", err)
		return
	}
	res = []types.Result{}
	for r.Next() {
		record := r.Record()
		keys, err := r.Keys()
		if err != nil {
			logrus.Errorf("Error getting keys: %s\n", err)
			return nil, err
		}
		row := types.NewResult()
		for idx, key := range keys {
			v := record.Values[idx]
			e := row.Add(key, v)
			if e != nil {
				err = errors.Wrap(e, "Error adding key to row")
				return res, err
			}
		}
		res = append(res, row)
	}

	return
}

func (n Neo) Bootstrap_Rules(bs utils.Bootstrap, br *utils.BootstrapResult, path string) (err error) {
	err = n.Setup_Rules(bs, br, path)
	if err != nil {
		err = errors.Wrapf(err, "Setup_Rules(%s)", path)
		return
	}

	return
}

func (n Neo) Bootstrap_Profiles(bs utils.Bootstrap, br *utils.BootstrapResult, path string) (err error) {
	err = n.Setup_Profiles(bs, br, path)
	if err != nil {
		err = errors.Wrapf(err, "Setup_Profiles(%s)", path)
		return
	}

	return
}

func (n Neo) Bootstrap_Annotation(bs utils.Bootstrap, br *utils.BootstrapResult, b string) (err error) {
	sl := strings.Split(b, ":")
	if len(sl) != 2 {
		err = errors.Errorf("invalid bootstrap annotation: %s", b)
		return
	}
	err = n.Setup_Annotation(bs, br, sl[0], sl[1])
	if err != nil {
		err = errors.Wrapf(err, "Bootstrap_Annotation(%s, %s)", sl[0], sl[1])
		return
	}

	return
}

func (n Neo) Bootstrap_Config(bs utils.Bootstrap, br *utils.BootstrapResult, b string) (err error) {
	switch b {
	case "ft_arm64":
		err = n.Setup_Config(br, types.FT_arm_Config(), "ft_arm64")
		if err != nil {
			err = errors.Wrapf(err, "while '%s'", b)
			return
		}
	case "ft_amd64":
		err = n.Setup_Config(br, types.FT_amd64_Config(), "ft_amd64")
		if err != nil {
			err = errors.Wrapf(err, "while '%s'", b)
			return
		}
	case "ft_zen3":
		err = n.Setup_Config(br, types.FT_zen3_Config(), "ft_zen3")
		if err != nil {
			err = errors.Wrapf(err, "while '%s'", b)
			return
		}
	}

	return
}

func (n Neo) Bootstrap_Upstream(bs utils.Bootstrap, br *utils.BootstrapResult, b string) (err error) {
	switch b {
	case "ft_arm64":
		err = n.Setup_Upstream(br, types.FT_arm_Upstream(), "ft_arm64")
		if err != nil {
			err = errors.Wrapf(err, "while '%s'", b)
			return
		}
	case "ft_amd64":
		err = n.Setup_Upstream(br, types.FT_amd64_Upstream(), "ft_amd64")
		if err != nil {
			err = errors.Wrapf(err, "while '%s'", b)
			return
		}
	case "ft_zen3":
		err = n.Setup_Upstream(br, types.FT_zen3_Upstream(), "ft_zen3")
		if err != nil {
			err = errors.Wrapf(err, "while '%s'", b)
			return
		}
	}
	return
}

func (n Neo) Bootstrap_Layer(bs utils.Bootstrap, br *utils.BootstrapResult, b string) (err error) {
	switch b {
	case "0:ft_arm64":
		err = n.Setup_Layer(br, types.FT_arm64_Layer0(), b)
		if err != nil {
			err = errors.Wrapf(err, "while '%s'", b)
			return
		}
	case "1:ft_arm64":
		err = n.Setup_Layer(br, types.FT_arm64_Layer1(), b)
		if err != nil {
			err = errors.Wrapf(err, "while '%s'", b)
			return
		}
	case "0:ft_amd64":
		err = n.Setup_Layer(br, types.FT_amd64_Layer0(), b)
		if err != nil {
			err = errors.Wrapf(err, "while '%s'", b)
			return
		}
	case "1:ft_amd64":
		err = n.Setup_Layer(br, types.FT_amd64_Layer1(), b)
		if err != nil {
			err = errors.Wrapf(err, "while '%s'", b)
			return
		}
	case "0:ft_zen3":
		err = n.Setup_Layer(br, types.FT_zen3_Layer0(), b)
		if err != nil {
			err = errors.Wrapf(err, "while '%s'", b)
			return
		}
	case "1:ft_zen3":
		err = n.Setup_Layer(br, types.FT_zen3_Layer1(), b)
		if err != nil {
			err = errors.Wrapf(err, "while '%s'", b)
			return
		}
	}
	return
}

func (n Neo) BootstrapTestWithBR(b string, bs utils.Bootstrap, br *utils.BootstrapResult) (err error) {
	sl := strings.Split(b, ":")
	if len(sl) < 2 {
		err = fmt.Errorf("invalid bootstrap string '%s'", b)
		return
	}
	rest := strings.Join(sl[1:], ":")
	switch sl[0] {
	case "annotation":
		err = n.Bootstrap_Annotation(bs, br, rest)
		if err != nil {
			err = errors.Wrapf(err, "while '%s'", b)
			return
		}
		return
	case "rules":
		err = n.Bootstrap_Rules(bs, br, rest)
		if err != nil {
			err = errors.Wrapf(err, "while '%s'", b)
			return
		}
		return

	case "profile":
		err = n.Bootstrap_Profiles(bs, br, rest)
		if err != nil {
			err = errors.Wrapf(err, "while '%s'", b)
			return
		}
		return
	case "config":
		err = n.Bootstrap_Config(bs, br, rest)
		if err != nil {
			err = errors.Wrapf(err, "while '%s'", b)
			return
		}
		return
	case "upstream":
		err = n.Bootstrap_Upstream(bs, br, rest)
		if err != nil {
			err = errors.Wrapf(err, "while '%s'", b)
			return
		}
		return
	case "layer":
		err = n.Bootstrap_Layer(bs, br, rest)
		if err != nil {
			err = errors.Wrapf(err, "while '%s'", b)
			return
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
	case "manifestdata:ft_arm64":
		err = n.Setup_ManifestData(br, types.FT_arm_ManifestData(), b)
		if err != nil {
			err = errors.Wrapf(err, "while '%s'", b)
			return
		}
	case "manifestdata:ft_amd64":
		err = n.Setup_ManifestData(br, types.FT_amd_ManifestData(), b)
		if err != nil {
			err = errors.Wrapf(err, "while '%s'", b)
			return
		}
	case "manifestdata:ft_zen3":
		err = n.Setup_ManifestData(br, types.FT_zen3_ManifestData(), b)
		if err != nil {
			err = errors.Wrapf(err, "while '%s'", b)
			return
		}
	case "manifestlist:ft_generic":
		err = n.Setup_ManifestList_FT_generic(br)
		if err != nil {
			err = errors.Wrapf(err, "while '%s'", b)
			return
		}
	case "manifest:ft_arm64":
		err = n.Setup_Manifest_FT_arm64(br)
		if err != nil {
			err = errors.Wrapf(err, "while '%s'", b)
			return
		}
	case "manifest:ft_zen3":
		err = n.Setup_Manifest_FT_zen3(br)
		if err != nil {
			err = errors.Wrapf(err, "while '%s'", b)
			return
		}
	case "manifest:ft_amd64":
		err = n.Setup_Manifest_FT_amd64(br)
		if err != nil {
			br.PrintDict()
			err = errors.Wrapf(err, "while '%s'", b)
			return
		}
	case "manifest:ft_mpich_amd64":
		err = n.Setup_Manifest_FT_mpich_amd64(br)
		if err != nil {
			br.PrintDict()
			err = errors.Wrapf(err, "while '%s'", b)
			return
		}
	case "collection:ft_generic":
		err = n.Setup_Collection_FT_generic(br)
		if err != nil {
			err = errors.Wrapf(err, "while '%s'", b)
			return
		}
	case "collection:ft_zen3":
		err = n.Setup_Collection_FT_zen3(br)
		if err != nil {
			err = errors.Wrapf(err, "while '%s'", b)
			return
		}
	case "collection:ft_arm64":
		err = n.Setup_Collection_FT_arm64(br)
		if err != nil {
			err = errors.Wrapf(err, "while '%s'", b)
			return
		}
	case "collection:ft_amd64":
		err = n.Setup_Collection_FT_amd64(br)
		if err != nil {
			err = errors.Wrapf(err, "while '%s'", b)
			return
		}
	case "cypher:manifestlist:ft_generic":
		err = n.Cypher_ManifestList_FT_generic(br)
		if err != nil {
			err = errors.Wrapf(err, "while '%s'", b)
			return
		}
	case "cypher:collection:ft_tag1":
		err = n.Cypher_Collection_FT_tag1(br)
		if err != nil {
			err = errors.Wrapf(err, "while '%s'", b)
			return
		}

	case "cypher:collection_only:ft_generic":
		err = n.Cypher_AddCollection_FT_generic(br)
		if err != nil {
			err = errors.Wrapf(err, "while '%s'", b)
			return
		}
	case "cypher:manifestlist:ft_skylake":
		err = n.Cypher_ManifestList_FT_skylake(br)
		if err != nil {
			err = errors.Wrapf(err, "while '%s'", b)
			return
		}

	default:
		err = fmt.Errorf("no Setup for '%s'", b)
		return err
	}
	return
}

func (n Neo) BootstrapTest(bs utils.Bootstrap) (br utils.BootstrapResult, err error) {
	br = utils.NewBootstrapResult()
	for _, b := range bs.Bootstraps {
		err = n.BootstrapTestWithBR(b, bs, &br)
		if err != nil {
			err = errors.Wrapf(err, "while '%s'", b)
			return
		}
	}
	return
}

func (n Neo) TeardownTest(bs utils.Bootstrap) {
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
