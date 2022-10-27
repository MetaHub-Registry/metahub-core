package minio

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	minioAccessKey = "minio"
	minioSecretKey = "miniominio"
)

var min Minio

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

func newBucketConfig(bucket, url string) BucketConfig {
	bc := BucketConfig{
		URL:      url,
		Region:   "us-east-1",
		User:     minioAccessKey,
		Password: minioSecretKey,
		Redirect: false,
		Ssl:      false,
		Bucket:   bucket,
	}
	return bc
}

func TestMain(m *testing.M) {
	logrus.Info("Start TestMain")
	ctx := context.Background()
	minioC, err := SetupMinio(ctx)
	if err != nil {
		logrus.Fatal(err)
	}
	cfgName := "../../../misc/config/metahub-admin.yaml"
	min, err = NewMinio(cfgName)
	if err != nil {
		logrus.Fatal(err)
	}
	logrus.Warnf("Minio: %v", min)
	min.BucketConfig = map[string]BucketConfig{
		configsBucketKey:   newBucketConfig(configsBucketKey, minioC.HttpURI),
		layersBucketKey:    newBucketConfig(layersBucketKey, minioC.HttpURI),
		manifestsBucketKey: newBucketConfig(manifestsBucketKey, minioC.HttpURI),
	}
	err = min.ConnectClients()
	if err != nil {
		logrus.Fatal(err)
	}
	min.BucketCheck()
	for dig, blob := range cfgBlobs {
		min.PutConfig(ctx, blob, dig)
	}
	code := m.Run()
	os.Exit(code)
	logrus.Info("End TestMain")
}

type MinioContainer struct {
	testcontainers.Container
	HttpURI    string
	ConsoleURI string
}

func SetupMinio(ctx context.Context) (*MinioContainer, error) {
	req := testcontainers.ContainerRequest{
		Image:        "quay.io/minio/minio:RELEASE.2022-08-02T23-59-16Z",
		ExposedPorts: []string{"9000/tcp", "9001/tcp"},
		WaitingFor:   wait.ForHTTP("/minio/health/live").WithPort("9000"),
		Env: map[string]string{
			"MINIO_ROOT_USER":     minioAccessKey,
			"MINIO_ROOT_PASSWORD": minioSecretKey,
		},
		Cmd: []string{"server", "/data", "--console-address=0.0.0.0:9001"},
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, err
	}

	time.Sleep(time.Second * 3)
	ip, err := container.Host(ctx)
	if err != nil {
		return nil, err
	}

	httpPort, err := container.MappedPort(ctx, "9000")
	if err != nil {
		return nil, err
	}
	consolePort, err := container.MappedPort(ctx, "9001")
	if err != nil {
		return nil, err
	}

	httpUri := fmt.Sprintf("%s:%s", ip, httpPort.Port())
	consoleUri := fmt.Sprintf("%s:%s", ip, consolePort.Port())

	return &MinioContainer{Container: container, HttpURI: httpUri, ConsoleURI: consoleUri}, nil
}
