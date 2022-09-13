package common

const AgentContextPath = "/home/context/"
const AgentVolumePath = "/home/volumes/"
const AgentHealthEndPoint = "/heath"
const AgentShutdownEndPoint = "/shutdown"
const AgentSwitchDataEndPoint = "/switch"
const AgentRestartEndPoint = "/restart"
const AgentIngressEndPoint = "/ingress"
const AgentPort = "8080"
const AgentSessionID = "SESSION_ID"
const HostContextPath = "HOST_CONTEXT_PATH"
const ConfigFileName = "compose.yml"
const AgentImage = "podcompose/agent"
const IngressImage = "envoyproxy/envoy:v1.23-latest"
const IngressVolumeName = "ingress"
const DefaultSwitchDataName = "default"
const AgentAutoRemove = true
