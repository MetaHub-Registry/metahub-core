package magazine

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/backends/minio"
	"github.com/metahub-registry/metahub-core/pkg/backends/sqlite"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

var mag Magazine

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
		Digest:     featureTestGenericCfgDigest,
		UUID:       uuid.New(),
		UploadedBy: "test",
		Repo:       "qnib/featuretest",
		BlobType:   "config",
		Data:       featureTestGenericCfg,
	}
)

func TestMain(m *testing.M) {
	logrus.Info("Start TestMain")
	ctx := context.Background()
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
		configsBucketKey:   minio.NewBucketConfig(configsBucketKey, minioC.HttpURI),
		layersBucketKey:    minio.NewBucketConfig(layersBucketKey, minioC.HttpURI),
		manifestsBucketKey: minio.NewBucketConfig(manifestsBucketKey, minioC.HttpURI),
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
	mag = Magazine{
		Minio:  min,
		SQlite: sq,
	}
	code := m.Run()
	os.Exit(code)
	logrus.Info("End TestMain")
}

func TeardownTest(bootstrap string) {
	logrus.Info("> TeardownTest")
	err := mag.Minio.EmptyAll()
	if err != nil {
		logrus.Fatal(err)
	}
	err = mag.SQlite.EmptyData()
	if err != nil {
		logrus.Fatal(err)
	}
}

func SetupTest(bootstrap string) (err error) {
	logrus.Info("> SetupTest")
	switch bootstrap {
	case "":
	case "cfg:generic":
		err = mag.StoreLayerUpload(ctx, &BlobUp_FeaturetestGeneric, BlobUp_FeaturetestGeneric.Data)
		return
	default:
		err = fmt.Errorf("No teardown for '%s'", bootstrap)
		return err
	}
	return
}
