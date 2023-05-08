package magazine

import (
	"context"
	"fmt"
	"os"
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
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".cobra-example" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".metahub-admin")
		viper.SetConfigType("yaml")
	}
	err = viper.ReadInConfig()
	if err != nil {
		logrus.Fatalf("Error reading config file, %s", err)
	}

	min, err := minio.NewMinio(cfgFile)
	if err != nil {
		err = errors.Wrapf(err, "failed to create minio backend")
		logrus.Fatal(err)
	}
	err = min.ConnectClients()
	if err != nil {
		err = errors.Wrapf(err, "failed to connect clients")
		logrus.Fatal(err)
	}

	err = min.BucketCheck()
	if err != nil {
		err = errors.Wrapf(err, "failed to execute BucketCheck")
		logrus.Fatal(err)
	}
	fr := viper.GetString("frontend.redirect")
	if fr != "" {
		logrus.Infof("Redirect frontend to: %s", fr)
	}
	au := viper.GetString("auth.url")
	var regList []string
	regs := viper.GetString("registries")
	if regs != "" {
		logrus.Infof("add pull-through registries from config: %s", regs)
		regList = append(regList, strings.Split(regs, ",")...)

	}
	cfg := parseFlags(cmd, regList)
	cfg.AuthURL = au
	cfg.RedirectDoc = fr
	ctx, e := initTracing(cmd, &cfg)
	if e != nil {
		err = errors.Wrapf(e, "failed to init tracing")
		logrus.Fatal(err)
	}
	if cfg.Tracing.Enabled {
		var span trace.Span
		ctx, span = tracer.Start(ctx, "NewMagazine")
		defer span.End()
	}

	m = Magazine{
		Minio:  min,
		Config: cfg,
	}
	// user
	userSqlFile, _ := cmd.Flags().GetString("user-db")
	m.UserManagement, err = sqlite.New(userSqlFile, "users,tokens")
	if err != nil {
		err = errors.Wrapf(err, "failed to create sqlite backend for users")
		logrus.Fatal(err)
	}
	// Neo4j
	neo4j, err := neo.New(cfgFile)
	if err != nil {
		err = errors.Wrapf(err, "failed to create neo4j backend")
		logrus.Fatal(err)
	}
	m.Database = neo4j
	err = m.Database.Setup(ctx)
	if err != nil {
		err = errors.Wrapf(err, "failed to setup neo4j backend")
		logrus.Fatal(err)

	}
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
		err := mag.ComputeManifests(ctx)
		if err != nil {
			err = errors.Wrap(err, "while SetupTest: ComputeManifests")
			return err
		}
	case "computeManifestlists":
		err := mag.ComputeManifests(ctx)
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
		err = mag.Minio.BootstrapTestWithBR(b, br)
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
