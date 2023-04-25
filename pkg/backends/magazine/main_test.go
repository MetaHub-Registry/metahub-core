package magazine

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/backends/minio"
	"github.com/metahub-registry/metahub-core/pkg/backends/neo"
	"github.com/metahub-registry/metahub-core/pkg/backends/sqlite"
	"github.com/metahub-registry/metahub-core/pkg/tracing"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"github.com/metahub-registry/metahub-core/pkg/yaml/config"
	"golang.org/x/crypto/bcrypt"
)

const (
	neoDb     = "test"
	neoUser   = "neo4j"
	neoPasswd = "test"
)

var (
	mag             Magazine
	regHttpURI      string
	jeagerHttpURI   string
	jeagerThriftURI string
	svcName         = "magazine-test"
)

var (
	featureTestGenericCfgDigest = "sha256:b109e1dac65af00234a8a7f817dcb1e18bc18ca97a5b805283995ebf1137e471"
	featureTestGenericCfg       = []byte(`{
		"architecture":"amd64",
		"config":{
			"Env":[
				"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
				"SKIP_ENTRYPOINTS=true",
				"PLATFORM_FEATURES=arch:cascadelake"
			],
			"Cmd":["/usr/bin/entry.sh"],
			"Labels":{"platform.features":"arch:cascadelake"},
			"ArgsEscaped":true,
			"OnBuild":null
		},
		"created":"2022-05-05T11:21:27.783635915Z",
		"history":[
			{"created":"2021-11-24T20:19:40.199700946Z","created_by":"/bin/sh -c #(nop) ADD file:9233f6f2237d79659a9521f7e390df217cec49f1a8aa3a12147bbca1956acdb9 in / "},
			{"created":"2021-11-24T20:19:40.483367546Z","created_by":"/bin/sh -c #(nop)  CMD [\"/bin/sh\"]","empty_layer":true},
			{"created":"2022-05-05T11:21:27.783635915Z","created_by":"ARG PLATFORM_FEATURES=test","comment":"buildkit.dockerfile.v0","empty_layer":true},
			{"created":"2022-05-05T11:21:27.783635915Z","created_by":"LABEL platform.features=arch:cascadelake","comment":"buildkit.dockerfile.v0","empty_layer":true},
			{"created":"2022-05-05T11:21:27.783635915Z","created_by":"COPY bin/entry.sh /usr/bin/entry.sh # buildkit","comment":"buildkit.dockerfile.v0"},
			{"created":"2022-05-05T11:21:27.783635915Z","created_by":"CMD [\"/usr/bin/entry.sh\"]","comment":"buildkit.dockerfile.v0","empty_layer":true},
			{"created":"2022-05-05T11:21:27.783635915Z","created_by":"ENV SKIP_ENTRYPOINTS=true","comment":"buildkit.dockerfile.v0","empty_layer":true},
			{"created":"2022-05-05T11:21:27.783635915Z","created_by":"ENV PLATFORM_FEATURES=arch:cascadelake","comment":"buildkit.dockerfile.v0","empty_layer":true}
		],
		"os":"linux",
		"rootfs":{
			"type":"layers",
			"diff_ids":["sha256:8d3ac3489996423f53d6087c81180006263b79f206d3fdec9e66f0e27ceb8759","sha256:aff8eba572d7cf1d51ae8245ac42189a896b0d0f069ad5f130795916ade7587a"]}}`)
	BlobUp_FeaturetestGeneric = types.OCIBlobUpload{
		Digest:        featureTestGenericCfgDigest,
		UUID:          uuid.New(),
		UploadUser:    "test",
		UploadProfile: "generic",
		Repo:          "qnib/featuretest",
		BlobType:      "config",
		Data:          featureTestGenericCfg,
	}
)

func TestMain(m *testing.M) {
	logrus.Info("Start TestMain")
	ctx := context.Background()
	/// Neo4j
	neoC, err := neo.SetupNeo4j(ctx)
	if err != nil {
		logrus.Fatal(err)
	}
	defer neoC.Terminate(ctx)
	ncfg := neo.NeoConfig{
		DbName: neoDb,
		User:   neoUser,
		Passwd: neoPasswd,
		URL:    neoC.BoltURI,
		HTTP:   neoC.HttpURI,
	}
	gdb := neo.NewNeo(ncfg)
	err = gdb.Connect()
	if err != nil {
		logrus.Fatal(err)
	}
	err = gdb.Setup(ctx)
	if err != nil {
		logrus.Fatal(err)
	}
	/// Minio
	minioC, err := minio.SetupMinio(ctx)
	if err != nil {
		logrus.Fatal(err)
	}
	defer minioC.Terminate(ctx)
	configsBucketKey := minio.GetConfigsBucketKey()
	layersBucketKey := minio.GetLayersBucketKey()
	manifestsBucketKey := minio.GetManifestsBucketKey()
	bc := map[string]minio.BucketConfig{
		configsBucketKey:   minio.NewBucketConfig(configsBucketKey, minioC.HttpURI, minioC.ConsoleURI),
		layersBucketKey:    minio.NewBucketConfig(layersBucketKey, minioC.HttpURI, minioC.ConsoleURI),
		manifestsBucketKey: minio.NewBucketConfig(manifestsBucketKey, minioC.HttpURI, minioC.ConsoleURI),
	}
	min, err := minio.NewMiniWithCfg(bc)
	if err != nil {
		logrus.Fatal(err)
	}
	err = min.ConnectClients()
	if err != nil {
		logrus.Fatal(err)
	}
	min.BucketCheck()
	/// Registry
	regC, err := upstream.SetupRegistry(ctx)
	if err != nil {
		logrus.Fatal(err)
	}
	defer regC.Terminate(ctx)
	regHttpURI = regC.HttpURI
	//jeager
	jeagerC, err := tracing.SetupJeager(ctx)
	if err != nil {
		logrus.Fatal(err)
	}
	defer jeagerC.Terminate(ctx)
	jeagerHttpURI = jeagerC.HttpURI
	jeagerThriftURI = jeagerC.ThriftURI
	/// SQLite
	f, err := os.CreateTemp("", "sqlite-test")
	if err != nil {
		logrus.Fatal(err)
	}

	logrus.Infof("Start TestMain: %s", f.Name())
	defer os.Remove(f.Name())
	sq, err := sqlite.New(f.Name(), "")
	if err != nil {
		logrus.Fatal(err)
	}
	trEnd := fmt.Sprintf("http://%s/api/traces", jeagerC.ThriftURI)
	logrus.Infof("Tracing endpoint '%s': %s", svcName, trEnd)
	tr, err := tracing.InitTracer(svcName, trEnd)
	if err != nil {
		logrus.Fatalf("Could not initialize jaeger tracer: %s", err.Error())
	}
	trCfg := TraceCfg{
		Enabled:  true,
		Endpoint: jeagerC.ThriftURI,
		Tracer:   tr,
	}
	mag = Magazine{
		Minio:          min,
		Database:       gdb,
		UserManagement: sq,
		Config: MagConfig{
			Tracing: trCfg,
		},
	}
	mag.Config.Args = map[string]string{
		"TEST_REGISTRY":      regC.HttpURI,
		"TEST_JAEGER_HTTP":   jeagerC.HttpURI,
		"TEST_JAEGER_THRIFT": jeagerC.ThriftURI,
		"registries":         regC.HttpURI,
	}
	code := m.Run()
	os.Exit(code)
	logrus.Info("End TestMain")
}

func TeardownTest(bs utils.Bootstrap) {
	logrus.Info("> magazine.TeardownTest")
	if bs.PostTestSleep > 0 {
		logrus.Infof("Sleep %d seconds before teardown", bs.PostTestSleep)
		jh, ok := mag.Config.Args["TEST_JAEGER_HTTP"]
		if ok {
			logrus.Infof("Jaeger HTTP: http://%s", jh)
		}
	}
	mag.Minio.PreTearDown(bs)
	mag.Database.TeardownTest(bs)
	err := mag.Minio.EmptyAll()
	if err != nil {
		logrus.Fatal(err)
	}
	err = mag.UserManagement.EmptyData(bs.EmptyAll)
	if err != nil {
		logrus.Fatal(err)
	}
}

func Setup_FTarm_Config() {
	bu, b := types.FT_arm_BlobUpload_Config()
	err := mag.StoreBlobUpload(ctx, &bu, b)
	if err != nil {
		logrus.Fatal(err)
	}
}

func Setup_Users() {
	//logrus.Warn("Setup Users")
	usrs, err := config.FindUsers(ctx, "../../../misc/yaml/users")
	if err != nil {
		logrus.Fatal(err)
	}
	for _, user := range usrs {
		u := types.User{
			Username:  user.Username,
			FirstName: user.FirstName,
			LastName:  user.LastName,
			Email:     user.Email,
		}
		switch {
		case (user.Hash != "") && (user.Password != ""):
			logrus.Errorf("User '%s' has both 'hash' and 'password' set. Please remove one of them.", user.Username)
			continue
		case user.Hash != "":
			u.PasswordHash = user.Hash
		case user.Password != "":
			hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), 12)
			if err != nil {
				logrus.Errorf("Error while hashing password for user '%s': %s", user.Username, err)
				continue
			}
			u.PasswordHash = string(hashedPassword)
		}
		err := mag.InsertUser(ctx, u)
		if err != nil {
			logrus.Error(err)
		}
	}
}
