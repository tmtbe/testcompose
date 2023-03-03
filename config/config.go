package config

import (
	"encoding/json"
	"go.uber.org/zap"
)

var ComposeConfig Config

type Config struct {
	Image Image `json:"image" yaml:"image"`
}
type Image struct {
	Agent   string `json:"agent" yaml:"agent"`
	Ingress string `json:"ingress" yaml:"ingress"`
	Pause   string `json:"pause" yaml:"pause"`
}

func init() {
	ComposeConfig.Image = Image{
		Agent:   "testmesh/compose-agent",
		Ingress: "envoyproxy/envoy:v1.23-latest",
		Pause:   "gcr.io/google_containers/pause:3.0",
	}
}

func SetConfigJson(configJson string) error {
	zap.L().Info("Reset config: " + configJson)
	var composeConfig Config
	err := json.Unmarshal([]byte(configJson), &composeConfig)
	if err != nil {
		return err
	}
	if composeConfig.Image.Pause != "" {
		ComposeConfig.Image.Pause = composeConfig.Image.Pause
	}
	if composeConfig.Image.Ingress != "" {
		ComposeConfig.Image.Ingress = composeConfig.Image.Ingress
	}
	if composeConfig.Image.Agent != "" {
		ComposeConfig.Image.Agent = composeConfig.Image.Agent
	}
	return nil
}

func GetConfigJson() string {
	marshal, _ := json.Marshal(ComposeConfig)
	return string(marshal)
}
