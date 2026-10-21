package traffic_inspector

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/modules/dockercompose"
	serviceModel "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/model"
	workspaceModel "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
	"github.com/prasannashrestha011/devOrbit/internal/process"
)

type workspaceLookup interface {
	Get(context.Context, uuid.UUID) (*workspaceModel.Workspace, error)
}

type serviceLookup interface {
	Get(context.Context, uuid.UUID) (*serviceModel.Service, error)
}

type processLookup interface {
	Status(uuid.UUID) process.Status
}

type dockerLookup interface {
	Inspect(context.Context, string, dockercompose.Candidate) (*dockercompose.Project, error)
	Status(context.Context, string, *dockercompose.Project) error
}

type Resolver struct {
	Workspaces workspaceLookup
	Services   serviceLookup
	Processes  processLookup
	Docker     dockerLookup
	APIHost    string
}

func (r Resolver) Resolve(ctx context.Context, target Target) (Target, int, error) {
	workspace, err := r.Workspaces.Get(ctx, target.WorkspaceID)
	if err != nil || workspace == nil {
		return Target{}, 0, errors.New("workspace not found")
	}

	var port int
	switch target.Kind {
	case "local":
		target, port, err = r.resolveLocal(ctx, target)
	case "docker":
		target, port, err = r.resolveDocker(ctx, target, workspace.RootPath)
	default:
		return Target{}, 0, errors.New("kind must be local or docker")
	}
	if err != nil {
		return Target{}, 0, err
	}

	if port < 1 || port > 65535 {
		return Target{}, 0, errors.New("invalid HTTP port")
	}
	if net.JoinHostPort("127.0.0.1", strconv.Itoa(port)) == r.APIHost {
		return Target{}, 0, errors.New("cannot inspect the DevOrbit API")
	}

	if err := probeHTTP(ctx, port); err != nil {
		return Target{}, 0, err
	}
	return target, port, nil
}

func probeHTTP(ctx context.Context, port int) error {
	probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	client := &http.Client{
		Transport: &http.Transport{Proxy: nil},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	defer client.CloseIdleConnections()

	req, _ := http.NewRequestWithContext(probeCtx, http.MethodHead, loopbackURL(port).String()+"/", nil)
	response, err := client.Do(req)
	if err != nil {
		return errors.New("selected port is not a reachable HTTP endpoint")
	}
	_ = response.Body.Close()
	return nil
}

func selectPort(ports map[string]int, key string) (string, int, error) {
	// User explicitly selected a port
	if key != "" {
		if port, ok := ports[key]; ok {
			return key, port, nil
		}
		return "", 0, fmt.Errorf("HTTP port %q not found", key)
	}

	// Automatically select if exactly one port exists
	if len(ports) == 1 {
		for name, port := range ports {
			return name, port, nil
		}
	}

	if len(ports) == 0 {
		return "", 0, errors.New("no HTTP port available")
	}

	return "", 0, errors.New("multiple HTTP ports; select one")
}

func loopbackURL(port int) *url.URL {
	return &url.URL{Scheme: "http", Host: net.JoinHostPort("127.0.0.1", strconv.Itoa(port))}
}
