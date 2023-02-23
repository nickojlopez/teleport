/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package inventory

import (
	"context"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"

	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
)

const (
	nodeService           = "node"
	kubeService           = "kube"
	appService            = "app"
	dbService             = "db"
	windowsDesktopService = "windows_desktop"
)

// This regexp is used to validate if the host architecture fetched
// has the expected format.
var matchHostArchitecture = regexp.MustCompile(`^\w+$`)

// fetchConfig contains the configuration used by the fetchAgentMetadata method.
type fetchConfig struct {
	ctx context.Context
	// hello is the initial upstream hello message.
	hello proto.UpstreamInventoryHello
	// readFile is the method called to read a file.
	// It is configurable so that it can be mocked in tests.
	readFile func(name string) ([]byte, error)
	// execCommand is the method called to execute a command.
	// It is configurable so that it can be mocked in tests.
	execCommand func(name string) ([]byte, error)
}

// setDefaults sets the values of readFile and execCommand to the ones in the
// standard library. Having these two methods configurable allows us to mock
// them in tests.
func (cfg *fetchConfig) setDefaults() {
	if cfg.readFile == nil {
		cfg.readFile = os.ReadFile
	}
	if cfg.execCommand == nil {
		cfg.execCommand = func(name string) ([]byte, error) {
			return exec.Command(name).Output()
		}
	}
}

// fetchAgentMetadata fetches and calculates all agent metadata we are interested
// in tracking.
func fetchAgentMetadata(c *fetchConfig) proto.UpstreamInventoryAgentMetadata {
	c.setDefaults()
	return proto.UpstreamInventoryAgentMetadata{
		Version:               c.fetchVersion(),
		HostID:                c.fetchHostID(),
		Services:              c.fetchServices(),
		OS:                    c.fetchOS(),
		OSVersion:             c.fetchOSVersion(),
		HostArchitecture:      c.fetchHostArchitecture(),
		GLibCVersion:          c.fetchGlibcVersion(),
		InstallMethods:        c.fetchInstallMethods(),
		ContainerRuntime:      c.fetchContainerRuntime(),
		ContainerOrchestrator: c.fetchContainerOrchestrator(),
		CloudEnvironment:      c.fetchCloudEnvironment(),
	}
}

// fetchVersion returns the Teleport version present in the hello message.
func (c *fetchConfig) fetchVersion() string {
	return c.hello.Version
}

// fetchHostID returns the agent ID present in the hello message.
func (c *fetchConfig) fetchHostID() string {
	return c.hello.ServerID
}

// fetchOS returns the value of GOOS.
func (c *fetchConfig) fetchOS() string {
	return runtime.GOOS
}

// fetchServices computes the list of access protocols enabled at the agent from
// the list of system roles present in the hello message.
func (c *fetchConfig) fetchServices() []string {
	var services []string
	for _, svc := range c.hello.Services {
		switch svc {
		case types.RoleNode:
			services = append(services, nodeService)
		case types.RoleKube:
			services = append(services, kubeService)
		case types.RoleApp:
			services = append(services, appService)
		case types.RoleDatabase:
			services = append(services, dbService)
		case types.RoleWindowsDesktop:
			services = append(services, windowsDesktopService)
		}
	}
	return services
}

// fetchHostArchitecture computes the host architecture using the arch
// command-line utility.
func (c *fetchConfig) fetchHostArchitecture() string {
	return c.exec("arch", func(out string) (string, bool) {
		if matchHostArchitecture.MatchString(out) {
			return out, true
		}
		return "", false
	})
}

func (c *fetchConfig) fetchInstallMethods() []string {
	// TODO(vitorenesduarte): fetch install methods
	return []string{}

}

// fetchContainerRuntime returns "docker" if the file "/.dockerenv" exists.
func (c *fetchConfig) fetchContainerRuntime() string {
	return c.read("/.dockerenv", func(_ string) (string, bool) {
		// If the file exists, we should be running on Docker.
		return "docker", true
	})
}

func (c *fetchConfig) fetchContainerOrchestrator() string {
	// TODO(vitorenesduarte): fetch container orchestrator
	return ""
}

func (c *fetchConfig) fetchCloudEnvironment() string {
	// TODO(vitorenesduarte): fetch cloud environment
	return ""
}

type parseFun func(string) (string, bool)

// exec runs a command and validates its output using the parse function.
func (cfg fetchConfig) exec(name string, parse parseFun) string {
	out, err := cfg.execCommand(name)
	if err != nil {
		log.Debugf("Failed to execute command '%s': %s", name, err)
		return ""
	}
	return validate(name, string(out), parse)
}

// read reads a read and validates its content using the parse function.
func (cfg fetchConfig) read(name string, parse parseFun) string {
	out, err := cfg.readFile(name)
	if err != nil {
		log.Debugf("Failed to read file '%s': %s", name, err)
		return ""
	}
	return validate(name, string(out), parse)
}

// validate validates some output/content using the parse function.
// If the output/content is not valid, it is sanitized and returned.
func validate(in, out string, parse parseFun) string {
	parsed, ok := parse(out)
	if !ok {
		log.Debugf("Unexpected '%s' format: %s", in, out)
		return sanitize(out)
	}
	return parsed
}

// sanitize sanitizes some output/content by quoting it.
func sanitize(s string) string {
	return strconv.Quote(s)
}