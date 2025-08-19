// Source: https://github.com/openconfig/containerz/blob/master/client/start_container.go

// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package client

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	cpb "github.com/openconfig/gnoi/containerz"
)

// StartContainer starts a container with the provided configuration and returns its instance name if the
// operation succeeded or an error otherwise.
func (c *Client) StartContainer(ctx context.Context, image string, tag string, cmd string, instance string, opts ...StartOption) (string, error) {
	req, err := startContainerRequestWithOptions(ctx, image, tag, cmd, instance, opts...)
	if err != nil {
		return "", err
	}

	resp, err := c.cli.StartContainer(ctx, req)
	if err != nil {
		return "", err
	}

	switch resp.GetResponse().(type) {
	case *cpb.StartContainerResponse_StartOk:
		return resp.GetStartOk().GetInstanceName(), nil
	case *cpb.StartContainerResponse_StartError:
		return "", status.Errorf(codes.Internal, "failed to start container: %s", resp.GetStartError().GetDetails())
	default:
		return "", status.Error(codes.Unknown, "unknown container state")
	}
}

func startContainerRequestWithOptions(ctx context.Context, image string, tag string, cmd string, instance string, opts ...StartOption) (*cpb.StartContainerRequest, error) {
	optionz := &startOptions{}
	for _, opt := range opts {
		opt(optionz)
	}

	portMappings, err := ports(optionz.ports)
	if err != nil {
		return nil, err
	}

	envMappings, err := envs(optionz.envs)
	if err != nil {
		return nil, err
	}

	volumeMappings, err := volumes(optionz.volumes)
	if err != nil {
		return nil, err
	}

	deviceMappings, err := devices(optionz.devices)
	if err != nil {
		return nil, err
	}

	capabilities, err := capabilities(optionz.capAdd, optionz.capRemove)
	if err != nil {
		return nil, err
	}

	runAs, err := runAs(optionz.runAs)
	if err != nil {
		return nil, err
	}

	restartPolicy, err := restart(optionz.policy)
	if err != nil {
		return nil, err
	}

	return &cpb.StartContainerRequest{
		ImageName:    image,
		Tag:          tag,
		Cmd:          cmd,
		Ports:        portMappings,
		Environment:  envMappings,
		InstanceName: instance,
		Volumes:      volumeMappings,
		Devices:      deviceMappings,
		Network:      optionz.network,
		Cap:          capabilities,
		RunAs:        runAs,
		Restart:      restartPolicy,
		Labels:       optionz.labels,
		Limits: &cpb.StartContainerRequest_Limits{
			MaxCpu:       optionz.cpus,
			SoftMemBytes: optionz.softMem,
			HardMemBytes: optionz.hardMem,
		},
	}, nil
}

func ports(ports []string) ([]*cpb.StartContainerRequest_Port, error) {
	mapping := make([]*cpb.StartContainerRequest_Port, 0, len(ports))
	for _, port := range ports {
		parts := strings.SplitN(port, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("port definition %s is invalid", port)
		}

		in, err := strconv.Atoi(parts[0])
		if err != nil {
			return nil, err
		}

		out, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, err
		}

		mapping = append(mapping, &cpb.StartContainerRequest_Port{Internal: uint32(in), External: uint32(out)})
	}

	return mapping, nil
}

func envs(envs []string) (map[string]string, error) {
	mapping := make(map[string]string, len(envs))

	for _, env := range envs {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("env definition %s is invalid", env)
		}
		mapping[parts[0]] = parts[1]
	}

	return mapping, nil
}

func volumes(volumes []string) ([]*cpb.Volume, error) {
	vols := make([]*cpb.Volume, 0, len(volumes))

	for _, volume := range volumes {
		parts := strings.SplitN(volume, ":", 3)
		switch len(parts) {
		case 2:
			vols = append(vols, &cpb.Volume{
				Name:       parts[0],
				MountPoint: parts[1],
			})
		case 3:
			vols = append(vols, &cpb.Volume{
				Name:       parts[0],
				MountPoint: parts[1],
				ReadOnly:   parts[2] == "ro",
			})
		default:
			return nil, fmt.Errorf("volume definition %s is invalid", volume)
		}
	}

	return vols, nil
}

func runAs(runAs string) (*cpb.StartContainerRequest_RunAs, error) {
	if runAs == "" {
		return nil, nil
	}
	parts := strings.SplitN(runAs, ":", 2)
	switch len(parts) {
	case 1:
		return &cpb.StartContainerRequest_RunAs{
			User: parts[0],
		}, nil
	case 2:
		return &cpb.StartContainerRequest_RunAs{
			User:  parts[0],
			Group: parts[1],
		}, nil
	default:
		return nil, fmt.Errorf("runAs definition %s is invalid", runAs)
	}
}

func restart(policy string) (*cpb.StartContainerRequest_Restart, error) {
	if policy == "" {
		return nil, nil
	}
	parts := strings.SplitN(policy, ":", 2)
	var attempts int
	var err error
	if len(parts) == 2 {
		attempts, err = strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("failed to parse attempts in restart policy: %v", err)
		}
	}

	var polType cpb.StartContainerRequest_Restart_Policy
	switch strings.ToLower(parts[0]) {
	case "always":
		polType = cpb.StartContainerRequest_Restart_ALWAYS
	case "on-failure":
		polType = cpb.StartContainerRequest_Restart_ON_FAILURE
	case "unless-stopped":
		polType = cpb.StartContainerRequest_Restart_UNLESS_STOPPED
	case "none":
		polType = cpb.StartContainerRequest_Restart_NONE
	default:
		return nil, status.Errorf(codes.FailedPrecondition, "restart policy `%s` is none of always, on-failure, unless-stopped, none", parts[0])
	}

	return &cpb.StartContainerRequest_Restart{
		Policy:   polType,
		Attempts: uint32(attempts),
	}, nil
}

func capabilities(capAdd, capRemove []string) (*cpb.StartContainerRequest_Capabilities, error) {
	return &cpb.StartContainerRequest_Capabilities{
		Add:    capAdd,
		Remove: capRemove,
	}, nil
}

func devices(devices []string) ([]*cpb.Device, error) {
	devs := make([]*cpb.Device, 0, len(devices))

	for _, dev := range devices {
		parts := strings.SplitN(dev, ":", 3)
		switch len(parts) {
		case 1: // Example: /dev/video0
			devs = append(devs, &cpb.Device{
				SrcPath:     parts[0],
				DstPath:     parts[0],
				Permissions: []cpb.Device_Permission{cpb.Device_READ, cpb.Device_WRITE, cpb.Device_MKNOD},
			})
		case 2: // Example: /dev/video0:/dev/video1
			devs = append(devs, &cpb.Device{
				SrcPath:     parts[0],
				DstPath:     parts[1],
				Permissions: []cpb.Device_Permission{cpb.Device_READ, cpb.Device_WRITE},
			})
		case 3: // Example: /dev/video0:/dev/video1:rwm
			var perms []cpb.Device_Permission
			for _, p := range parts[2] {
				switch p {
				case 'r':
					perms = append(perms, cpb.Device_READ)
				case 'w':
					perms = append(perms, cpb.Device_WRITE)
				case 'm':
					perms = append(perms, cpb.Device_MKNOD)
				default:
					return nil, fmt.Errorf("unknown device permission %c", p)
				}
			}
			devs = append(devs, &cpb.Device{
				SrcPath:     parts[0],
				DstPath:     parts[1],
				Permissions: perms,
			})
		default:
			return nil, fmt.Errorf("device definition %s is invalid", dev)
		}
	}

	return devs, nil
}
