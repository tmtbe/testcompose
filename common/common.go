package common

const TpcDebug = "TPC_DEBUG"
const TpcName = "TPC_NAME"

const AgentContextPath = "/home/context/"
const AgentLogPath = "/home/logs/"
const AgentVolumePath = "/home/volumes/"
const EndPointAgentStart = "/start"
const EndPointAgentHealth = "/heath"
const EndPointAgentShutdown = "/shutdown"
const EndPointAgentTaskGroup = "/taskGroup"
const EndPointAgentStop = "/stop"
const EndPointAgentSwitchData = "/switch"
const EndPointAgentRestart = "/restart"
const EndPointAgentIngress = "/ingress"
const EndPointAgentInfo = "/info"
const ServerAgentPort = "80"
const ServerAgentEventBusPort = "7070"

const LabelSessionID = "SESSION_ID"
const LabelPodName = "POD_NAME"
const LabelContainerName = "CONTAINER_NAME"

const EnvHostContextPath = "HOST_CONTEXT_PATH"
const ConfigFileName = "compose.yaml"

const IngressVolumeName = "ingress"
const SystemLogVolumeName = "tpc_system_log"
const InitExitTimeOut = 60000
const ContainerNamePrefix = "tpc_"
