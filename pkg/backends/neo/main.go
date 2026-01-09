package neo

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/mitchellh/go-homedir"
	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/metahub-registry/metahub-core/pkg/tracing"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

var (
	tracer = otel.Tracer("neo")
)

const (
	whileFmrt = "while '%s'"
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
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": "neo/main.go",
		"func":    "Neo.Setup",
	})
	defer span.End()
	_ = ctx
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
		}, {
			name: "Upstream",
			attr: []string{"digest", "label"},
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
			attr: []string{"label"},
		}, {
			//name: "ManifestData",
			//attr: []string{"digest", "mediaType"},
			//}, {
			name: "Config",
			attr: []string{"digest", "mediaType"},
		}, {
			name: "Layer",
			attr: []string{"digest"},
		}, {
			name: "ManipuLayer",
			attr: []string{"label", "rule"},
		},
	}
	for _, t := range nodes {
		qDrop := fmt.Sprintf(`DROP CONSTRAINT constraint_node_%s IF EXISTS`, strings.ToLower(t.name))
		_, err = session.Run(qDrop, input)
		if err != nil {
			utils.ErrorHandler(l, span, err, "")
			l.Fatal()
		}
		q := fmt.Sprintf(`CREATE CONSTRAINT constraint_node_%s IF NOT EXISTS FOR (p:%s) REQUIRE (p.`, strings.ToLower(t.name), t.name)
		q += strings.Join(t.attr, ", p.")
		q += `) IS UNIQUE`
		logrus.Debug(q)
		_ = input
		_, err = session.Run(q, input)
		if err != nil {
			utils.ErrorHandler(l, span, err, "")
			l.Fatal()
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

func (n Neo) EmptyDataKeepUpstreamConfigLayer() (err error) {
	logrus.Warn("Neo.EmptyData_KeepUpstream")
	q := `MATCH (n) WHERE NOT LABELS(n) IN [['Config'],['Upstream'],['Layer']] DETACH DELETE n`
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
	return n.QueryOneWithCtx(nil, q)
}

func (n Neo) QueryOneWithCtx(ctx *context.Context, q string) (res types.Result, found bool, err error) {
	var l *logrus.Entry
	var span trace.Span

	if ctx != nil {
		l, _, span = utils.InitLogging(*ctx, tracer, map[string]any{
			"package": pkgNameMD,
			"func":    "QueryOneWithCtx",
		})
		defer span.End()
	}

	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	if err != nil {
		logrus.Errorf("Error creating neo session: %s\n", err)
		return
	}
	defer session.Close()
	input := map[string]interface{}{}
	utils.Debug(l, span, "QueryOne: %s", q)
	r, err := session.Run(q, input)
	if err != nil {
		logrus.Errorf("Error running query: %s\n", err)
		return
	}
	if r.Next() {
		utils.Debug(l, span, "after r.Next")
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
	utils.Debug(l, span, "found: %v", found)
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
