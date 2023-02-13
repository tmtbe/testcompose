package example

import (
	"context"
	"os"
	"path/filepath"
	"podcompose/testcompose"
	"testing"
)

func TestTestCompose(t *testing.T) {
	getwd, err := os.Getwd()
	if err != nil {
		return
	}
	compose, err := testcompose.NewTestCompose(filepath.Join(getwd, "./pod"))
	if err != nil {
		panic(err)
	}
	ctx := context.Background()
	err = compose.Start(ctx, true)
	if err != nil {
		panic(err)
	}
}
