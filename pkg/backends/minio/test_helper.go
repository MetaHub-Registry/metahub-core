package minio

import (
	"context"
	"fmt"
	"time"

	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

const (
	minioAccessKey = "minio"
	minioSecretKey = "miniominio"
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

func Setup_Config_FT_arm64(min *Minio, br *utils.BootstrapResult) (err error) {
	ctx := context.Background()
	cfgByte := types.FT_arm_ConfigByte()
	cfgDig := digest.FromBytes(cfgByte)
	err = min.ConfigPutObj(ctx, cfgDig.String(), cfgByte)
	return
}

func Setup_Config_FT_amd64(min *Minio, br *utils.BootstrapResult) (err error) {
	ctx := context.Background()
	cfgByte := types.FT_amd64_ConfigByte()
	cfgDig := digest.FromBytes(cfgByte)
	err = min.ConfigPutObj(ctx, cfgDig.String(), cfgByte)
	return
}

func (m Minio) BootstrapTestWithBR(b string, br *utils.BootstrapResult) (err error) {
	switch b {
	case "config:ft_arm":
		err = Setup_Config_FT_arm64(&m, br)
		if err != nil {
			err = errors.Wrapf(err, "while '%s'", b)
			return
		}
	case "config:ft_amd64":
		err = Setup_Config_FT_amd64(&m, br)
		if err != nil {
			err = errors.Wrapf(err, "while '%s'", b)
			return
		}
	}
	return
}
