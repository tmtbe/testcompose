package main

import (
	"context"
	"podcompose/docker"
)

type Cleaner struct {
	dockerProvider *docker.DockerProvider
	sessionId      string
}

func NewCleaner(sessionId string) (*Cleaner, error) {
	dockerProvider, err := docker.NewDockerProvider()
	if err != nil {
		return nil, err
	}
	return &Cleaner{
		sessionId:      sessionId,
		dockerProvider: dockerProvider,
	}, nil
}

func (c *Cleaner) clear() {
	ctx := context.Background()
	c.dockerProvider.ClearWithSession(ctx, c.sessionId)
}
