package testcompose

import (
	"context"
	"testing"
)

func TestTestCompose(t *testing.T) {
	compose, err := NewTestCompose("./test", "")
	if err != nil {
		panic(err)
	}
	ctx := context.Background()
	err = compose.Start(ctx)
	if err != nil {
		panic(err)
	}
}
