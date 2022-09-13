package ingress

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"testing"
)

func TestEnvoyConfig_AddExposePort(t *testing.T) {
	config := NewEnvoyConfig()
	config.AddExposePort("test", 80, 8080)
	config.AddExposePort("test2", 80, 18080)
	marshal, err := yaml.Marshal(config)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(marshal))
}
