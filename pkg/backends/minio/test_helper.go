package minio

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
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
	featureTestGenericCfgDigest = "sha256:f70ce6a20a96d310e32c1f5a005b24dc8b225f949a69e1ec46dc5b7c12779dc9"
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

const (
	minioAccessKey = "minio"
	minioSecretKey = "miniominio"
	whileFmrt      = "while '%s'"
)

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

func NewBucketConfig(bucket, url, console string) BucketConfig {
	bc := BucketConfig{
		URL:        url,
		ConsoleURL: console,
		Region:     "us-east-1",
		User:       minioAccessKey,
		Password:   minioSecretKey,
		Redirect:   false,
		Ssl:        false,
		Bucket:     bucket,
	}
	return bc
}

func Setup_Config(min *Minio, br *utils.BootstrapResult, dt types.DataTuple) (err error) {
	ctx := context.Background()
	return min.BlobPutObj(ctx, dt.Digest.String(), dt.Data)
}
func Setup_Config_FT_arm64(min *Minio, br *utils.BootstrapResult) (err error) {
	ctx := context.Background()
	cfgByte := types.ConfigByteFtArm64()
	cfgDig := digest.FromBytes(cfgByte)
	err = min.BlobPutObj(ctx, cfgDig.String(), cfgByte)
	return
}

func SetupConfig83f(min *Minio, br *utils.BootstrapResult) (err error) {
	ctx := context.Background()
	dt := types.ConfigByteFtArm64x83f()
	cfgByte := dt.Data
	cfgDig := digest.FromBytes(cfgByte)
	err = min.BlobPutObj(ctx, cfgDig.String(), cfgByte)
	return
}

func SetupConfig(min *Minio, br *utils.BootstrapResult, dt types.DataTuple) (err error) {
	ctx := context.Background()
	cfgByte := dt.Data
	cfgDig := digest.FromBytes(cfgByte)
	err = min.BlobPutObj(ctx, cfgDig.String(), cfgByte)
	return
}

func Setup_Config_FT_amd64_b109(min *Minio, br *utils.BootstrapResult) (err error) {
	ctx := context.Background()
	cfgByte := types.FT_amd64_b109_ConfigByte()
	cfgDig := digest.FromBytes(cfgByte)
	err = min.BlobPutObj(ctx, cfgDig.String(), cfgByte)
	return
}

func Setup_Config_FT_amd64(min *Minio, br *utils.BootstrapResult) (err error) {
	ctx := context.Background()
	cfgByte := types.FT_amd64_ConfigByte()
	cfgDig := digest.FromBytes(cfgByte)
	err = min.BlobPutObj(ctx, cfgDig.String(), cfgByte)
	return
}

func (m Minio) Setup_Layer(br *utils.BootstrapResult, layer types.Layer, b string) (err error) {
	ctx := context.Background()
	err = m.BlobPutObj(ctx, layer.Digest, layer.Data)
	return
}

func (m Minio) SetupTest(bs utils.Bootstrap) (br utils.BootstrapResult, err error) {
	br = utils.NewBootstrapResult()
	for _, b := range bs.Bootstraps {
		err = m.BootstrapTestWithBR(bs, &br, b)
		if err != nil {
			err = errors.Wrapf(err, whileFmrt, b)
			return
		}
	}
	return
}

func (m Minio) SetupLayer(br *utils.BootstrapResult, ly types.Layer, bootstrapstring string) (err error) {
	ctx := context.Background()
	err = m.BlobPutObj(ctx, ly.Digest, ly.Data)
	return
}

func (m Minio) BootstrapLayer(bs utils.Bootstrap, br *utils.BootstrapResult, b string) (err error) {
	switch b {
	case "0:glow150arm64_2660":
		if err = m.SetupLayer(br, types.Layer0Glow150ARM64, b); err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
	case "1:glow150arm64_3571":
		if err = m.SetupLayer(br, types.Layer1Glow150ARM64, b); err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
	case "2:glow150arm64_d54e":
		if err = m.SetupLayer(br, types.Layer2Glow150ARM64, b); err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
	case "0:glow151arm64_7c57":
		if err = m.SetupLayer(br, types.Layer0Glow151ARM64, b); err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
	case "1:glow151arm64_8a17":
		if err = m.SetupLayer(br, types.Layer1Glow151ARM64, b); err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
	case "2:glow151arm64_a24c":
		if err = m.SetupLayer(br, types.Layer2Glow151ARM64, b); err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
	case "0:ft_arm64":
		if err = m.SetupLayer(br, types.FT_arm64_Layer0(), b); err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
	case "1:ft_zen_4c85":
		if err = m.SetupLayer(br, types.LayerFeaturetestL4c85(), b); err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
	case "0:ft_zen_f144":
		if err = m.SetupLayer(br, types.LayerFeaturetestLf144(), b); err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
	case "1:ft_amd64":
		if err = m.SetupLayer(br, types.FT_amd64_Layer1(), b); err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
	case "0:ft_amd64":
		if err = m.SetupLayer(br, types.FT_amd64_Layer0(), b); err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
	case "1:ft_arm64":
		if err = m.SetupLayer(br, types.FT_arm64_Layer1(), b); err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}
	default:
		return errors.Errorf("unknown bootstrap %s", b)
	}
	return nil
}

func (m Minio) BootstrapTestWithBR(bs utils.Bootstrap, br *utils.BootstrapResult, b string) (err error) {
	ctx := context.Background()
	bsType, bsName, found := strings.Cut(b, ":")
	if !found {
		return errors.Errorf("invalid bootstrap %s", b)
	}
	switch bsType {
	case "layer":
		if err = m.BootstrapLayer(bs, br, bsName); err != nil {
			return errors.Wrapf(err, whileFmrt, b)
		}

	default:
		switch b {
		case "config:glow150arm64":
			err = Setup_Config(&m, br, types.ConfigDTGlow150ARM64)
			if err != nil {
				err = errors.Wrapf(err, whileFmrt, b)
				return
			}
		case "config:glow151arm64":
			err = Setup_Config(&m, br, types.ConfigDTGlow151ARM64)
			if err != nil {
				err = errors.Wrapf(err, whileFmrt, b)
				return
			}
		case "config:ft_arm64":
			err = Setup_Config_FT_arm64(&m, br)
			if err != nil {
				err = errors.Wrapf(err, whileFmrt, b)
				return
			}
		case "config:ft_amd64":
			err = Setup_Config_FT_amd64(&m, br)
			if err != nil {
				err = errors.Wrapf(err, whileFmrt, b)
				return
			}
		case "config:ft_amd64_b109":
			err = Setup_Config_FT_amd64_b109(&m, br)
			if err != nil {
				err = errors.Wrapf(err, whileFmrt, b)
				return
			}
		case "config:ft_amd64_83f":
			err = SetupConfig83f(&m, br)
			if err != nil {
				err = errors.Wrapf(err, whileFmrt, b)
				return
			}
		case "config:ft_zen":
			err = SetupConfig(&m, br, types.ConfigDTFeaturetestZenc259())
			if err != nil {
				err = errors.Wrapf(err, whileFmrt, b)
				return
			}
		case "cfg:generic":
			err := m.PutBlob(ctx, featureTestGenericCfg, featureTestGenericCfgDigest)
			if err != nil {
				logrus.Fatal(err)
			}
		case "manifestdata:ft_amd64":
			dt := types.ManifestDataAmd64Byte()
			err = m.ManifestsPutObj(ctx, dt.Digest.String(), dt.Data)
			if err != nil {
				err = errors.Wrapf(err, whileFmrt, b)
				return
			}
		case "manifestdata:ft_zen_5c99":
			dt := types.ManifestDataDTFeaturetestZen()
			err = m.ManifestsPutObj(ctx, dt.Digest.String(), dt.Data)
			if err != nil {
				err = errors.Wrapf(err, whileFmrt, b)
				return
			}
		default:
			logrus.Warnf("unknown MinIO bootstrap '%s'", b)
		}
	}

	return
}
