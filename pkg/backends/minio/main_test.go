package minio

import (
	"context"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
)

var (
	min Minio
	ctx = context.Background()
)

var (
	featureTestGenericManiDigest = "sha256:9a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b"
	featureTestGenericMani       = []byte(`{
		"schemaVersion": 2,
		"mediaType": "application/vnd.docker.distribution.manifest.v2+json",
		"config": {
		   "mediaType": "application/vnd.docker.container.image.v1+json",
		   "size": 1639,
		   "digest": "sha256:b109e1dac65af00234a8a7f817dcb1e18bc18ca97a5b805283995ebf1137e471"
		},
		"layers": [
		   {
			  "mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
			  "size": 2818413,
			  "digest": "sha256:59bf1c3509f33515622619af21ed55bbe26d24913cedbca106468a5fb37a50c3"
		   },
		   {
			  "mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
			  "size": 304,
			  "digest": "sha256:c1ac79912c878f2697b915c7df8741867e64b1d42a644befe65af041078b748b"
		   }
		]
	 }`)
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
	cfgBlobs = map[string][]byte{
		featureTestGenericCfgDigest: featureTestGenericCfg,
	}
)

func TestMain(m *testing.M) {
	logrus.Info("Start TestMain")
	ctx := context.Background()
	minioC, err := SetupMinio(ctx)
	if err != nil {
		logrus.Fatal(err)
	}

	logrus.Warnf("Minio: %v", min)
	bc := map[string]BucketConfig{
		configsBucketKey:   NewBucketConfig(configsBucketKey, minioC.HttpURI),
		layersBucketKey:    NewBucketConfig(layersBucketKey, minioC.HttpURI),
		manifestsBucketKey: NewBucketConfig(manifestsBucketKey, minioC.HttpURI),
	}
	min, err = NewMiniWithCfg(bc)
	if err != nil {
		logrus.Fatal(err)
	}
	err = min.ConnectClients()
	if err != nil {
		logrus.Fatal(err)
	}
	min.BucketCheck()
	defer minioC.Terminate(ctx)
	code := m.Run()
	os.Exit(code)
	logrus.Info("End TestMain")
}

func TeardownTest(bootstrap string) {
	logrus.Info("> TeardownTest")
	switch bootstrap {
	case "cfg:generic":
		err := min.ConfigRemoveObj(ctx, featureTestGenericCfgDigest)
		if err != nil {
			logrus.Fatal(err)
		}
	}
}

func SetupTest(bootstrap string) (err error) {
	logrus.Info("> SetupTest")
	switch bootstrap {
	case "cfg:generic":
		err := min.PutConfig(ctx, featureTestGenericCfg, featureTestGenericCfgDigest)
		if err != nil {
			logrus.Fatal(err)
		}
	case "mani:generic":
		err := min.PutConfig(ctx, featureTestGenericCfg, featureTestGenericCfgDigest)
		if err != nil {
			logrus.Fatal(err)
		}
	}
	return
}
