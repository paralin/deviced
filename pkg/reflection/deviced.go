package reflection

/*
DeviceD Reflection
==================

This will only work if:
 - --net=host is NOT used
 - the container hostname matches the beginning
   of the container ID (this is default)
*/

import (
	dc "github.com/fsouza/go-dockerclient"
	"os"
)

type DevicedReflection struct {
	Container *dc.Container
}

func BuildReflection(client *dc.Client) (*DevicedReflection, error) {
	ctr, err := InspectCurrentContainer(client)
	if err != nil {
		return nil, err
	}
	return &DevicedReflection{Container: ctr}, nil
}

func InspectCurrentContainer(client *dc.Client) (*dc.Container, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	return client.InspectContainer(hostname)
}
