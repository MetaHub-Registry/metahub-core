package neo

import (
	"fmt"
	"os"
	"strings"

	"github.com/mitchellh/go-homedir"
	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type NeoConfig struct {
	DbName string
	User   string
	Passwd string
	URL    string
}

type Neo struct {
	neoDriver neo4j.Driver
	neoConfig NeoConfig
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
	ncfg := NeoConfig{
		DbName: viper.GetString("neo4j.dbname"),
		User:   viper.GetString("neo4j.user"),
		Passwd: viper.GetString("neo4j.password"),
		URL:    viper.GetString("neo4j.url"),
	}
	n := Neo{
		neoConfig: ncfg,
	}
	return n, nil
}

func (n *Neo) Setup() (err error) {
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
