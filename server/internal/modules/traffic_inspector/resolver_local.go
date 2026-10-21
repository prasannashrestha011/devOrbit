package traffic_inspector

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/process"
)

func (r Resolver) resolveLocal(ctx context.Context, target Target) (Target, int, error) {
	serviceID, err := uuid.Parse(target.ServiceKey)
	if err != nil {
		return Target{}, 0, errors.New("invalid service id")
	}
	service, err := r.Services.Get(ctx, serviceID)
	if err != nil || service == nil || service.WorkspaceID != target.WorkspaceID {
		return Target{}, 0, errors.New("service not found in workspace")
	}
	if r.Processes.Status(serviceID).State != process.Running {
		return Target{}, 0, errors.New("start the service before inspecting traffic")
	}

	available := map[string]int{}
	for name, port := range service.Ports {
		if httpPortName(name) {
			available[name] = port
		}
	}

	portKey, port, err := selectPort(available, target.PortKey)
	if err != nil {
		return Target{}, 0, err
	}
	target.PortKey = portKey
	target.File = ""
	return target, port, nil
}

func httpPortName(name string) bool {
	name = strings.ToLower(name)
	return strings.HasPrefix(name, "http") || name == "web" || name == "api"
}
