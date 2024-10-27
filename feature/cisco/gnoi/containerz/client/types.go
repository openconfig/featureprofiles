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
	"time"

	"k8s.io/klog/v2"
)

// Progress contains progress information about this operation.
type Progress struct {
	Finished      bool
	Image         string
	Tag           string
	BytesReceived uint64
	Error         error
}

// ContainerInfo contains information about a container on the target system.
type ContainerInfo struct {
	ID        string
	Name      string
	ImageName string
	State     string

	Error error
}

// ImageInfo contains information about an image on the target system.
type ImageInfo struct {
	ID        string
	ImageName string
	ImageTag  string

	Error error
}

// VolumeInfo contains information about a volume on the target system.
type VolumeInfo struct {
	Name         string
	Driver       string
	Labels       map[string]string
	Options      map[string]string
	CreationTime time.Time
	Error        error
}

// LogMessage contains the log message retrieved from the target system as well as any error that
// may have occurred
type LogMessage struct {
	Msg string

	Error error
}

type startOptions struct {
	envs      []string
	ports     []string
	volumes   []string
	network   string
	capAdd    []string
	capRemove []string
	policy    string
	runAs     string
}

type nonBlockTypes interface {
	*Progress | *ContainerInfo | *LogMessage | *VolumeInfo | *ImageInfo
}

// StartOption is an option passed to a start container call.
type StartOption func(*startOptions)

// WithEnv sets the environment to be passed to the start operation.
func WithEnv(envs []string) StartOption {
	return func(opt *startOptions) {
		opt.envs = envs
	}
}

// WithPorts sets the ports to be passed to the start operation.
func WithPorts(ports []string) StartOption {
	return func(opt *startOptions) {
		opt.ports = ports
	}
}

// WithVolumes sets the volumes to be passed to the start operation.
func WithVolumes(volumes []string) StartOption {
	return func(opt *startOptions) {
		opt.volumes = volumes
	}
}

// WithNetwork sets the network to be passed to the start operation.
func WithNetwork(network string) StartOption {
	return func(opt *startOptions) {
		opt.network = network
	}
}

// WithCapabilities sets the capablities to be passed to the start operation.
func WithCapabilities(add, remove []string) StartOption {
	return func(opt *startOptions) {
		opt.capAdd = add
		opt.capRemove = remove
	}
}

// WithRestartPolicy sets the restart policy to be passed to the start operation.
func WithRestartPolicy(policy string) StartOption {
	return func(opt *startOptions) {
		opt.policy = policy
	}
}

// WithRunAs sets the run user and group to be passed to the start operation.
func WithRunAs(runAs string) StartOption {
	return func(opt *startOptions) {
		opt.runAs = runAs
	}
}

// nonBlockingChannelSend attempts to send a message in a non blocking manner. If the context is
// cancelled it simply returns with an indication that the context was cancelled
func nonBlockingChannelSend[T nonBlockTypes](ctx context.Context, ch chan T, data T) bool {
	select {
	case <-ctx.Done():
		return true
	case ch <- data:
		return false
	default:
		klog.Warningf("unable to send message; dropping")
	}
	return false
}
