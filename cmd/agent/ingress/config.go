package ingress

import (
	"fmt"
	"github.com/pkg/errors"
)

type EnvoyConfig struct {
	port            map[int]int
	StaticResources *StaticResources `json:"static_resources" yaml:"static_resources"`
}
type StaticResources struct {
	Listeners []Listener `json:"listeners" yaml:"listeners"`
	Clusters  []Cluster  `json:"clusters" yaml:"clusters"`
}
type Listener struct {
	Name         string        `json:"name" yaml:"name"`
	Address      Address       `json:"address" yaml:"address"`
	FilterChains []FilterChain `json:"filter_chains" yaml:"filter_chains"`
}
type Filter struct {
	Name        string      `json:"name" yaml:"name"`
	TypedConfig TypedConfig `json:"typed_config" yaml:"typed_config"`
}
type TypedConfig struct {
	Type       string `json:"@type" yaml:"@type"`
	StatPrefix string `json:"stat_prefix" yaml:"stat_prefix"`
	Cluster    string `json:"cluster" yaml:"cluster"`
}
type FilterChain struct {
	Filters []Filter `json:"filters" yaml:"filters"`
}
type Address struct {
	SocketAddress SocketAddress `json:"socket_address" yaml:"socket_address"`
}
type SocketAddress struct {
	Address   string `json:"address" yaml:"address"`
	PortValue int    `json:"port_value" yaml:"port_value"`
}

type Cluster struct {
	Name            string         `json:"name" yaml:"name"`
	ConnectTimeout  string         `json:"connect_timeout" yaml:"connect_timeout"`
	Type            string         `json:"type" yaml:"type"`
	DnsLookupFamily string         `json:"dns_lookup_family" yaml:"dns_lookup_family"`
	LoadAssignment  LoadAssignment `json:"load_assignment" yaml:"load_assignment"`
}
type LoadAssignment struct {
	ClusterName string                   `json:"cluster_name" yaml:"cluster_name"`
	Endpoints   []LoadAssignmentEndpoint `json:"endpoints" yaml:"endpoints"`
}
type LoadAssignmentEndpoint struct {
	LbEndpoints []LbEndpoint `json:"lb_endpoints" yaml:"lb_endpoints"`
}
type LbEndpoint struct {
	Endpoint Endpoint `json:"endpoint" yaml:"endpoint"`
}
type Endpoint struct {
	Address Address `json:"address" yaml:"address"`
}

func NewEnvoyConfig() *EnvoyConfig {
	return &EnvoyConfig{
		port: map[int]int{},
		StaticResources: &StaticResources{
			Listeners: make([]Listener, 0),
			Clusters:  make([]Cluster, 0),
		},
	}
}
func (e *EnvoyConfig) AddExposePort(podName string, port int, exposePort int) error {
	if _, ok := e.port[exposePort]; ok {
		return errors.Errorf("port:%d is duplicate", exposePort)
	}
	listener := Listener{
		Name: fmt.Sprintf("listener_%s_%d", podName, port),
		Address: Address{
			SocketAddress: SocketAddress{
				Address:   "0.0.0.0",
				PortValue: exposePort,
			},
		},
		FilterChains: []FilterChain{
			{
				Filters: []Filter{
					{
						Name: "envoy.filters.network.tcp_proxy",
						TypedConfig: TypedConfig{
							Type:       "type.googleapis.com/envoy.extensions.filters.network.tcp_proxy.v3.TcpProxy",
							StatPrefix: "destination",
							Cluster:    fmt.Sprintf("cluster_%s_%d", podName, port),
						},
					},
				},
			},
		},
	}
	cluster := Cluster{
		Name:            fmt.Sprintf("cluster_%s_%d", podName, port),
		ConnectTimeout:  "30s",
		Type:            "LOGICAL_DNS",
		DnsLookupFamily: "V4_ONLY",
		LoadAssignment: LoadAssignment{
			ClusterName: fmt.Sprintf("cluster_%s_%d", podName, port),
			Endpoints: []LoadAssignmentEndpoint{
				{
					LbEndpoints: []LbEndpoint{
						{
							Endpoint{
								Address: Address{
									SocketAddress: SocketAddress{
										Address:   podName,
										PortValue: port,
									},
								},
							},
						},
					},
				},
			},
		},
	}
	e.StaticResources.Listeners = append(e.StaticResources.Listeners, listener)
	e.StaticResources.Clusters = append(e.StaticResources.Clusters, cluster)
	return nil
}
