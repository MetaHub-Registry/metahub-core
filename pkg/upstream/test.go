package upstream

import (
	"context"
	"fmt"

	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type RegContainer struct {
	testcontainers.Container
	HttpURI string
}

func SetupRegistry(ctx context.Context) (*RegContainer, error) {
	regPort := "5050"
	req := testcontainers.ContainerRequest{
		Image:        "quay.io/cqnib/testcontainer-registry:2023-05-31.1-amd64",
		ExposedPorts: []string{regPort},
		WaitingFor:   wait.ForListeningPort(nat.Port(regPort)),
		Env: map[string]string{
			"REGISTRY_HTTP_ADDR":                        fmt.Sprintf("0.0.0.0:%s", regPort),
			"REGISTRY_STORAGE_FILESYSTEM_ROOTDIRECTORY": "/opt/registry",
		},
		SkipReaper: false,
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, err
	}
	ip, err := container.Host(ctx)
	if err != nil {
		return nil, err
	}

	mappedPort, err := container.MappedPort(ctx, nat.Port(regPort))
	if err != nil {
		//readLogs(ctx, container)
		return nil, err
	}
	//readLogs(ctx, container)
	httpUri := fmt.Sprintf("%s:%s", ip, mappedPort.Port())
	return &RegContainer{Container: container, HttpURI: httpUri}, nil
}
