package testcompose

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestTestCompose(t *testing.T) {
	getwd, err := os.Getwd()
	if err != nil {
		return
	}
	compose, err := NewTestCompose(filepath.Join(getwd, "./example"), "")
	if err != nil {
		panic(err)
	}
	ctx := context.Background()
	err = compose.Start(ctx)
	if err != nil {
		panic(err)
	}
}
