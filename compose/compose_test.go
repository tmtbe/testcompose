package compose

import (
	"context"
	"os"
	"testing"
)

func Test_Compose(t *testing.T) {
	file, err := os.ReadFile("./test.yml")
	if err != nil {
		panic(err)
	}
	compose, err := NewCompose(file, "", "")
	if err != nil {
		panic(err)
	}
	ctx := context.Background()
	err = compose.PrepareNetworkAndVolumes(ctx)
	if err != nil {
		panic(err)
	}
	err = compose.StartPods(ctx)
	if err != nil {
		panic(err)
	}
}
