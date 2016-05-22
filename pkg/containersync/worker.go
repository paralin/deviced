package containersync

import (
	"fmt"
	"strings"
	"sync"
	"time"

	dc "github.com/fsouza/go-dockerclient"
	"github.com/synrobo/deviced/pkg/config"
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
	cw.EventsChannel = make(chan *dc.APIEvents, 50)
	cw.WakeChannel = make(chan bool, 1)
	cw.QuitChannel = make(chan bool, 1)
	cw.StateChangedChannel = make(chan bool, 1)
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

func (cw *ContainerSyncWorker) Run() {
	for cw.Running {
		hasEvents := true
		for hasEvents {
			select {
			case _ = <-cw.WakeChannel:
				continue
			case _ = <-cw.StateChangedChannel:
				continue
			default:
				hasEvents = false
				break
			}
		}

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
			if cw.sleepShouldQuit(time.Duration(2 * time.Second)) {
				return
			}
		}

		// Initially grab the available images list.
		liOpts := dc.ListImagesOptions{}
		images, err := cw.DockerClient.ListImages(liOpts)
		if err != nil {
			fmt.Printf("Error fetching images list %v\n", err)
			if cw.sleepShouldQuit(time.Duration(2 * time.Second)) {
				return
			}
			continue
		}

		availableTagMap := utils.BuildImageMap(&images)

		// Lock config
		cw.ConfigLock.Lock()

		// Convenience
		tctrs := &cw.Config.Containers

		// Sync containers to running containers list.
		devicedIdToContainer := make(map[string]*state.RunningContainer)
		containersToDelete := []string{}
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
				fmt.Printf("Cannot find match for container %s, scheduling delete.\n", ctr.Image)
				containersToDelete = append(containersToDelete, ctr.ID)
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
					devicedIdToContainer[matchingTarget.Id] = runningContainer
					containersToDelete = append(containersToDelete, val.ApiContainer.ID)
				} else {
					fmt.Printf("Choosing container %s (%s) over container %s (%s).\n", val.ApiContainer.ID, oimageTag, ctr.ID, imageTag)
					containersToDelete = append(containersToDelete, ctr.ID)
				}
			} else {
				devicedIdToContainer[matchingTarget.Id] = runningContainer
			}
		}

		// Decide if there's a better image for each target
		for _, tctr := range cw.Config.Containers {
			currentCtr := devicedIdToContainer[tctr.Id]
			if currentCtr != nil && currentCtr.Score == 0 {
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
				if currentCtr != nil && avail == currentCtr.ImageTag {
					continue
				}
				if currentCtr != nil && currentCtr.Score < score {
					continue
				}
				selectedCtr = new(state.RunningContainer)
				selectedCtr.DevicedID = tctr.Id
				selectedCtr.Image = tctr.Image
				selectedCtr.ImageTag = avail
				selectedCtr.Score = score
				selectedCtr.ApiContainer = nil
			}
			if selectedCtr == nil {
				fmt.Printf("Container %s has no suitable image, skipping.\n", tctr.Image)
				continue
			}
			if currentCtr == selectedCtr {
				fmt.Printf("Container %s has no better image than the current, skipping.\n", tctr.Image)
				continue
			}
			if currentCtr != nil && selectedCtr != currentCtr {
				fmt.Printf("Replacing container %s:%s with new container at %s:%s\n", currentCtr.Image, currentCtr.ImageTag, selectedCtr.Image, selectedCtr.ImageTag)
				containersToDelete = append(containersToDelete, currentCtr.ApiContainer.ID)
			}
			fmt.Printf("Starting container %s:%s...\n", selectedCtr.Image, selectedCtr.ImageTag)
			tctr.Options.Name = strings.Join([]string{"deviced", tctr.Id}, "_")
			if tctr.Options.Config == nil {
				tctr.Options.Config = new(dc.Config)
			}
			if tctr.Options.Config.Labels == nil {
				tctr.Options.Config.Labels = make(map[string]string)
			}
			tctr.Options.Config.Labels["mangedby"] = "deviced"
			tctr.Options.Config.Image = strings.Join([]string{selectedCtr.Image, selectedCtr.ImageTag}, ":")
			containersToCreate = append(containersToCreate, &tctr.Options)
			devicedIdToContainer[tctr.Id] = selectedCtr
		}
		cw.State.RunningContainers = devicedIdToContainer

		// Unlock config
		cw.ConfigLock.Unlock()

		// We have picked the containers to keep. Delete the others.
		for _, cid := range containersToDelete {
			opts := dc.RemoveContainerOptions{ID: cid, Force: true}
			if err := cw.DockerClient.RemoveContainer(opts); err != nil {
				fmt.Printf("Error attempting to remove container, %v\n", err)
			}
		}
		containersToDelete = nil

		for _, ctr := range containersToCreate {
			created, err := cw.DockerClient.CreateContainer(*ctr)
			if err != nil {
				fmt.Printf("Container creation error: %v\n", err)
				continue
			}
			err = cw.DockerClient.StartContainer(created.ID, nil)
			if err != nil {
				fmt.Printf("Container start error: %v\n", err)
			}
		}
		containersToCreate = nil

		cw.StateChangedChannel <- true

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

		eventsToListen := map[string]bool{
			"pull":   true,
			"tag":    true,
			"import": true,
		}

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
				// use continue to ignore event
				// fmt.Printf("ContainerSyncWorker processing event, %s\n", event)
				if event.Type == "image" && eventsToListen[event.Action] {
					// Check the actor
					cw.ConfigLock.Lock()
					// If the image is in our list of targets...
					for _, tgt := range cw.Config.Containers {
						if event.Actor.Attributes["name"] == tgt.Image {
							doRecheck = true
							break
						}
					}
					cw.ConfigLock.Unlock()
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
