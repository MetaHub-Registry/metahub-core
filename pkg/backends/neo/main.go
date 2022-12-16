package neo

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mitchellh/go-homedir"
	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

type NeoConfig struct {
	DbName string
	User   string
	Passwd string
	URL    string
	HTTP   string
}

type Neo struct {
	Enabled   bool
	neoDriver neo4j.Driver
	neoConfig NeoConfig
}

func NewNeo(ncfg NeoConfig) (n Neo) {
	return Neo{
		Enabled:   true,
		neoConfig: ncfg,
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

func (n Neo) Setup() (err error) {
	n.neoDriver, err = neo4j.NewDriver(n.neoConfig.URL, neo4j.BasicAuth(n.neoConfig.User, n.neoConfig.Passwd, ""))
	if err != nil {
		logrus.Errorf("Error creating neo driver: %s\n", err)
		return
	}
	logrus.Debug("Set database constraints (if not already present")
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	if err != nil {
		logrus.Errorf("Error creating neo session: %s\n", err)
		return
	}
	defer session.Close()
	input := map[string]interface{}{}
	type pair struct {
		name string
		attr string
	}
	nodes := []pair{
		{
			name: "Profile",
			attr: "name",
		}, {
			name: "Upstream",
			attr: "label",
		}, {
			name: "Filter",
			attr: "label",
		}, {
			name: "Collection",
			attr: "label",
		}, {
			name: "Platform",
			attr: "label",
		}, {
			name: "Manifest",
			attr: "digest",
			//}, {
			//	name: "ManifestList",
			//	attr: "digest",
		}, {
			name: "Config",
			attr: "digest",
		}, {
			name: "Layer",
			attr: "digest",
		},
	}
	for _, t := range nodes {
		q := fmt.Sprintf(`CREATE CONSTRAINT constraint_node_%s IF NOT EXISTS FOR (p:%s) REQUIRE p.%s IS UNIQUE`, strings.ToLower(t.name), t.name, t.attr)
		logrus.Debug(q)
		_ = input
		/// TODO: enable CREATE CONSTRAINT
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

func (n Neo) EmptyGraph() (err error) {
	logrus.Warn("Neo.EmptyGraph")
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	if err != nil {
		logrus.Errorf("Error creating neo session: %s\n", err)
		return
	}
	defer session.Close()
	input := map[string]interface{}{}
	q := `MATCH (n) DETACH DELETE n`
	_, err = session.Run(q, input)
	return
}

func (n Neo) Execute(q string) (res Result, err error) {
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
	res = NewResult()
	for r.Next() {
		record := r.Record()
		keys, err := r.Keys()
		if err != nil {
			logrus.Errorf("Error getting keys: %s\n", err)
			return res, err
		}
		for idx, key := range keys {
			v := record.Values[idx]
			res.Add(key, v)
		}
	}
	return
}

func (n Neo) QueryOne(q string) (res Result, found bool, err error) {
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
		res = NewResult()
		for idx, key := range keys {
			v := record.Values[idx]
			res.Add(key, v)
		}
		found = true
	}
	return
}

func (n Neo) Query(q string) (res []Result, err error) {
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
	res = []Result{}
	for r.Next() {
		record := r.Record()
		keys, err := r.Keys()
		if err != nil {
			logrus.Errorf("Error getting keys: %s\n", err)
			return nil, err
		}
		row := NewResult()
		for idx, key := range keys {
			v := record.Values[idx]
			row.Add(key, v)
		}
		res = append(res, row)
	}

	return
}

func (n Neo) BootstrapTestWithBR(b string, br *utils.BootstrapResult) (err error) {
	switch b {
	case "filter":
		err = n.Setup_Filter(br, "../../../misc/yaml/profiles/generic")
		if err != nil {
			err = errors.Wrap(err, "Setup_Filter")
			return
		}
	case "profiles":
		err = n.Setup_Profiles(br, "../../../misc/yaml/profiles/generic")
		if err != nil {
			err = errors.Wrap(err, "Setup_Profiles")
			return
		}
	case "platforms:generic":
		err = n.Setup_Platforms(br, types.Example_PlatformGeneric)
		if err != nil {
			err = errors.Wrap(err, "Setup_Platforms:Generic")
			return
		}
	case "platforms:amd64":
		err = n.Setup_Platforms(br, types.Example_PlatformAMD64)
		if err != nil {
			err = errors.Wrap(err, "Setup_Platforms:amd64")
			return
		}
	case "platforms:arm64":
		err = n.Setup_Platforms(br, types.Example_PlatformARM64)
		if err != nil {
			err = errors.Wrapf(err, "while '%s'", b)
			return
		}
	case "config:ft_arm":
		err = n.Setup_Config(br, types.FT_arm_Config(), "ft_arm64")
		if err != nil {
			err = errors.Wrapf(err, "while '%s'", b)
			return
		}
	case "config:ft_amd64":
		err = n.Setup_Config(br, types.FT_amd64_Config(), "ft_amd64")
		if err != nil {
			err = errors.Wrapf(err, "while '%s'", b)
			return
		}
	case "upstream:ft_arm64":
		err = n.Setup_Upstream(br, types.FT_arm_Upstream(), "ft_arm64")
		if err != nil {
			err = errors.Wrapf(err, "while '%s'", b)
			return
		}
	case "upstream:ft_amd64":
		err = n.Setup_Upstream(br, types.FT_amd64_Upstream(), "ft_amd64")
		if err != nil {
			err = errors.Wrapf(err, "while '%s'", b)
			return
		}
	case "layer0:ft_arm64":
		err = n.Setup_Layer(br, types.FT_arm64_Layer0(), b)
		if err != nil {
			err = errors.Wrapf(err, "while '%s'", b)
			return
		}
	case "layer1:ft_arm64":
		err = n.Setup_Layer(br, types.FT_arm64_Layer1(), b)
		if err != nil {
			err = errors.Wrapf(err, "while '%s'", b)
			return
		}
	case "layer0:ft_amd64":
		err = n.Setup_Layer(br, types.FT_amd64_Layer0(), b)
		if err != nil {
			err = errors.Wrapf(err, "while '%s'", b)
			return
		}
	case "layer1:ft_amd64":
		err = n.Setup_Layer(br, types.FT_amd64_Layer1(), b)
		if err != nil {
			err = errors.Wrapf(err, "while '%s'", b)
			return
		}
	case "manifest:ft_arm64":
		err = n.Setup_Manifest(br, types.FT_arm_Manifest(), b)
		if err != nil {
			err = errors.Wrapf(err, "while '%s'", b)
			return
		}
	case "manifest:ft_amd64":
		err = n.Setup_Manifest(br, types.FT_amd_Manifest(), b)
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
	case "collection:ft_generic":
		err = n.Setup_Collection_FT_generic(br)
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
	case "ComputeManifestlists":
		err = n.ComputeManifestlists()
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
		err = n.BootstrapTestWithBR(b, &br)
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
