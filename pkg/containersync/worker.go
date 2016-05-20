package containersync

import (
	"fmt"
	"strings"
	"sync"
	"time"

	dc "github.com/fsouza/go-dockerclient"
	"github.com/synrobo/deviced/pkg/config"
	"github.com/synrobo/deviced/pkg/state"
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
	State        *state.ContainerWorkerState
	ConfigLock   *sync.Mutex
	DockerClient *dc.Client

	Running             bool
	EventsChannel       chan *dc.APIEvents
	WakeChannel         chan bool
	QuitChannel         chan bool
	StateChangedChannel chan bool
}

// Init the worker
func (cw *ContainerSyncWorker) Init() error {
	cw.Running = true
	cw.EventsChannel = make(chan *dc.APIEvents, 10)
	cw.WakeChannel = make(chan bool, 1)
	cw.QuitChannel = make(chan bool, 1)
	cw.StateChangedChannel = make(chan bool, 1)
	fmt.Printf("Registering event listeners...\n")
	if err := cw.DockerClient.AddEventListener(cw.EventsChannel); err != nil {
		return err
	}
	return nil
}

func sleepShouldQuit(cw *ContainerSyncWorker, t Duration) bool {
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

func parseImageAndTag(imagestr string) (string, string) {
	imagePts := strings.Split(imagestr, ":")
	image := imagePts[0]
	var imageTag string
	if len(imagePts) < 2 {
		imageTag = "latest"
	} else {
		imageTag = imagePts[1]
	}
	return image, imageTag
}

func (cw *ContainerSyncWorker) Run() {
	for cw.Running {
		// Load the current container list
		listOpts := dc.ListContainersOptions{
			All:  true,
			Size: false,
		}

		if !cw.Config.ContainerConfig.ManageAllContainers {
			listOpts.Filters = map[string][]string{}
			listOpts.Filters["managedby"] = []string{"deviced"}
		}

		containers, err := cw.DockerClient.ListContainers(listOpts)
		if err != nil {
			fmt.Errorf("Unable to list containers, error: %v\n", err)
			if sleepShouldQuit(time.Duration(2)) {
				return
			}
		}

		// Initially grab the available images list.
		liOpts := dc.ListImagesOptions{}
		images, err := cw.DockerClient.ListImages(liOpts)
		if err != nil {
			fmt.Printf("Error fetching images list %v\n", err)
			if sleepShouldQuit(time.Duration(2)) {
				return
			}
			continue
		}

		// Map of image name -> available tag list
		availableTagMap := map[string][]string{}
		for _, img := range images {
			for _, tagfull := range img.RepoTags {
				if strings.Contains(tagfull, "<none>") {
					continue
				}
				image, tag := parseImageAndTag(tagfull)
				tagList := availableTagMap[image]
				tagList = append(tagList, tag)
				availableTagMap[image] = tagList
			}
		}

		// Lock config
		cw.ConfigLock.Lock()

		// Convenience
		tctrs := &cw.Config.Containers

		// Sync containers to running containers list.
		devicedIdToContainer := make(map[string]*dc.APIContainers)
		devicedIdToContainerScore := make(map[string]uint)
		containersToDelete := []string{}
		for _, ctr := range containers {
			fmt.Printf("Container name: %s tag: %s\n", ctr.Names[0], ctr.Image)
			image, imageTag := parseImageAndTag(ctr.Image)

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
				fmt.Printf("Cannot find match for container %s, scheduling delete.\n", ctr.Image)
				containersToDelete = append(containersToDelete, ctr.ID)
				continue
			}

			if val, ok := devicedIdToContainer[matchingTarget.Id]; ok {
				// We have an existing container that satisfies this target
				// Pick one. Compare versions.
				// Lower is better.
				_, oimageTag := parseImageAndTag(val.Image)
				otherScore := matchingTarget.ContainerVersionScore(oimageTag)
				thisScore := matchingTarget.ContainerVersionScore(imageTag)
				if thisScore < otherScore {
					fmt.Printf("Choosing container %s (%s) over container %s (%s).\n", ctr.ID, imageTag, val.ID, oimageTag)
					devicedIdToContainer[matchingTarget.Id] = &ctr
					devicedIdToContainerScore[matchingTarget.Id] = thisScore
					containersToDelete = append(containersToDelete, val.ID)
				} else {
					fmt.Printf("Choosing container %s (%s) over container %s (%s).\n", val.ID, oimageTag, ctr.ID, imageTag)
					containersToDelete = append(containersToDelete, ctr.ID)
				}
			} else {
				devicedIdToContainer[matchingTarget.Id] = &ctr
				devicedIdToContainerScore[matchingTarget.Id] = matchingTarget.ContainerVersionScore(imageTag)
			}
		}

		// We have picked the containers to keep. Delete the others.
		for _, cid := range containersToDelete {
			opts := dc.RemoveContainerOptions{ID: cid, Force: true}
			if err := cw.DockerClient.RemoveContainer(opts); err != nil {
				fmt.Printf("Error attempting to remove container, %v\n", err)
			}
		}
		containersToDelete = nil

		// Build an array of RunningContainer
		nrunning := make([]*state.RunningContainer, len(devicedIdToContainer))
		idx := 0
		for mtid, ctr := range devicedIdToContainer {
			nrc := new(state.RunningContainer)
			nrunning[idx] = nrc
			nrc.DevicedID = mtid
			nrc.Image, nrc.ImageTag = parseImageAndTag(ctr.Image)
			nrc.ApiContainer = *ctr
			idx++
		}

		// Calculate the best available score for downloaded images

		// Unlock config
		cw.ConfigLock.Unlock()
		cw.StateChangedChannel <- true

		for {
			select {
			case <-cw.QuitChannel:
				fmt.Printf("ContainerSyncWorker exiting...\n")
				return
			case <-cw.WakeChannel:
				fmt.Printf("ContainerSyncWorker woken, re-checking...\n")
				break
			case event := <-cw.EventsChannel:
				_ = event
				// use continue to ignore event
				// fmt.Printf("ContainerSyncWorker processing event, %s\n", event)
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
