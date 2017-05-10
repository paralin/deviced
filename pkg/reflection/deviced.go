package reflection

import (
	"context"
)

/*
DeviceD Reflection
==================

This will only work if:
 - --net=host is NOT used
 - the container hostname matches the beginning
   of the container ID (this is default)
*/

import (
	dct "github.com/docker/docker/api/types"
	dc "github.com/docker/docker/client"
	"os"
)

type DevicedReflection struct {
	Container *dct.ContainerJSON
}

func BuildReflection(client *dc.Client) (*DevicedReflection, error) {
	ctr, err := InspectCurrentContainer(client)
	if err != nil {
		return nil, err
	}
	return &DevicedReflection{Container: ctr}, nil
}

func InspectCurrentContainer(client *dc.Client) (*dct.ContainerJSON, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	img, err := client.ContainerInspect(context.Background(), hostname)
	if err != nil {
		return nil, err
	}
	return &img, err
}
