package common

type Info struct {
	SessionId   string
	IsReady     bool
	VolumeInfos []VolumeInfo
	PodInfos    []PodInfo
	Ingresses   []IngressInfo
}
type IngressInfo struct {
	ServiceName string
	ServicePort string
	HostPort    string
}
type PodInfo struct {
	Name           string
	ContainerInfos []ContainerInfo
}
type ContainerInfo struct {
	Name        string
	ContainerId string
	State       string
	Image       string
	Created     int64
}
type VolumeInfo struct {
	Name     string
	VolumeId string
}
