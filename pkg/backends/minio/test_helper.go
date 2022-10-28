package minio

import (
	"context"
	"fmt"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
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

func NewBucketConfig(bucket, url string) BucketConfig {
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
