package traffic_inspector

import (
	"context"
	"errors"
	"strconv"

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
	status := r.Processes.Status(serviceID)
	if status.State != process.Running {
		return Target{}, 0, errors.New("start the service before inspecting traffic")
	}

	available := map[string]int{}
	if status.PID != nil {
		for _, port := range process.ListeningPorts(*status.PID) {
			available[strconv.Itoa(port)] = port
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
