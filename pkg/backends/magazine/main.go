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
		err = errors.Wrapf(err, "%s", msg)
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
	// Parse registries from YAML config — supports both legacy string and structured list
	var regList []string
	var yamlRegistries []RegistryConfig
	rawRegs := viper.Get("registries")
	switch v := rawRegs.(type) {
	case string:
		// Legacy format: "zot.example.com,docker.io"
		if v != "" {
			logrus.Infof("add pull-through registries from config (string): %s", v)
			for _, h := range strings.Split(v, ",") {
				h = strings.TrimSpace(h)
				if h == "" {
					continue
				}
				regList = append(regList, h)
				yamlRegistries = append(yamlRegistries, RegistryConfigFromHostname(h))
			}
		}
	case []interface{}:
		// Structured format: list of objects or list of strings
		for _, item := range v {
			switch entry := item.(type) {
			case map[interface{}]interface{}:
				rc := registryConfigFromMap(entry)
				yamlRegistries = append(yamlRegistries, rc)
				regList = append(regList, rc.EffectiveURL())
			case map[string]interface{}:
				rc := registryConfigFromStringMap(entry)
				yamlRegistries = append(yamlRegistries, rc)
				regList = append(regList, rc.EffectiveURL())
			case string:
				if entry != "" {
					regList = append(regList, entry)
					yamlRegistries = append(yamlRegistries, RegistryConfigFromHostname(entry))
				}
			}
		}
		if len(yamlRegistries) > 0 {
			logrus.Infof("add pull-through registries from config (structured): %d entries", len(yamlRegistries))
		}
	case nil:
		// No registries configured
	default:
		logrus.Warnf("unexpected registries config type %T, ignoring", rawRegs)
	}
	// Fail-fast: reject entries with empty hostnames (common with unquoted
	// colon-bearing YAML values like "hostname: registry:5050").
	if errs := ValidateRegistries(yamlRegistries); len(errs) > 0 {
		for _, e := range errs {
			logrus.Errorf("invalid registry config: %v", e)
		}
		logrus.Fatal("aborting: fix the registries block in the config file (quote hostnames containing colons)")
	}
	// CORS
	corsEnabled := viper.GetBool("cors.enabled")
	logrus.Infof("CORS enabled: %t", corsEnabled)
	cors := CORS{
		Enabled: corsEnabled,
	}
	cfg := parseFlags(cmd, regList, yamlRegistries)
	cfg.CORS = cors
	cfg.AuthURL = au
	cfg.RedirectDoc = fr
	// No-overcommit logical storage-pool budget (add-registry-project-quotas);
	// 0/absent means unlimited (no pool cap enforced).
	cfg.LogicalPoolBytes = viper.GetInt64("registry.logical_pool_bytes")
	if e := initOauth(&cfg); e != nil {
		err = errors.Wrapf(e, "failed to init oauth")
		logrus.Fatal(err)
	}
	if e := initOIDC(&cfg); e != nil {
		err = errors.Wrapf(e, "failed to init oidc")
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
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"func": "setupBootStrap",
	})
	_ = l
	defer span.End()
	switch b {
	// Users/Profile/Platforms
	case "users":
		err = sqlite.Setup_Users(ctx, &mag.UserManagement, br)
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
		err = mag.Minio.BootstrapTestWithBR(ctx, bs, br, b)
		if err != nil {
			err = errors.Wrap(err, "Setup_Users in Minio")
			return
		}
	}
	return
}

func (mag Magazine) SetupTest(ctx context.Context, bs utils.Bootstrap) (br utils.BootstrapResult, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"func": "SetupTest",
	})
	_ = l
	defer span.End()
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

func parseFlags(cmd *cobra.Command, regList []string, yamlRegistries []RegistryConfig) (cfg MagConfig) {
	// Flags
	profileRegex, _ := cmd.Flags().GetString("profile")
	repoRegex, _ := cmd.Flags().GetString("repo")
	tagRegex, _ := cmd.Flags().GetString("tag")
	manifest, _ := cmd.Flags().GetString("manifest")
	flagReg, _ := cmd.Flags().GetString("registries")
	// Build structured registries: YAML entries first, then CLI flag entries appended
	var registries []RegistryConfig
	registries = append(registries, yamlRegistries...)
	if flagReg != "" {
		logrus.Infof("append pull-through registries from flags: %s", flagReg)
		for _, h := range strings.Split(flagReg, ",") {
			h = strings.TrimSpace(h)
			if h == "" {
				continue
			}
			regList = append(regList, h)
			registries = append(registries, RegistryConfigFromHostname(h))
		}
	}
	return MagConfig{
		Tracing:    TraceCfg{Enabled: false},
		Registries: registries,
		Args: map[string]string{
			"profileRegex": profileRegex,
			"repoRegex":    repoRegex,
			"tagRegex":     tagRegex,
			"manifest":     manifest,
			// Backward compat: keep flat string for any code reading Args["registries"]
			"registries": strings.Join(regList, ","),
		},
	}
}

func initOIDC(cfg *MagConfig) (err error) {
	issuer := viper.GetString("oidc.issuer")
	if issuer == "" {
		return nil
	}
	cfg.OIDC = OIDCCfg{
		Issuer:       issuer,
		ClientID:     viper.GetString("oidc.clientid"),
		ClientSecret: viper.GetString("oidc.secret"),
		RedirectURI:  viper.GetString("oidc.redirect_uri"),
	}
	return nil
}

func initOauth(cfg *MagConfig) (err error) {
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
	if ctx == nil {
		// Older cobra returns a nil context when the command was not run via
		// ExecuteContext; a nil ctx panics the OTLP exporter's lazy Start.
		ctx = context.Background()
	}
	// tracing.* may be overridden via environment variables so pipelines (e.g.
	// the e2e evidence run) can point tracing at an OTLP backend such as
	// VictoriaTraces without editing the config file / k8s secret. Per viper
	// precedence a bound env var wins over the config-file value.
	_ = viper.BindEnv("tracing.endpoint", "TRACING_ENDPOINT")
	_ = viper.BindEnv("tracing.name", "TRACING_NAME")
	_ = viper.BindEnv("tracing.protocol", "TRACING_PROTOCOL")
	_ = viper.BindEnv("tracing.sync", "TRACING_SYNC")
	trEnd := viper.GetString("tracing.endpoint")
	svcName := viper.GetString("tracing.name")
	if svcName == "" || trEnd == "" {
		logrus.Warn("Tracing is disabled (tracing.name/tracing.endpoint is empty)")
	}
	if trEnd != "" {
		proto := viper.GetString("tracing.protocol")
		logrus.Infof("Tracing endpoint '%s' [proto=%s]: %s", svcName, proto, trEnd)
		if proto == "otlp" {
			// OTLP/HTTP exporter (e.g. VictoriaTraces). Use a synchronous
			// processor for short-lived CLI commands so spans are not dropped
			// on process exit (tracing.sync defaults false for long-running
			// services which flush via the batch processor).
			tr, err := tracing.InitTracerOTLP(ctx, svcName, trEnd, viper.GetBool("tracing.sync"))
			if err != nil {
				logrus.Fatalf("Could not initialize otlp tracer: %s", err.Error())
			}
			cfg.Tracing = TraceCfg{Enabled: true, Endpoint: trEnd, Tracer: tr}
		} else {
			tr, err := tracing.InitTracer(svcName, trEnd)
			if err != nil {
				logrus.Fatalf("Could not initialize jaeger tracer: %s", err.Error())
			}
			cfg.Tracing = TraceCfg{Enabled: true, Endpoint: trEnd, Tracer: tr}
		}
	}
	return
}
