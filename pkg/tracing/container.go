package tracing

import (
	"context"
	"fmt"
	"os"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/otel/trace"
)

type JeagerContainer struct {
	testcontainers.Container
	HttpURI   string
	ThriftURI string
}
type TraceCfg struct {
	Enabled  bool
	Tracer   trace.Tracer
	Endpoint string
}

func SetupJeager(ctx context.Context) (*JeagerContainer, error) {
	ports := []string{"6831/udp", "14268"}
	jP := os.Getenv("JEAGER_PORT")
	if jP != "" {
		ports = append(ports, fmt.Sprintf("%s:16686/tcp", jP))
	} else {
		ports = append(ports, "16686/tcp")
	}
	req := testcontainers.ContainerRequest{
		Image:        "jaegertracing/all-in-one:latest",
		ExposedPorts: ports,

		WaitingFor: wait.ForListeningPort("14268"),
		Env:        map[string]string{},
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
	mappedPort, err := container.MappedPort(ctx, "16686")
	if err != nil {
		return nil, err
	}
	httpUri := fmt.Sprintf("%s:%s", ip, mappedPort.Port())

	thriftPort, err := container.MappedPort(ctx, "14268")
	if err != nil {
		return nil, err
	}
	thriftUri := fmt.Sprintf("%s:%s", ip, thriftPort.Port())
	return &JeagerContainer{Container: container, HttpURI: httpUri, ThriftURI: thriftUri}, nil
}
