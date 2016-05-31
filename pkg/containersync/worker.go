package containersync

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	dc "github.com/fsouza/go-dockerclient"
	"github.com/synrobo/deviced/pkg/config"
	"github.com/synrobo/deviced/pkg/reflection"
	"github.com/synrobo/deviced/pkg/state"
	"github.com/synrobo/deviced/pkg/utils"
)

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

	Running       bool
	EventsChannel chan *dc.APIEvents
	WakeChannel   chan bool
	QuitChannel   chan bool
}

// Init the worker
func (cw *ContainerSyncWorker) Init() error {
	cw.Running = true
	cw.EventsChannel = make(chan *dc.APIEvents, 50)
	cw.WakeChannel = make(chan bool, 5)
	cw.QuitChannel = make(chan bool, 5)
	fmt.Printf("Registering event listeners...\n")
	if err := cw.DockerClient.AddEventListener(cw.EventsChannel); err != nil {
		return err
	}
	return nil
}

func (cw *ContainerSyncWorker) sleepShouldQuit(t time.Duration) bool {
	time.Sleep(t)
	select {
	case <-cw.QuitChannel:
		return true
	case <-cw.WakeChannel:
		return false
	default:
		return false
	}
}

func buildRunningContainer(ctr *dc.APIContainers, mt *config.TargetContainer, score uint) *state.RunningContainer {
	nrc := new(state.RunningContainer)
	nrc.DevicedID = mt.Id
	nrc.Image, nrc.ImageTag = utils.ParseImageAndTag(ctr.Image)
	nrc.ApiContainer = ctr
	nrc.Score = score
	return nrc
}

func (cw *ContainerSyncWorker) processOnce() {
	// Lock config
	cw.ConfigLock.Lock()
	defer cw.ConfigLock.Unlock()
	cw.WorkerLock.Lock()
	defer cw.WorkerLock.Unlock()

	fmt.Printf("ContainerSyncWorker checking containers...\n")

	// Load the current container list
	listOpts := dc.ListContainersOptions{
		All:  true,
		Size: false,
	}

	if !cw.Config.ContainerConfig.ManageAllContainers {
		listOpts.Filters = map[string][]string{}
		listOpts.Filters["label"] = []string{"deviced.id"}
	}

	containers, err := cw.DockerClient.ListContainers(listOpts)
	if err != nil {
		fmt.Errorf("Unable to list containers, error: %v\n", err)
		if cw.sleepShouldQuit(time.Duration(2 * time.Second)) {
			return
		}
	}

	// Initially grab the available images list.
	liOpts := dc.ListImagesOptions{}
	images, err := cw.DockerClient.ListImages(liOpts)
	if err != nil {
		fmt.Printf("Error fetching images list %v\n", err)
		return
	}

	availableTagMap := utils.BuildImageMap(&images)

	// Convenience
	tctrs := &cw.Config.Containers

	// Sync containers to running containers list.
	devicedIdToContainer := make(map[string]state.RunningContainer)
	containersToDelete := make(map[string]bool)
	containersToStart := make(map[string]bool)
	containersToCreate := []*dc.CreateContainerOptions{}
	for _, ctr := range containers {
		fmt.Printf("Container name: %s tag: %s\n", ctr.Names[0], ctr.Image)
		image, imageTag := utils.ParseImageAndTag(ctr.Image)

		// try to match the container to a target container
		// match by image
		var matchingTarget *config.TargetContainer
		for _, tctr := range *tctrs {
			if strings.EqualFold(tctr.Image, image) {
				matchingTarget = &tctr
				break
			}
		}

		if matchingTarget == nil {
			fmt.Printf("Cannot find match for container %s (%s), scheduling delete.\n", ctr.Names[0], ctr.Image)
			containersToDelete[ctr.ID] = true
			continue
		}

		if ctr.State != "running" && !matchingTarget.RestartExited {
			fmt.Printf("Container %s (%s) not running and RestartExited not set, killing.\n", ctr.Names[0], ctr.Image)
			containersToDelete[ctr.ID] = true
			continue
		}

		runningContainer := buildRunningContainer(&ctr, matchingTarget, matchingTarget.ContainerVersionScore(imageTag))

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
				containersToDelete[val.ApiContainer.ID] = true
				containersToStart[ctr.ID] = true
			} else {
				fmt.Printf("Choosing container %s (%s) over container %s (%s).\n", val.ApiContainer.ID, oimageTag, ctr.ID, imageTag)
				containersToDelete[ctr.ID] = true
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
			containersToDelete[currentCtr.ApiContainer.ID] = true
		}
		fmt.Printf("Starting container %s:%s...\n", selectedCtr.Image, selectedCtr.ImageTag)
		opts := &dc.CreateContainerOptions{
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
	for cid, _ := range containersToDelete {
		if cw.Reflection != nil && cw.Reflection.Container.ID == cid {
			if !cw.Config.ContainerConfig.AllowSelfDelete {
				fmt.Printf("Preventing deletion of ourselves...\n")
				continue
			}
			fmt.Printf("Allowing self deletion...\n")
		}
		opts := dc.RemoveContainerOptions{ID: cid, Force: true}
		if err := cw.DockerClient.RemoveContainer(opts); err != nil {
			fmt.Printf("Error attempting to remove container, %v\n", err)
		}
	}

	for _, ctr := range containersToCreate {
		created, err := cw.DockerClient.CreateContainer(*ctr)
		if err != nil {
			fmt.Printf("Container creation error: %v\n", err)
			continue
		}
		containersToStart[created.ID] = true
	}

	for ctr, _ := range containersToStart {
		err = cw.DockerClient.StartContainer(ctr, nil)
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
			case <-cw.QuitChannel:
				fmt.Printf("ContainerSyncWorker exiting...\n")
				return
			case <-cw.WakeChannel:
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
				if event.Type == "image" {
					doRecheck = true
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
	cw.QuitChannel <- true
}
