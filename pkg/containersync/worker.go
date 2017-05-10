package containersync

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	dct "github.com/docker/docker/api/types"
	dce "github.com/docker/docker/api/types/events"
	dcf "github.com/docker/docker/api/types/filters"
	dc "github.com/docker/docker/client"
	"github.com/fuserobotics/deviced/pkg/config"
	"github.com/fuserobotics/deviced/pkg/reflection"
	"github.com/fuserobotics/deviced/pkg/state"
	"github.com/fuserobotics/deviced/pkg/utils"
)

const deviced_id_label string = "deviced.id"

/* Container Sync Worker
This worker periodically compares the list of
target containers, the list of running containers,
and attempts to reconcile by deleting / creating.

It also checks the list of available images to see
if a better version for a container is available.

If so it will delete the old container and make
a new one with the new version.

The worker uses the Docker events API to wait for events.

Waking the worker can be done by sending
*/
type ContainerSyncWorker struct {
	Config       *config.DevicedConfig
	ConfigLock   *sync.Mutex
	WorkerLock   *sync.Mutex
	DockerClient *dc.Client
	Reflection   *reflection.DevicedReflection

	EventsContext       context.Context
	EventsContextCancel context.CancelFunc
	Running             bool

	EventsChannel <-chan dce.Message
	ErrorsChannel <-chan error
	WakeChannel   chan bool
}

// Init the worker
func (cw *ContainerSyncWorker) Init() error {
	cw.Running = true
	cw.WakeChannel = make(chan bool)
	cw.startEventStream()
	return nil
}

func (cw *ContainerSyncWorker) startEventStream() {
	cw.stopEventStream()

	fmt.Printf("Registering event listeners...\n")
	cw.EventsContext, cw.EventsContextCancel = context.WithCancel(context.Background())
	cw.EventsChannel, cw.ErrorsChannel = cw.DockerClient.Events(cw.EventsContext, dct.EventsOptions{})
}

func (cw *ContainerSyncWorker) stopEventStream() {
	if cw.EventsContextCancel == nil {
		return
	}

	fmt.Printf("Closing event listeners...\n")
	cw.EventsContextCancel()
	cw.EventsContextCancel = nil
	cw.EventsContext = nil
}

func (cw *ContainerSyncWorker) sleepShouldQuit(t time.Duration) bool {
	select {
	case <-time.After(t):
		return false
	case _, ok := <-cw.WakeChannel:
		return !ok
	}
}

func buildRunningContainer(ctr dct.Container, mt *config.TargetContainer, score uint) *state.RunningContainer {
	nrc := new(state.RunningContainer)
	nrc.DevicedID = mt.Id
	nrc.Image, nrc.ImageTag = utils.ParseImageAndTag(ctr.Image)
	nrc.ApiContainer = &ctr
	nrc.Score = score
	return nrc
}

func (cw *ContainerSyncWorker) processNetworks() map[string]dct.NetworkResource {
	// Build map of current networks
	netMap := make(map[string]dct.NetworkResource)

	/*
		Can't do this, what if we have a container that wants for example "bridge"?
		if len(cw.Config.Networks) == 0 {
			return netMap
		}
	*/

	fmt.Printf("ContainerSyncWorker checking networks...\n")

	// Load the current network list
	list, err := cw.DockerClient.NetworkList(context.Background(), dct.NetworkListOptions{})
	if err != nil {
		fmt.Printf("Unable to sync networks, error: %v\n", err)
		return nil
	}

	for _, net := range list {
		netMap[net.Name] = net
	}

	// Copy the target net list
	targetNetworks := cw.Config.Networks
	for _, net := range targetNetworks {
		if net.Name == "" {
			fmt.Printf("Warning: invalid network definition in config with empty name.\n")
			continue
		}
		if _, ok := netMap[net.Name]; ok {
			continue
		}

		fmt.Printf("Attempting to create network %s...\n", net.Name)
		cnet, err := cw.DockerClient.NetworkCreate(context.Background(), net.Name, net.NetworkCreate)
		if err != nil {
			fmt.Printf("Error creating network %s, %v!\n", net.Name, err)
			continue
		}
		fmt.Printf("Created network %s succesfully.\n", net.Name)
		resource, err := cw.DockerClient.NetworkInspect(context.Background(), cnet.ID)
		if err != nil {
			fmt.Printf("Error fetching created network %s, %v!\n", cnet.ID, err)
			netMap[net.Name] = dct.NetworkResource{ID: cnet.ID}
			continue
		}
		netMap[net.Name] = resource
	}

	return netMap
}

func (cw *ContainerSyncWorker) processOnce() {
	// Lock config
	cw.ConfigLock.Lock()
	defer cw.ConfigLock.Unlock()
	cw.WorkerLock.Lock()
	defer cw.WorkerLock.Unlock()

	netMap := cw.processNetworks()

	fmt.Printf("ContainerSyncWorker checking containers...\n")

	// Load the current container list
	args, err := dcf.ParseFlag("label="+deviced_id_label, dcf.NewArgs())
	if err != nil {
		fmt.Printf("Unable to build label filter! %v\n", err)
		return
	}

	containers, err := cw.DockerClient.ContainerList(context.Background(), dct.ContainerListOptions{
		All:     true,
		Filters: args,
	})
	if err != nil {
		fmt.Printf("Unable to list containers, error: %v\n", err)
		if cw.sleepShouldQuit(time.Duration(2 * time.Second)) {
			return
		}
	}

	// Initially grab the available images list.
	images, err := cw.DockerClient.ImageList(context.Background(), dct.ImageListOptions{All: true})
	if err != nil {
		fmt.Printf("Error fetching images list %v\n", err)
		return
	}

	availableTagMap := utils.BuildImageMap(images)

	// Sync containers to running containers list.
	devicedIdToContainer := make(map[string]state.RunningContainer)
	containersToDelete := make(map[string][]config.LifecycleHook)
	containersToStart := make(map[string]bool)
	containersToCreate := []dct.ContainerCreateConfig{}
	for _, ctr := range containers {
		fmt.Printf("Container name: %s tag: %s\n", ctr.Names[0], ctr.Image)
		_, imageTag := utils.ParseImageAndTag(ctr.Image)

		// try to match the container to a target container
		// match by tag
		var matchingTarget *config.TargetContainer
		for _, tctr := range cw.Config.Containers {
			if strings.EqualFold(tctr.Id, ctr.Labels[deviced_id_label]) {
				matchingTarget = tctr
				break
			}
		}

		if matchingTarget == nil {
			fmt.Printf("Cannot find match for container %s (%s), scheduling delete.\n", ctr.Names[0], ctr.Image)
			containersToDelete[ctr.ID] = []config.LifecycleHook{}
			continue
		}

		if ctr.State != "running" && !matchingTarget.RestartExited {
			fmt.Printf("Container %s (%s) not running and RestartExited not set, killing.\n", ctr.Names[0], ctr.Image)
			containersToDelete[ctr.ID] = []config.LifecycleHook{}
			continue
		}

		runningContainer := buildRunningContainer(ctr, matchingTarget, matchingTarget.ContainerVersionScore(imageTag))
		if val, ok := devicedIdToContainer[matchingTarget.Id]; ok {
			// We have an existing container that satisfies this target
			// Pick one. Compare versions.
			// Lower is better.
			_, oimageTag := utils.ParseImageAndTag(val.Image)
			otherScore := matchingTarget.ContainerVersionScore(oimageTag)
			thisScore := matchingTarget.ContainerVersionScore(imageTag)
			if thisScore < otherScore {
				fmt.Printf("Choosing container %s (%s) over container %s (%s).\n", ctr.ID, imageTag, val.ApiContainer.ID, oimageTag)
				devicedIdToContainer[matchingTarget.Id] = *runningContainer
				containersToDelete[val.ApiContainer.ID] = matchingTarget.LifecycleHooks.OnStop
				containersToStart[ctr.ID] = true
			} else {
				fmt.Printf("Choosing container %s (%s) over container %s (%s).\n", val.ApiContainer.ID, oimageTag, ctr.ID, imageTag)
				containersToDelete[ctr.ID] = matchingTarget.LifecycleHooks.OnStop
				containersToStart[val.ApiContainer.ID] = true
			}
		} else {
			devicedIdToContainer[matchingTarget.Id] = *runningContainer
		}
	}

	// Decide if there's a better image for each target
	for _, tctr := range cw.Config.Containers {
		currentCtr, ok := devicedIdToContainer[tctr.Id]
		okn := false
		if ok && currentCtr.Score == 0 {
			continue
		}
		images := availableTagMap[tctr.Image]
		if len(images) == 0 {
			fmt.Printf("Container %s has no available tags yet.\n", tctr.Image)
			continue
		}
		selectedCtr := currentCtr
		fmt.Printf("Container %s available tags:\n", tctr.Image)
		for _, avail := range images {
			score := tctr.ContainerVersionScore(avail)
			fmt.Printf(" => %s score %d\n", avail, score)
			// will be int.max if invalid
			if !tctr.UseAnyVersion && score > 1000 {
				continue
			}
			if ok && avail == currentCtr.ImageTag {
				continue
			}
			if ok && currentCtr.Score < score {
				continue
			}
			selectedCtr = state.RunningContainer{
				DevicedID:    tctr.Id,
				Image:        tctr.Image,
				ImageTag:     avail,
				Score:        score,
				ApiContainer: nil,
			}
			okn = true
		}
		if !ok && !okn {
			fmt.Printf("Container %s has no suitable image, skipping.\n", tctr.Image)
			continue
		}
		if ok && currentCtr == selectedCtr {
			fmt.Printf("Container %s has no better image than the current, skipping.\n", tctr.Image)
			continue
		}
		if ok && selectedCtr != currentCtr {
			fmt.Printf("Replacing container %s:%s with new container at %s:%s\n", currentCtr.Image, currentCtr.ImageTag, selectedCtr.Image, selectedCtr.ImageTag)
			containersToDelete[currentCtr.ApiContainer.ID] = tctr.LifecycleHooks.OnStop
		}
		fmt.Printf("Starting container (%s) %s:%s...\n", tctr.Id, selectedCtr.Image, selectedCtr.ImageTag)
		opts := dct.ContainerCreateConfig{
			Name:             strings.Join([]string{"devd", tctr.Id, strconv.Itoa(rand.Int() % 100)}, "_"),
			Config:           &tctr.DockerConfig,
			HostConfig:       &tctr.DockerHostConfig,
			NetworkingConfig: &tctr.DockerNetworkingConfig,
		}
		if opts.Config.Labels == nil {
			opts.Config.Labels = make(map[string]string)
		}
		opts.Config.Labels["deviced.id"] = tctr.Id
		opts.Config.Image = strings.Join([]string{selectedCtr.Image, selectedCtr.ImageTag}, ":")
		containersToCreate = append(containersToCreate, opts)
		devicedIdToContainer[tctr.Id] = selectedCtr
	}

	// We have picked the containers to keep. Delete the others.
	for cid, hooks := range containersToDelete {
		if cw.Reflection != nil && cw.Reflection.Container.ID == cid {
			if !cw.Config.ContainerConfig.AllowSelfDelete {
				fmt.Printf("Preventing deletion of ourselves...\n")
				continue
			}
			fmt.Printf("Allowing self deletion...\n")
		}

		// Run stop hooks
		fmt.Printf("Stopping container %s (running stop hooks)...\n", cid)
		for hidx, hook := range hooks {
			fmt.Printf("Running stop hook %d...\n", hidx)
			if hook.Exec != nil {
				fmt.Printf("Running stop hook %d exec...\n", hidx)
				execCtx, execCtxCancel := context.WithCancel(context.Background())
				exec, err := cw.DockerClient.ContainerExecCreate(execCtx, cid, dct.ExecConfig{
					Cmd: hook.Exec.Command,
					Tty: true,
				})
				if err != nil {
					fmt.Printf("Error creating exec for %s onexit hook: %v\n", cid, err)
					execCtxCancel()
					continue
				}
				conn, err := cw.DockerClient.ContainerExecAttach(execCtx, exec.ID, dct.ExecConfig{
					Tty: true,
				})
				if err != nil {
					fmt.Printf("Error starting exec for %s onexit hook: %v\n", cid, err)
					execCtxCancel()
					continue
				}
				closeChannel := make(chan bool)
				go func() {
					defer func() {
						conn.Close()
						close(closeChannel)
					}()
					_, err := ioutil.ReadAll(conn.Reader)
					if err != nil {
						fmt.Printf("Error waiting for finish exec for %s onexit hook: %v\n", cid, err)
					}
				}()

				waitDur, err := time.ParseDuration(hook.Exec.Timeout)
				if err != nil || hook.Exec.Timeout == "" {
					fmt.Printf("Using default wait time of 30 seconds for hook...\n")
					waitDur = time.Duration(30) * time.Second
				}
				select {
				case _, _ = <-closeChannel:
				case <-time.After(waitDur):
					fmt.Printf("Exec stop hook timed out, continuing.")
				}
				execCtxCancel()
			}
		}

		fmt.Printf("Stopping container %s...\n", cid)
		secThirty := time.Duration(30) * time.Second
		if err := cw.DockerClient.ContainerStop(context.Background(), cid, &secThirty); err != nil {
			fmt.Printf("Error stopping container %s, %v\n", cid, err)
		}
		opts := dct.ContainerRemoveOptions{Force: true}
		if err := cw.DockerClient.ContainerRemove(context.Background(), cid, opts); err != nil {
			fmt.Printf("Error attempting to remove container, %v\n", err)
		}
	}

	for _, ctr := range containersToCreate {
		if ctr.HostConfig.NetworkMode != "" {
			if _, ok := netMap[ctr.HostConfig.NetworkMode.NetworkName()]; !ok {
				fmt.Printf("Cannot find network %s in available networks. Skipping creation of %s.\n", ctr.HostConfig.NetworkMode, ctr.Name)
				continue
			}
		}
		created, err := cw.DockerClient.ContainerCreate(context.Background(), ctr.Config, ctr.HostConfig, ctr.NetworkingConfig, ctr.Name)
		if err != nil {
			fmt.Printf("Container creation error: %v\n", err)
			continue
		}
		containersToStart[created.ID] = true
	}

	for ctr := range containersToStart {
		err = cw.DockerClient.ContainerStart(context.Background(), ctr, dct.ContainerStartOptions{})
		if err != nil {
			if !strings.Contains(err.Error(), "already running") {
				fmt.Printf("Container start error: %v\n", err)
			}
		}
	}
	containersToStart = nil
}

func (cw *ContainerSyncWorker) Run() {
	for cw.Running {
		hasEvents := true
		for hasEvents {
			select {
			case _ = <-cw.WakeChannel:
				continue
			default:
				hasEvents = false
				break
			}
		}

		cw.processOnce()

		// Flush the events
		hasEvents = true
		for hasEvents {
			select {
			case _ = <-cw.EventsChannel:
				continue
			default:
				hasEvents = false
				break
			}
		}

		fmt.Printf("ContainerSyncWorker sleeping...\n")
		doRecheck := false
		for !doRecheck {
			select {
			case _, ok := <-cw.WakeChannel:
				if !ok {
					fmt.Printf("ContainerSyncWorker exiting...\n")
					return
				}

				fmt.Printf("ContainerSyncWorker woken, re-checking...\n")
				doRecheck = true
				break
				// We want to re-check with the following events:
				// - pull
				// - tag
				// - import
			case event := <-cw.EventsChannel:
				fmt.Printf("Docker event type triggered: %s\n", event.Type)
				// use continue to ignore event
				if event.Type == "image" || event.Type == "network" || event.Type == "container" {
					fmt.Printf("Rechecking in %d seconds due to %s event.\n", 1, event.Type)
					doRecheck = true
					time.Sleep(1 * time.Second)
				}
				break
			}
		}
	}
}

func (cw *ContainerSyncWorker) Quit() {
	if !cw.Running {
		return
	}
	cw.Running = false
	close(cw.WakeChannel)
}
