package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"github.com/cenkalti/backoff/v4"
	"github.com/containerd/containerd/platforms"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"github.com/docker/go-connections/nat"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"podcompose/common"
	"podcompose/docker/wait"
	"podcompose/event"
	"strings"
	"time"
)

const (
	Bridge                 = "bridge"        // Bridge network name (as well as driver)
	DefaultNetwork         = "default_agent" // Default network name when bridge is not available
	Host                   = "DOCKER_HOST"
	PodContainerLabel      = "PodContainer"
	ComposeSessionID       = "ComposeSessionID"
	VolumeGroup            = "VolumeGroup"
	AgentType              = "AgentType"
	AgentTypeCleaner       = "cleaner"
	AgentTypeServer        = "server"
	AgentTypeVolume        = "volume"
	AgentTypeIngressVolume = "ingressVolume"
	AgentTypeIngress       = "ingress"
	AgentTypeSwitchData    = "switchData"
)

var (
	// Implement interfaces
	_                       Container = (*DockerContainer)(nil)
	ErrDuplicateMountTarget           = errors.New("duplicate mount target detected")
)

type DockerProvider struct {
	client         *client.Client
	hostCache      string
	defaultNetwork string // default container network
}

// NewDockerProvider creates a Docker provider with the EnvClient
func NewDockerProvider() (*DockerProvider, error) {
	opts := []client.Opt{client.FromEnv}
	c, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, err
	}
	_, err = c.Ping(context.TODO())
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	c.NegotiateAPIVersion(ctx)
	p := &DockerProvider{
		client: c,
	}
	err = p.setDefaultNetwork(ctx)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (p *DockerProvider) GetClient() *client.Client {
	return p.client
}

func (p *DockerProvider) GetDefaultNetwork() string {
	return p.defaultNetwork
}
func (p *DockerProvider) setDefaultNetwork(ctx context.Context) error {
	// Make sure that bridge network exists
	// In case it is disabled we will create agent_default network
	var err error
	p.defaultNetwork, err = getDefaultNetwork(ctx, p.client)
	if err != nil {
		return err
	}
	return nil
}

func (p *DockerProvider) CreateContainerAutoLabel(ctx context.Context, req ContainerRequest, sessionId string) (Container, error) {
	return p.CreateContainer(ctx, req, sessionId, true)
}

// CreateContainer fulfills a request for a container without starting it
func (p *DockerProvider) CreateContainer(ctx context.Context, req ContainerRequest, sessionId string, autoLabel bool) (Container, error) {
	var err error
	if req.Labels == nil {
		req.Labels = make(map[string]string)
	}
	if autoLabel {
		req.Labels[PodContainerLabel] = "true"
		req.Labels[ComposeSessionID] = sessionId
	}

	exposedPortSet, exposedPortMap, err := nat.ParsePortSpecs(req.ExposedPorts)
	if err != nil {
		return nil, err
	}

	var env []string
	for envKey, envVar := range req.Env {
		env = append(env, envKey+"="+envVar)
	}

	if req.Labels == nil {
		req.Labels = make(map[string]string)
	}

	var termSignal chan bool

	if err = req.Validate(); err != nil {
		return nil, err
	}

	var tag string
	var platform *specs.Platform

	tag = req.Image

	if req.ImagePlatform != "" {
		p, err := platforms.Parse(req.ImagePlatform)
		if err != nil {
			return nil, fmt.Errorf("invalid platform %s: %w", req.ImagePlatform, err)
		}
		platform = &p
	}

	var shouldPullImage bool

	if req.AlwaysPullImage {
		shouldPullImage = true // If requested always attempt to pull image
	} else {
		image, _, err := p.client.ImageInspectWithRaw(ctx, tag)
		if err != nil {
			if client.IsErrNotFound(err) {
				shouldPullImage = true
			} else {
				return nil, err
			}
		}
		if platform != nil && (image.Architecture != platform.Architecture || image.Os != platform.OS) {
			shouldPullImage = true
		}
	}

	if shouldPullImage {
		pullOpt := types.ImagePullOptions{
			Platform: req.ImagePlatform, // may be empty
		}

		if req.RegistryCred != "" {
			pullOpt.RegistryAuth = req.RegistryCred
		}

		event.Publish(&event.ContainerEventData{
			PodName:       req.Labels[common.LabelPodName],
			ContainerName: req.Labels[common.LabelContainerName],
			Type:          event.ContainerEventPullStartType,
			Id:            "",
			Name:          req.Name,
			Image:         req.Image,
		})

		if err := p.attemptToPullImage(ctx, tag, pullOpt); err != nil {
			event.Publish(&event.ContainerEventData{
				PodName:       req.Labels[common.LabelPodName],
				ContainerName: req.Labels[common.LabelContainerName],
				Type:          event.ContainerEventPullFailType,
				Id:            "",
				Name:          req.Name,
				Image:         req.Image,
			})
			return nil, err
		}

		event.Publish(&event.ContainerEventData{
			PodName:       req.Labels[common.LabelPodName],
			ContainerName: req.Labels[common.LabelContainerName],
			Type:          event.ContainerEventPullSuccessType,
			Id:            "",
			Name:          req.Name,
			Image:         req.Image,
		})
	}

	dockerInput := &container.Config{
		Entrypoint:   req.Entrypoint,
		Image:        tag,
		Env:          env,
		ExposedPorts: exposedPortSet,
		Labels:       req.Labels,
		Cmd:          req.Cmd,
		Hostname:     req.Hostname,
		User:         req.User,
		WorkingDir:   req.WorkingDir,
	}

	// prepare mounts
	mounts := mapToDockerMounts(req.Mounts)

	hostConfig := &container.HostConfig{
		PortBindings: exposedPortMap,
		Mounts:       mounts,
		Tmpfs:        req.Tmpfs,
		AutoRemove:   req.AutoRemove,
		Privileged:   req.Privileged,
		NetworkMode:  req.NetworkMode,
		Resources:    req.Resources,
		DNS:          req.DNS,
		CapAdd:       req.CapAdd,
		CapDrop:      req.CapDrop,
	}

	endpointConfigs := map[string]*network.EndpointSettings{}

	// #248: Docker allows only one network to be specified during container creation
	// If there is more than one network specified in the request container should be attached to them
	// once it is created. We will take a first network if any specified in the request and use it to create container
	if len(req.Networks) > 0 {
		attachContainerTo := req.Networks[0]

		nw, err := p.GetNetwork(ctx, NetworkRequest{
			Name: attachContainerTo,
		})
		if err == nil {
			endpointSetting := network.EndpointSettings{
				Aliases:   req.NetworkAliases[attachContainerTo],
				NetworkID: nw.ID,
			}
			endpointConfigs[attachContainerTo] = &endpointSetting
		}
	}

	networkingConfig := network.NetworkingConfig{
		EndpointsConfig: endpointConfigs,
	}

	resp, err := p.client.ContainerCreate(ctx, dockerInput, hostConfig, &networkingConfig, platform, req.Name)
	if err != nil {
		return nil, err
	}

	// #248: If there is more than one network specified in the request attach newly created container to them one by one
	if len(req.Networks) > 1 {
		for _, n := range req.Networks[1:] {
			nw, err := p.GetNetwork(ctx, NetworkRequest{
				Name: n,
			})
			if err == nil {
				endpointSetting := network.EndpointSettings{
					Aliases: req.NetworkAliases[n],
				}
				err = p.client.NetworkConnect(ctx, nw.ID, resp.ID, &endpointSetting)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	c := &DockerContainer{
		ID:                resp.ID,
		WaitingFor:        req.WaitingFor,
		Image:             tag,
		sessionId:         sessionId,
		provider:          p,
		terminationSignal: termSignal,
		stopProducer:      make(chan bool),
		logger:            Logger,
	}
	event.Publish(&event.ContainerEventData{
		PodName:       req.Labels[common.LabelPodName],
		ContainerName: req.Labels[common.LabelContainerName],
		Type:          event.ContainerEventCreatedType,
		Id:            c.ID,
		Name:          req.Name,
		Image:         req.Image,
	})
	return c, nil
}

// attemptToPullImage tries to pull the image while respecting the ctx cancellations.
// Besides, if the image cannot be pulled due to ErrorNotFound then no need to retry but terminate immediately.
func (p *DockerProvider) attemptToPullImage(ctx context.Context, tag string, pullOpt types.ImagePullOptions) error {
	var (
		err  error
		pull io.ReadCloser
	)
	err = backoff.Retry(func() error {
		pull, err = p.client.ImagePull(ctx, tag, pullOpt)
		if err != nil {
			if _, ok := err.(errdefs.ErrNotFound); ok {
				return backoff.Permanent(err)
			}
			Logger.Printf("Failed to pull image: %s, will retry", err)
			return err
		}
		return nil
	}, backoff.WithContext(backoff.NewExponentialBackOff(), ctx))
	if err != nil {
		return err
	}
	defer pull.Close()

	// download of docker image finishes at EOF of the pull request
	_, err = ioutil.ReadAll(pull)
	return err
}

// Health measure the healthiness of the provider. Right now we leverage the
// docker-client ping endpoint to see if the daemon is reachable.
func (p *DockerProvider) Health(ctx context.Context) (err error) {
	_, err = p.client.Ping(ctx)
	return
}

// RunContainer takes a RequestContainer as input, and it runs a container via the docker sdk
func (p *DockerProvider) RunContainer(ctx context.Context, req ContainerRequest, sessionId string) (Container, error) {
	c, err := p.CreateContainerAutoLabel(ctx, req, sessionId)
	if err != nil {
		return nil, err
	}
	if err := c.Start(ctx, req); err != nil {
		return c, fmt.Errorf("%w: could not start container", err)
	}

	return c, nil
}

// daemonHost gets the host or ip of the Docker daemon where ports are exposed on
// Warning: this is based on your Docker host setting. Will fail if using an SSH tunnel
// You can use the "Host" env variable to set this yourself
func (p *DockerProvider) daemonHost(ctx context.Context) (string, error) {
	if p.hostCache != "" {
		return p.hostCache, nil
	}

	host, exists := os.LookupEnv(Host)
	if exists {
		p.hostCache = host
		return p.hostCache, nil
	}

	// infer from Docker host
	urlParse, err := url.Parse(p.client.DaemonHost())
	if err != nil {
		return "", err
	}

	switch urlParse.Scheme {
	case "http", "https", "tcp":
		p.hostCache = urlParse.Hostname()
	case "unix", "npipe":
		if inAContainer() {
			ip, err := p.GetGatewayIP(ctx)
			if err != nil {
				// fallback to getDefaultGatewayIP
				ip, err = getDefaultGatewayIP()
				if err != nil {
					ip = "localhost"
				}
			}
			p.hostCache = ip
		} else {
			p.hostCache = "localhost"
		}
	default:
		return "", errors.New("Could not determine host through env or docker host")
	}

	return p.hostCache, nil
}

// CreateNetwork returns the object representing a new network identified by its name
func (p *DockerProvider) CreateNetwork(ctx context.Context, req NetworkRequest, sessionId string) (Network, error) {
	var err error
	if req.Labels == nil {
		req.Labels = make(map[string]string)
	}

	req.Labels[PodContainerLabel] = "true"
	req.Labels[ComposeSessionID] = sessionId

	nc := types.NetworkCreate{
		Driver:         req.Driver,
		CheckDuplicate: req.CheckDuplicate,
		Internal:       req.Internal,
		EnableIPv6:     req.EnableIPv6,
		Attachable:     req.Attachable,
		Labels:         req.Labels,
	}
	var termSignal chan bool
	response, err := p.client.NetworkCreate(ctx, req.Name, nc)
	if err != nil {
		return &DockerNetwork{}, err
	}

	n := &DockerNetwork{
		ID:                response.ID,
		Driver:            req.Driver,
		Name:              req.Name,
		terminationSignal: termSignal,
		provider:          p,
	}

	return n, nil
}

// GetNetwork returns the object representing the network identified by its name
func (p *DockerProvider) GetNetwork(ctx context.Context, req NetworkRequest) (types.NetworkResource, error) {
	networkResource, err := p.client.NetworkInspect(ctx, req.Name, types.NetworkInspectOptions{
		Verbose: true,
	})
	if err != nil {
		return types.NetworkResource{}, err
	}

	return networkResource, err
}

func (p *DockerProvider) GetGatewayIP(ctx context.Context) (string, error) {
	// Use a default network as defined in the DockerProvider
	if p.defaultNetwork == "" {
		var err error
		p.defaultNetwork, err = getDefaultNetwork(ctx, p.client)
		if err != nil {
			return "", err
		}
	}
	nw, err := p.GetNetwork(ctx, NetworkRequest{Name: p.defaultNetwork})
	if err != nil {
		return "", err
	}

	var ip string
	for _, config := range nw.IPAM.Config {
		if config.Gateway != "" {
			ip = config.Gateway
			break
		}
	}
	if ip == "" {
		return "", errors.New("Failed to get gateway IP from network settings")
	}

	return ip, nil
}
func (p *DockerProvider) CreateVolume(ctx context.Context, name string, sessionId string, volumeGroup string) (types.Volume, error) {
	return p.client.VolumeCreate(ctx, volume.VolumeCreateBody{
		Driver: "local",
		Name:   name + "_" + sessionId,
		Labels: map[string]string{
			PodContainerLabel: "true",
			ComposeSessionID:  sessionId,
			VolumeGroup:       volumeGroup,
		},
	})
}

func (p *DockerProvider) RemoveVolume(ctx context.Context, volumeName string, sessionId string, force bool) error {
	if !strings.HasSuffix(volumeName, sessionId) {
		volumeName = volumeName + "_" + sessionId
	}
	zap.L().Sugar().Debugf("remove volume : %s", volumeName)
	return p.client.VolumeRemove(ctx, volumeName, force)
}

func (p *DockerProvider) RemoveNetwork(ctx context.Context, networkID string) error {
	zap.L().Sugar().Debugf("remove network : %s", networkID)
	return p.client.NetworkRemove(ctx, networkID)
}

func (p *DockerProvider) ClearWithSession(ctx context.Context, sessionId string) {
	if sessionId == "" {
		return
	}
	zap.L().Sugar().Infof("sessionId:%s", sessionId)
	filtersJSON := fmt.Sprintf(`{"label":{"%s":"true"}}`, PodContainerLabel)
	fj, _ := filters.FromJSON(filtersJSON)
	// clear container
	containerList, err := p.client.ContainerList(ctx, types.ContainerListOptions{
		All:     true,
		Filters: fj,
	})
	if err == nil {
		for _, c := range containerList {
			if c.Labels[ComposeSessionID] == sessionId {
				if c.Labels[AgentType] == AgentTypeCleaner {
					continue
				}
				zap.L().Sugar().Infof("remove container:%s", c.ID)
				err = p.client.ContainerRemove(ctx, c.ID, types.ContainerRemoveOptions{
					RemoveVolumes: true,
					RemoveLinks:   false,
					Force:         true,
				})
				if err != nil {
					zap.L().Sugar().Error(err)
				}
			}
		}
	}
	// clear volume
	volumeList, err := p.client.VolumeList(ctx, fj)
	if err == nil {
		for _, v := range volumeList.Volumes {
			if v.Labels[ComposeSessionID] == sessionId {
				zap.L().Sugar().Infof("remove volume:%s", v.Name)
				err = p.client.VolumeRemove(ctx, v.Name, true)
				if err != nil {
					zap.L().Sugar().Error(err)
				}
			}
		}
	}
	// clear network
	networkList, err := p.client.NetworkList(ctx, types.NetworkListOptions{
		Filters: fj,
	})
	if err == nil {
		for _, n := range networkList {
			if n.Labels[ComposeSessionID] == sessionId {
				zap.L().Sugar().Infof("remove network:%s", n.Name)
				err = p.client.NetworkRemove(ctx, n.ID)
				if err != nil {
					zap.L().Sugar().Error(err)
				}
			}
		}
	}
}

func (p *DockerProvider) RemoveContainer(ctx context.Context, id string) error {
	zap.L().Sugar().Debugf("remove container : %s", id)
	inspect, err := p.ContainerInspect(ctx, id)
	if err == nil {
		event.Publish(&event.ContainerEventData{
			PodName:       inspect.Config.Labels[common.LabelPodName],
			ContainerName: inspect.Config.Labels[common.LabelContainerName],
			Type:          event.ContainerEventRemoveType,
			Id:            inspect.ID,
			Name:          inspect.Name,
			Image:         inspect.Image,
		})
	}
	return p.client.ContainerRemove(ctx, id, types.ContainerRemoveOptions{
		Force: true,
	})
}

func (p *DockerProvider) FindAllPodContainers(ctx context.Context) ([]types.Container, error) {
	filtersJSON := fmt.Sprintf(`{"label":{"%s":"true"}}`, PodContainerLabel)
	fj, _ := filters.FromJSON(filtersJSON)
	list, err := p.client.ContainerList(ctx, types.ContainerListOptions{
		Filters: fj,
		All:     true,
	})
	if err != nil {
		return nil, err
	}
	return list, nil
}

func (p *DockerProvider) FindAllContainersWithSessionId(ctx context.Context, sessionId string) ([]types.Container, error) {
	filtersJSON := fmt.Sprintf(`{"label":{"%s":%s}}`, ComposeSessionID, sessionId)
	fj, _ := filters.FromJSON(filtersJSON)
	list, err := p.client.ContainerList(ctx, types.ContainerListOptions{
		Filters: fj,
		All:     true,
	})
	if err != nil {
		return nil, err
	}
	return list, nil
}

func (p *DockerProvider) FindContainers(ctx context.Context, sessionId string) ([]types.Container, error) {
	filtersJSON := fmt.Sprintf(`{"label":{"%s":"true"}}`, PodContainerLabel)
	fj, _ := filters.FromJSON(filtersJSON)
	list, err := p.client.ContainerList(ctx, types.ContainerListOptions{
		Filters: fj,
	})
	if err != nil {
		return nil, err
	}
	result := make([]types.Container, 0)
	for _, c := range list {
		if c.Labels[ComposeSessionID] == sessionId {
			result = append(result, c)
		}
	}
	return result, nil
}

func (p *DockerProvider) FindAllVolumes(ctx context.Context) ([]*types.Volume, error) {
	filtersJSON := fmt.Sprintf(`{"label":{"%s":"true"}}`, PodContainerLabel)
	fj, _ := filters.FromJSON(filtersJSON)
	volumeListOKBody, err := p.client.VolumeList(ctx, fj)
	if err != nil {
		return nil, err
	}
	return volumeListOKBody.Volumes, nil
}

func (p *DockerProvider) FindAllVolumesWithSessionId(ctx context.Context, sessionId string) ([]*types.Volume, error) {
	filtersJSON := fmt.Sprintf(`{"label":{"%s":%s}}`, ComposeSessionID, sessionId)
	fj, _ := filters.FromJSON(filtersJSON)
	volumeListOKBody, err := p.client.VolumeList(ctx, fj)
	if err != nil {
		return nil, err
	}
	return volumeListOKBody.Volumes, nil
}

func (p *DockerProvider) FindAllNetworks(ctx context.Context) ([]types.NetworkResource, error) {
	filtersJSON := fmt.Sprintf(`{"label":{"%s":"true"}}`, PodContainerLabel)
	fj, _ := filters.FromJSON(filtersJSON)
	networks, err := p.client.NetworkList(ctx, types.NetworkListOptions{
		Filters: fj,
	})
	if err != nil {
		return nil, err
	}
	return networks, nil
}

func (p *DockerProvider) FindContainerByName(ctx context.Context, name string) (*types.Container, error) {
	filtersJSON := fmt.Sprintf(`{"name":{"%s":true}}`, name)
	fj, _ := filters.FromJSON(filtersJSON)
	list, err := p.client.ContainerList(ctx, types.ContainerListOptions{
		Filters: fj,
	})
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, errors.Errorf("not found container name is %s", name)
	} else {
		return &list[0], nil
	}
}

func (p *DockerProvider) State(ctx context.Context, id string) (*types.ContainerState, error) {
	inspect, err := p.client.ContainerInspect(ctx, id)
	if err != nil {
		return nil, err
	}
	if err != nil {
		return inspect.State, err
	}
	return inspect.State, nil
}

func (p *DockerProvider) ContainerInspect(ctx context.Context, id string) (types.ContainerJSON, error) {
	return p.GetClient().ContainerInspect(ctx, id)
}

func getDefaultNetwork(ctx context.Context, cli *client.Client) (string, error) {
	// Get list of available networks
	networkResources, err := cli.NetworkList(ctx, types.NetworkListOptions{})
	if err != nil {
		return "", err
	}

	agentNetwork := DefaultNetwork

	agentNetworkExists := false

	// clean empty network
	for _, net := range networkResources {
		if len(net.Containers) == 0 {
			if net.Labels[PodContainerLabel] == "true" {
				_ = cli.NetworkRemove(ctx, net.ID)
			}
		}
	}

	for _, net := range networkResources {
		if net.Name == Bridge {
			return Bridge, nil
		}

		if net.Name == agentNetwork {
			agentNetworkExists = true
		}
	}

	// Create a bridge network for the container communications
	if !agentNetworkExists {
		_, err = cli.NetworkCreate(ctx, agentNetwork, types.NetworkCreate{
			Driver:     Bridge,
			Attachable: true,
		})

		if err != nil {
			return "", err
		}
	}

	return agentNetwork, nil
}

func inAContainer() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	return false
}
func getDefaultGatewayIP() (string, error) {
	cmd := exec.Command("sh", "-c", "ip route|awk '/default/ { print $3 }'")
	stdout, err := cmd.Output()
	if err != nil {
		return "", errors.New("Failed to detect docker host")
	}
	ip := strings.TrimSpace(string(stdout))
	if len(ip) == 0 {
		return "", errors.New("Failed to parse default gateway IP")
	}
	return string(ip), nil
}

// DockerContainer represents a container started using Docker
type DockerContainer struct {
	// Container ID from Docker
	ID                string
	WaitingFor        wait.Strategy
	Image             string
	provider          *DockerProvider
	sessionId         string
	terminationSignal chan bool
	consumers         []LogConsumer
	raw               *types.ContainerJSON
	stopProducer      chan bool
	logger            Logging
}

func NewDockerContainer(id string, image string, provider *DockerProvider, sessionId string, terminationSignal chan bool) *DockerContainer {
	return &DockerContainer{
		ID:                id,
		WaitingFor:        nil,
		Image:             image,
		provider:          provider,
		sessionId:         sessionId,
		terminationSignal: terminationSignal,
		stopProducer:      make(chan bool),
		logger:            Logger,
	}
}

func (c *DockerContainer) GetContainerID() string {
	return c.ID
}

// Endpoint gets proto://host:port string for the first exposed port
// Will returns just host:port if proto is ""
func (c *DockerContainer) Endpoint(ctx context.Context, proto string) (string, error) {
	ports, err := c.Ports(ctx)
	if err != nil {
		return "", err
	}

	// get first port
	var firstPort nat.Port
	for p := range ports {
		firstPort = p
		break
	}

	return c.PortEndpoint(ctx, firstPort, proto)
}

// PortEndpoint gets proto://host:port string for the given exposed port
// Will returns just host:port if proto is ""
func (c *DockerContainer) PortEndpoint(ctx context.Context, port nat.Port, proto string) (string, error) {
	host, err := c.Host(ctx)
	if err != nil {
		return "", err
	}

	outerPort, err := c.MappedPort(ctx, port)
	if err != nil {
		return "", err
	}

	protoFull := ""
	if proto != "" {
		protoFull = fmt.Sprintf("%s://", proto)
	}

	return fmt.Sprintf("%s%s:%s", protoFull, host, outerPort.Port()), nil
}

// Host gets host (ip or name) of the docker daemon where the container port is exposed
// Warning: this is based on your Docker host setting. Will fail if using an SSH tunnel
// You can use the "Host" env variable to set this yourself
func (c *DockerContainer) Host(ctx context.Context) (string, error) {
	host, err := c.provider.daemonHost(ctx)
	if err != nil {
		return "", err
	}
	return host, nil
}

// MappedPort gets externally mapped port for a container port
func (c *DockerContainer) MappedPort(ctx context.Context, port nat.Port) (nat.Port, error) {
	inspect, err := c.inspectContainer(ctx)
	if err != nil {
		return "", err
	}
	if inspect.ContainerJSONBase.HostConfig.NetworkMode == "host" {
		return port, nil
	}
	ports, err := c.Ports(ctx)
	if err != nil {
		return "", err
	}

	for k, p := range ports {
		if k.Port() != port.Port() {
			continue
		}
		if port.Proto() != "" && k.Proto() != port.Proto() {
			continue
		}
		if len(p) == 0 {
			continue
		}
		return nat.NewPort(k.Proto(), p[0].HostPort)
	}

	return "", errors.New("port not found")
}

// Ports gets the exposed ports for the container.
func (c *DockerContainer) Ports(ctx context.Context) (nat.PortMap, error) {
	inspect, err := c.inspectContainer(ctx)
	if err != nil {
		return nil, err
	}
	return inspect.NetworkSettings.Ports, nil
}

// SessionID gets the current session id
func (c *DockerContainer) SessionID() string {
	return c.sessionId
}

// Start will start an already created container
func (c *DockerContainer) Start(ctx context.Context, req ContainerRequest) error {
	event.Publish(&event.ContainerEventData{
		PodName:       req.Labels[common.LabelPodName],
		ContainerName: req.Labels[common.LabelContainerName],
		Name:          req.Name,
		Image:         req.Image,
		Type:          event.ContainerEventStartType,
		Id:            c.ID,
	})
	shortID := c.ID[:12]
	c.logger.Printf("Starting container id: %s image: %s", shortID, c.Image)

	if err := c.provider.client.ContainerStart(ctx, c.ID, types.ContainerStartOptions{}); err != nil {
		return err
	}

	// if a Wait Strategy has been specified, wait before returning
	if c.WaitingFor != nil {
		c.logger.Printf("Waiting for container id: %s image: %s", shortID, c.Image)
		if err := c.WaitingFor.WaitUntilReady(ctx, c); err != nil {
			return err
		}
	}
	state, err := c.State(ctx)
	if err != nil || state.Running == false {
		c.logger.Printf("Container is removed id: %s image: %s", shortID, c.Image)
		event.Publish(&event.ContainerEventData{
			PodName:       req.Labels[common.LabelPodName],
			ContainerName: req.Labels[common.LabelContainerName],
			Name:          req.Name,
			Type:          event.ContainerEventRemoveType,
			Id:            c.ID,
			Image:         c.Image,
		})
	} else {
		c.logger.Printf("Container is ready id: %s image: %s", shortID, c.Image)
		event.Publish(&event.ContainerEventData{
			PodName:       req.Labels[common.LabelPodName],
			ContainerName: req.Labels[common.LabelContainerName],
			Name:          req.Name,
			Type:          event.ContainerEventReadyType,
			Id:            c.ID,
			Image:         c.Image,
		})
	}
	return nil
}

// Stop will stop an already started container
//
// In case the container fails to stop
// gracefully within a time frame specified by the timeout argument,
// it is forcefully terminated (killed).
//
// If the timeout is nil, the container's StopTimeout value is used, if set,
// otherwise the engine default. A negative timeout value can be specified,
// meaning no timeout, i.e. no forceful termination is performed.
func (c *DockerContainer) Stop(ctx context.Context, timeout *time.Duration) error {
	shortID := c.ID[:12]
	c.logger.Printf("Stopping container id: %s image: %s", shortID, c.Image)

	if err := c.provider.client.ContainerStop(ctx, c.ID, timeout); err != nil {
		return err
	}

	c.logger.Printf("Container is stopped id: %s image: %s", shortID, c.Image)

	return nil
}

// Terminate is used to kill the container. It is usually triggered by as defer function.
func (c *DockerContainer) Terminate(ctx context.Context) error {
	select {
	// close agent if it was created
	case c.terminationSignal <- true:
	default:
	}
	err := c.provider.client.ContainerRemove(ctx, c.GetContainerID(), types.ContainerRemoveOptions{
		RemoveVolumes: true,
		Force:         true,
	})
	if err != nil {
		return err
	}

	if err := c.provider.client.Close(); err != nil {
		return err
	}
	return nil
}

// update container raw info
func (c *DockerContainer) inspectRawContainer(ctx context.Context) (*types.ContainerJSON, error) {
	inspect, err := c.provider.client.ContainerInspect(ctx, c.ID)
	if err != nil {
		return nil, err
	}
	c.raw = &inspect
	return c.raw, nil
}

func (c *DockerContainer) inspectContainer(ctx context.Context) (*types.ContainerJSON, error) {
	inspect, err := c.provider.client.ContainerInspect(ctx, c.ID)
	if err != nil {
		return nil, err
	}

	return &inspect, nil
}

// Logs will fetch both STDOUT and STDERR from the current container. Returns a
// ReadCloser and leaves it up to the caller to extract what it wants.
func (c *DockerContainer) Logs(ctx context.Context) (io.ReadCloser, error) {
	options := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	}
	return c.provider.client.ContainerLogs(ctx, c.ID, options)
}

// FollowOutput adds a LogConsumer to be sent logs from the container's
// STDOUT and STDERR
func (c *DockerContainer) FollowOutput(consumer LogConsumer) {
	if c.consumers == nil {
		c.consumers = []LogConsumer{
			consumer,
		}
	} else {
		c.consumers = append(c.consumers, consumer)
	}
}

// Name gets the name of the container.
func (c *DockerContainer) Name(ctx context.Context) (string, error) {
	inspect, err := c.inspectContainer(ctx)
	if err != nil {
		return "", err
	}
	return inspect.Name, nil
}

// State returns container's running state
func (c *DockerContainer) State(ctx context.Context) (*types.ContainerState, error) {
	inspect, err := c.inspectRawContainer(ctx)
	if err != nil {
		return c.raw.State, err
	}
	return inspect.State, nil
}

// Networks gets the names of the networks the container is attached to.
func (c *DockerContainer) Networks(ctx context.Context) ([]string, error) {
	inspect, err := c.inspectContainer(ctx)
	if err != nil {
		return []string{}, err
	}

	networks := inspect.NetworkSettings.Networks

	n := []string{}

	for k := range networks {
		n = append(n, k)
	}

	return n, nil
}

// ContainerIP gets the IP address of the primary network within the container.
func (c *DockerContainer) ContainerIP(ctx context.Context) (string, error) {
	inspect, err := c.inspectContainer(ctx)
	if err != nil {
		return "", err
	}

	return inspect.NetworkSettings.IPAddress, nil
}

// NetworkAliases gets the aliases of the container for the networks it is attached to.
func (c *DockerContainer) NetworkAliases(ctx context.Context) (map[string][]string, error) {
	inspect, err := c.inspectContainer(ctx)
	if err != nil {
		return map[string][]string{}, err
	}

	networks := inspect.NetworkSettings.Networks

	a := map[string][]string{}

	for k := range networks {
		a[k] = networks[k].Aliases
	}

	return a, nil
}

func (c *DockerContainer) Exec(ctx context.Context, cmd []string) (int, error) {
	cli := c.provider.client
	response, err := cli.ContainerExecCreate(ctx, c.ID, types.ExecConfig{
		Cmd:    cmd,
		Detach: false,
	})
	if err != nil {
		return 0, err
	}

	err = cli.ContainerExecStart(ctx, response.ID, types.ExecStartCheck{
		Detach: false,
	})
	if err != nil {
		return 0, err
	}

	var exitCode int
	for {
		execResp, err := cli.ContainerExecInspect(ctx, response.ID)
		if err != nil {
			return 0, err
		}

		if !execResp.Running {
			exitCode = execResp.ExitCode
			break
		}

		time.Sleep(100 * time.Millisecond)
	}

	return exitCode, nil
}

type FileFromContainer struct {
	underlying *io.ReadCloser
	tarreader  *tar.Reader
}

func (fc *FileFromContainer) Read(b []byte) (int, error) {
	return (*fc.tarreader).Read(b)
}

func (fc *FileFromContainer) Close() error {
	return (*fc.underlying).Close()
}

func (c *DockerContainer) CopyFileFromContainer(ctx context.Context, filePath string) (io.ReadCloser, error) {
	r, _, err := c.provider.client.CopyFromContainer(ctx, c.ID, filePath)
	if err != nil {
		return nil, err
	}
	tarReader := tar.NewReader(r)

	// if we got here we have exactly one file in the TAR-stream
	// so we advance the index by one so the next call to Read will start reading it
	_, err = tarReader.Next()
	if err != nil {
		return nil, err
	}

	ret := &FileFromContainer{
		underlying: &r,
		tarreader:  tarReader,
	}

	return ret, nil
}

func (c *DockerContainer) CopyFileToContainer(ctx context.Context, hostFilePath string, containerFilePath string, fileMode int64) error {
	fileContent, err := ioutil.ReadFile(hostFilePath)
	if err != nil {
		return err
	}
	return c.CopyToContainer(ctx, fileContent, containerFilePath, fileMode)
}

// CopyToContainer copies fileContent data to a file in container
func (c *DockerContainer) CopyToContainer(ctx context.Context, fileContent []byte, containerFilePath string, fileMode int64) error {
	buffer := &bytes.Buffer{}

	tw := tar.NewWriter(buffer)
	defer tw.Close()

	hdr := &tar.Header{
		Name: filepath.Base(containerFilePath),
		Mode: fileMode,
		Size: int64(len(fileContent)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	if _, err := tw.Write(fileContent); err != nil {
		return err
	}

	return c.provider.client.CopyToContainer(ctx, c.ID, filepath.Dir(containerFilePath), buffer, types.CopyToContainerOptions{})
}

// StartLogProducer will start a concurrent process that will continuously read logs
// from the container and will send them to each added LogConsumer
func (c *DockerContainer) StartLogProducer(ctx context.Context) error {
	go func() {
		since := ""
		// if the socket is closed we will make additional logs request with updated Since timestamp
	BEGIN:
		options := types.ContainerLogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Follow:     true,
			Since:      since,
		}

		ctx, cancel := context.WithTimeout(ctx, time.Second*5)
		defer cancel()

		r, err := c.provider.client.ContainerLogs(ctx, c.GetContainerID(), options)
		if err != nil {
			// if we can't get the logs, panic, we can't return an error to anything
			// from within this goroutine
			panic(err)
		}

		for {
			select {
			case <-c.stopProducer:
				err := r.Close()
				if err != nil {
					// we can't close the read closer, this should never happen
					panic(err)
				}
				return
			default:
				h := make([]byte, 8)
				_, err := r.Read(h)
				if err != nil {
					// proper type matching requires https://go-review.googlesource.com/c/go/+/250357/ (go 1.16)
					if strings.Contains(err.Error(), "use of closed network connection") {
						now := time.Now()
						since = fmt.Sprintf("%d.%09d", now.Unix(), int64(now.Nanosecond()))
						goto BEGIN
					}
					// this explicitly ignores errors
					// because we want to keep procesing even if one of our reads fails
					continue
				}

				count := binary.BigEndian.Uint32(h[4:])
				if count == 0 {
					continue
				}
				logType := h[0]
				if logType > 2 {
					_, _ = fmt.Fprintf(os.Stderr, "received invalid log type: %d", logType)
					// sometimes docker returns logType = 3 which is an undocumented log type, so treat it as stdout
					logType = 1
				}

				// a map of the log type --> int representation in the header, notice the first is blank, this is stdin, but the go docker client doesn't allow following that in logs
				logTypes := []string{"", StdoutLog, StderrLog}

				b := make([]byte, count)
				_, err = r.Read(b)
				if err != nil {
					// TODO: add-logger: use logger to log out this error
					_, _ = fmt.Fprintf(os.Stderr, "error occurred reading log with known length %s", err.Error())
					continue
				}
				for _, c := range c.consumers {
					c.Accept(Log{
						LogType: logTypes[logType],
						Content: b,
					})
				}
			}
		}
	}()

	return nil
}

// StopLogProducer will stop the concurrent process that is reading logs
// and sending them to each added LogConsumer
func (c *DockerContainer) StopLogProducer() error {
	c.stopProducer <- true
	return nil
}

// DockerNetwork represents a network started using Docker
type DockerNetwork struct {
	ID                string // Network ID from Docker
	Driver            string
	Name              string
	provider          *DockerProvider
	terminationSignal chan bool
}

// Remove is used to remove the network. It is usually triggered by as defer function.
func (n *DockerNetwork) Remove(ctx context.Context) error {
	select {
	// close agent if it was created
	case n.terminationSignal <- true:
	default:
	}
	return n.provider.client.NetworkRemove(ctx, n.ID)
}
