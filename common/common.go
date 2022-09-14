package common

const ImageAgent = "podcompose/agent"
const ImageIngress = "envoyproxy/envoy:v1.23-latest"
const ImagePause = "gcr.io/google_containers/pause:3.0"

const AgentContextPath = "/home/context/"
const AgentVolumePath = "/home/volumes/"
const EndPointAgentHealth = "/heath"
const EndPointAgentShutdown = "/shutdown"
const EndPointAgentSwitchData = "/switch"
const EndPointAgentRestart = "/restart"
const EndPointAgentIngress = "/ingress"
const ServerAgentPort = "8080"

const LabelSessionID = "SESSION_ID"
const LabelPodName = "POD_NAME"

const EnvHostContextPath = "HOST_CONTEXT_PATH"
const ConfigFileName = "compose.yml"

const IngressVolumeName = "ingress"
const DefaultSwitchDataName = "default"
const AgentAutoRemove = true
const InitExitTimeOut = 60000
const ContainerNamePrefix = "tpc_"
