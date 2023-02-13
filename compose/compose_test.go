package compose

import (
	"context"
	"github.com/google/uuid"
	"os"
	"testing"
)

func Test_Compose(t *testing.T) {
	sessionUUID, _ := uuid.NewUUID()
	file, err := os.ReadFile("./test.yml")
	if err != nil {
		panic(err)
	}
	compose, err := NewCompose(file, sessionUUID.String(), "", "")
	if err != nil {
		panic(err)
	}
	ctx := context.Background()
	err = compose.PrepareNetwork(ctx)
	if err != nil {
		panic(err)
	}
	err = compose.StartPods(ctx)
	if err != nil {
		panic(err)
	}
}
