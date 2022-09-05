package docker

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/go-connections/nat"
	"github.com/pkg/errors"
	"io"
	"podcompose/docker/wait"
	"time"
)

// ContainerRequest represents the parameters used to get a running container
type ContainerRequest struct {
	Image           string
	Entrypoint      []string
	Env             map[string]string
	ExposedPorts    []string // allow specifying protocol info
	Cmd             []string
	Labels          map[string]string
	Mounts          ContainerMounts
	Tmpfs           map[string]string
	RegistryCred    string
	WaitingFor      wait.Strategy
	Name            string // for specifying container name
	Hostname        string
	Privileged      bool                // for starting privileged container
	Networks        []string            // for specifying network names
	NetworkAliases  map[string][]string // for specifying network aliases
	NetworkMode     container.NetworkMode
	Resources       container.Resources
	DNS             []string
	CapAdd          strslice.StrSlice // List of kernel capabilities to add to the container
	CapDrop         strslice.StrSlice // List of kernel capabilities to remove from the container
	User            string            // for specifying uid:gid
	AutoRemove      bool              // if set to true, the container will be removed from the host when stopped
	AlwaysPullImage bool              // Always pull image
	ImagePlatform   string            // ImagePlatform describes the platform which the image runs on.
}

// Container allows getting info about and controlling a single container instance
type Container interface {
	GetContainerID() string                                         // get the container id from the provider
	Endpoint(context.Context, string) (string, error)               // get proto://ip:port string for the first exposed port
	PortEndpoint(context.Context, nat.Port, string) (string, error) // get proto://ip:port string for the given exposed port
	Host(context.Context) (string, error)                           // get host where the container port is exposed
	MappedPort(context.Context, nat.Port) (nat.Port, error)         // get externally mapped port for a container port
	Ports(context.Context) (nat.PortMap, error)                     // get all exposed ports
	SessionID() string                                              // get session id
	Start(context.Context) error                                    // start the container
	Stop(context.Context, *time.Duration) error                     // stop the container
	Terminate(context.Context) error                                // terminate the container
	Logs(context.Context) (io.ReadCloser, error)                    // Get logs of the container
	FollowOutput(LogConsumer)
	StartLogProducer(context.Context) error
	StopLogProducer() error
	Name(context.Context) (string, error)                        // get container name
	State(context.Context) (*types.ContainerState, error)        // returns container's running state
	Networks(context.Context) ([]string, error)                  // get container networks
	NetworkAliases(context.Context) (map[string][]string, error) // get container network aliases for a network
	Exec(ctx context.Context, cmd []string) (int, error)
	ContainerIP(context.Context) (string, error) // get container ip
	CopyToContainer(ctx context.Context, fileContent []byte, containerFilePath string, fileMode int64) error
	CopyFileToContainer(ctx context.Context, hostFilePath string, containerFilePath string, fileMode int64) error
	CopyFileFromContainer(ctx context.Context, filePath string) (io.ReadCloser, error)
}

// Validate ensures that the ContainerRequest does not have invalid parameters configured to it
// ex. make sure you are not specifying both an image as well as a context
func (c *ContainerRequest) Validate() error {
	validationMethods := []func() error{
		c.validateImage,
		c.validateMounts,
	}

	var err error
	for _, validationMethod := range validationMethods {
		err = validationMethod()
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *ContainerRequest) validateImage() error {
	if c.Image == "" {
		return errors.New("you must specify an image")
	}
	return nil
}

func (c *ContainerRequest) validateMounts() error {
	targets := make(map[string]bool, len(c.Mounts))

	for idx := range c.Mounts {
		m := c.Mounts[idx]
		targetPath := m.Target.Target()
		if targets[targetPath] {
			return fmt.Errorf("%w: %s", ErrDuplicateMountTarget, targetPath)
		} else {
			targets[targetPath] = true
		}
	}
	return nil
}
