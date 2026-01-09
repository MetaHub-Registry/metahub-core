package sqlite

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type SQBrowserContainer struct {
	testcontainers.Container
	HttpURI string
}

func SetupSQBrowser(ctx context.Context, dbDir, sqbPort string) (*SQBrowserContainer, error) {
	ports := []string{}
	sqP := os.Getenv("SQBROWSER_PORT")
	if sqP != "" {
		ports = append(ports, fmt.Sprintf("%s:8080/tcp", sqP))
	} else {
		ports = append(ports, "8080/tcp")
	}
	req := testcontainers.ContainerRequest{
		Image:        "coleifer/sqlite-web",
		ExposedPorts: ports,
		WaitingFor:   wait.ForHTTP("/").WithPort("8080"),
		Env: map[string]string{
			"SQLITE_DATABASE": "/data/data.db",
		},
		Cmd: []string{},
		Mounts: testcontainers.ContainerMounts{
			testcontainers.BindMount(dbDir, "/data/data.db"),
		},
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

	httpPort, err := container.MappedPort(ctx, "8080")
	if err != nil {
		return nil, err
	}

	httpUri := fmt.Sprintf("%s:%s", ip, httpPort.Port())

	return &SQBrowserContainer{Container: container, HttpURI: httpUri}, nil
}
