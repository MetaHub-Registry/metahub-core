package magazine

import (
	"context"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/metahub-registry/metahub-core/pkg/backends/minio"
	"github.com/metahub-registry/metahub-core/pkg/backends/neo"
	"github.com/metahub-registry/metahub-core/pkg/backends/sqlite"
	"github.com/metahub-registry/metahub-core/pkg/tracing"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

var (
	tracer = otel.Tracer("metalib/magazine")
)

type Magazine struct {
	Minio          minio.Minio
	Database       Database
	UserManagement sqlite.Sqlite
	Config         MagConfig
}

func checkErr(err error, msg string) {
	if err != nil {
		err = errors.Wrapf(err, msg)
		logrus.Fatal(err)
	}
}

func (mag *Magazine) Setup() (err error) {
	return
}

func NewMagazine(cmd *cobra.Command, tables string) (m Magazine, ctx context.Context, err error) {
	cfgFile, _ := cmd.Flags().GetString("config")
	// viper
	viper.SetConfigType("yaml")
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		checkErr(err, err.Error())

		// Search config in home directory with name ".cobra-example" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".metahub-admin")
		viper.SetConfigType("yaml")
	}
	err = viper.ReadInConfig()
	checkErr(err, "Error reading config file")

	min, err := minio.NewMinio(cfgFile)
	checkErr(err, "failed to create minio backend")
	err = min.ConnectClients()
	checkErr(err, "failed to connect to minio backend")
	err = min.BucketCheck()
	checkErr(err, "failed to execute BucketCheck")
	fr := viper.GetString("frontend.redirect")
	logrus.Infof("Redirect frontend to: %s", fr)
	au := viper.GetString("auth.url")
	var regList []string
	regs := viper.GetString("registries")
	if regs != "" {
		logrus.Infof("add pull-through registries from config: %s", regs)
		regList = append(regList, strings.Split(regs, ",")...)

	}
	// CORS
	corsEnabled := viper.GetBool("cors.enabled")
	logrus.Infof("CORS enabled: %t", corsEnabled)
	cors := CORS{
		Enabled: corsEnabled,
	}
	cfg := parseFlags(cmd, regList)
	cfg.CORS = cors
	cfg.AuthURL = au
	cfg.RedirectDoc = fr
	if e := initOauth(cmd, &cfg); e != nil {
		err = errors.Wrapf(e, "failed to init oauth")
		logrus.Fatal(err)
	}
	ctx, err = initTracing(cmd, &cfg)
	checkErr(err, "failed to init tracing")

	if cfg.Tracing.Enabled {
		var span trace.Span
		ctx, span = tracer.Start(ctx, "NewMagazine")
		defer span.End()
	}
	cacheReg, _ := cmd.Flags().GetString("cache-registry")
	if cacheReg != "" {
		logrus.Infof("Add cache registry: %s", cacheReg)
		cfg.Args["cache-registry"] = cacheReg
	}
	if cacheLayers, e := cmd.Flags().GetString("cache-layers"); e == nil && cacheLayers != "" {
		cfg.Args["cache-layers"] = cacheLayers
	}
	m = Magazine{
		Minio:  min,
		Config: cfg,
	}
	// user
	userSqlFile, _ := cmd.Flags().GetString("user-db")
	m.UserManagement, err = sqlite.New(userSqlFile, "users,secrets")
	if err != nil {
		err = errors.Wrapf(err, "failed to create sqlite backend for users")
		logrus.Fatal(err)
	}
	// Neo4j
	neo4j, err := neo.New(cfgFile)
	checkErr(err, "failed to create neo4j backend")

	m.Database = neo4j
	err = m.Database.Setup(ctx)
	checkErr(err, "failed to setup neo4j backend")
	return
}

func (mag Magazine) setupBootStrap(ctx context.Context, bs utils.Bootstrap, br *utils.BootstrapResult, b string) (err error) {
	switch b {
	// Users/Profile/Platforms
	case "users":
		err = sqlite.Setup_Users(&mag.UserManagement, br)
		if err != nil {
			err = errors.Wrap(err, "Setup_Users")
			return
		}
	case "computeManifests":
		err := mag.ComputeAllManifests(ctx)
		if err != nil {
			err = errors.Wrap(err, "while SetupTest: ComputeManifests")
			return err
		}
	case "computeManifestlists":
		err := mag.ComputeManifestlists(ctx)
		if err != nil {
			err = errors.Wrap(err, "while SetupTest: computeManifestlists")
			return err
		}
	default:
		err = mag.Database.BootstrapTestWithBR(ctx, b, bs, br)
		if err != nil {
			err = errors.Wrap(err, "Setup_Users in Database")
			return
		}
		err = mag.Minio.BootstrapTestWithBR(bs, br, b)
		if err != nil {
			err = errors.Wrap(err, "Setup_Users in Minio")
			return
		}
	}
	return
}

func (mag Magazine) SetupTest(ctx context.Context, bs utils.Bootstrap) (br utils.BootstrapResult, err error) {
	logrus.Info("> SetupTest")
	//return
	br = utils.NewBootstrapResult()
	for _, b := range bs.Bootstraps {
		e := mag.setupBootStrap(ctx, bs, &br, b)
		if e != nil {
			err = errors.Wrapf(e, "failed to setup bootstrap %s", b)
			return
		}
	}
	return
}

func parseFlags(cmd *cobra.Command, regList []string) (cfg MagConfig) {
	// Flags
	profileRegex, _ := cmd.Flags().GetString("profile")
	repoRegex, _ := cmd.Flags().GetString("repo")
	tagRegex, _ := cmd.Flags().GetString("tag")
	manifest, _ := cmd.Flags().GetString("manifest")
	flagReg, _ := cmd.Flags().GetString("registries")
	if flagReg != "" {
		logrus.Infof("prepend pull-through registries from flags: %s", flagReg)
		regList = append(strings.Split(flagReg, ","), regList...)
	}
	return MagConfig{
		Tracing: TraceCfg{Enabled: false},
		Args: map[string]string{
			"profileRegex": profileRegex,
			"repoRegex":    repoRegex,
			"tagRegex":     tagRegex,
			"manifest":     manifest,
			"registries":   strings.Join(regList, ","),
		},
	}
}

func initOauth(cmd *cobra.Command, cfg *MagConfig) (err error) {
	cfg.OAuth = OAuthCfg{
		Enabled: false,
	}
	clientID := viper.GetString("oauth.clientID")
	if clientID != "" {
		cfg.OAuth = OAuthCfg{
			Enabled:      true,
			ClientID:     clientID,
			ClientSecret: viper.GetString("oauth.secret"),
			StateString:  viper.GetString("oauth.state"),
		}
	}
	return nil
}

func initTracing(cmd *cobra.Command, cfg *MagConfig) (ctx context.Context, err error) {
	ctx = cmd.Context()
	trEnd := viper.GetString("tracing.endpoint")
	svcName := viper.GetString("tracing.name")
	if svcName == "" || trEnd == "" {
		logrus.Warn("Tracing is disabled (tracing.name/tracing.endpoint is empty)")
	}
	if trEnd != "" {
		logrus.Infof("Tracing endpoint '%s': %s", svcName, trEnd)
		tr, err := tracing.InitTracer(svcName, trEnd)
		if err != nil {
			logrus.Fatalf("Could not initialize jaeger tracer: %s", err.Error())
		}
		trCfg := TraceCfg{
			Enabled:  true,
			Endpoint: trEnd,
			Tracer:   tr,
		}
		cfg.Tracing = trCfg

	}
	return
}
