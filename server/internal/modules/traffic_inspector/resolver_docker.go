package traffic_inspector

import (
	"context"
	"errors"
	"net"
	"strconv"

	"github.com/prasannashrestha011/devOrbit/internal/modules/dockercompose"
)

func (r Resolver) resolveDocker(ctx context.Context, target Target, rootPath string) (Target, int, error) {
	candidates, err := dockercompose.DiscoverWorkspace(rootPath)
	if err != nil {
		return Target{}, 0, errors.New("cannot read Compose configuration")
	}
	candidate, err := dockercompose.Select(candidates, target.File)
	if err != nil {
		return Target{}, 0, err
	}
	project, err := r.Docker.Inspect(ctx, candidate.Directory, candidate)
	if err != nil {
		return Target{}, 0, errors.New("cannot inspect Compose service")
	}
	if err := r.Docker.Status(ctx, candidate.Directory, project); err != nil {
		return Target{}, 0, errors.New("Docker container is unavailable")
	}

	available := map[string]int{}
	found := false
	for _, service := range project.Services {
		if service.Name != target.ServiceKey {
			continue
		}
		found = true
		for _, container := range service.Containers {
			if container.State != "running" {
				continue
			}
			for _, binding := range container.Ports {
				if binding.Protocol != "tcp" || binding.Published == "" || !localBinding(binding.Host) {
					continue
				}
				port, err := strconv.Atoi(binding.Published)
				if err == nil && port > 0 && port <= 65535 {
					available[binding.Published] = port
				}
			}
		}
	}
	if !found {
		return Target{}, 0, errors.New("Compose service not found")
	}

	portKey, port, err := selectPort(available, target.PortKey)
	if err != nil {
		return Target{}, 0, err
	}
	target.PortKey = portKey
	target.File = candidate.File
	return target, port, nil
}

func localBinding(host string) bool {
	if host == "" || host == "0.0.0.0" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.To4() != nil && ip.IsLoopback()
}
