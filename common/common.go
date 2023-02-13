package common

const TpcDebug = "TPC_DEBUG"
const ImageAgent = "podcompose/agent"
const ImageIngress = "envoyproxy/envoy:v1.23-latest"
const ImagePause = "gcr.io/google_containers/pause:3.0"

const AgentContextPath = "/home/context/"
const AgentVolumePath = "/home/volumes/"
const EndPointAgentStart = "/start"
const EndPointAgentHealth = "/heath"
const EndPointAgentShutdown = "/shutdown"
const EndPointAgentStop = "/stop"
const EndPointAgentSwitchData = "/switch"
const EndPointAgentRestart = "/restart"
const EndPointAgentIngress = "/ingress"
const EndPointAgentInfo = "/info"
const ServerAgentPort = "80"
const ServerAgentEventBusPort = "7070"
const ServerAgentEventBusPath = "/_server_bus_"

const LabelSessionID = "SESSION_ID"
const LabelPodName = "POD_NAME"
const LabelContainerName = "CONTAINER_NAME"

const EnvHostContextPath = "HOST_CONTEXT_PATH"
const ConfigFileName = "compose.yml"

const IngressVolumeName = "ingress"
const DefaultSwitchDataName = "default"
const InitExitTimeOut = 60000
const ContainerNamePrefix = "tpc_"

var AgentAutoRemove = true
