package imagesync

import (
	"fmt"
	"math"
	"net/url"
	"sync"
	"time"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/reference"
	"github.com/docker/engine-api/types"
	ddistro "github.com/synrobo/deviced/pkg/distribution"

	dc "github.com/fsouza/go-dockerclient"
	"github.com/synrobo/deviced/pkg/config"
	"github.com/synrobo/deviced/pkg/registry"
	"github.com/synrobo/deviced/pkg/utils"
)

type ImageSyncWorker struct {
	Config       *config.DevicedConfig
	ConfigLock   *sync.Mutex
	DockerClient *dc.Client

	Running     bool
	WakeChannel chan bool
	QuitChannel chan bool

	RegistryContext context.Context
}

func (iw *ImageSyncWorker) Init() {
	iw.Running = true
	iw.WakeChannel = make(chan bool, 1)
	iw.QuitChannel = make(chan bool, 1)
	iw.RegistryContext = context.Background()
}

func (iw *ImageSyncWorker) sleepShouldQuit(t time.Duration) bool {
	time.Sleep(t)
	select {
	case <-iw.QuitChannel:
		return true
	case <-iw.WakeChannel:
		return false
	default:
		return false
	}
}

// AvailableAt
// - map between tag -> registry
type imageToFetch struct {
	FetchAny    bool
	NeededTags  []string
	AvailableAt map[string][]*distribution.Repository
	Target      *config.TargetContainer
}

func (iw *ImageSyncWorker) Run() {
	doRecheck := true
	for iw.Running {
		fmt.Printf("ImageSyncWorker sleeping...\n")
		for !doRecheck {
			select {
			case <-iw.QuitChannel:
				fmt.Printf("ImageSyncWorker exiting...\n")
				return
			case <-iw.WakeChannel:
				fmt.Printf("ImageSyncWorker woken, re-checking...\n")
				doRecheck = true
				break
			}
		}
		doRecheck = false
		fmt.Printf("ImageSyncWorker checking repositories...\n")

		iw.ConfigLock.Lock()
		if len(iw.Config.Repos) == 0 {
			fmt.Printf("No repositories given in config.\n")
			iw.ConfigLock.Unlock()
			continue
		}
		iw.ConfigLock.Unlock()

		// Load the current image list
		liOpts := dc.ListImagesOptions{}
		images, err := iw.DockerClient.ListImages(liOpts)
		if err != nil {
			fmt.Printf("Error fetching images list %v\n", err)
			if iw.sleepShouldQuit(time.Duration(2 * time.Second)) {
				return
			}
			continue
		}

		imageMap := utils.BuildImageMap(&images)

		// For each target container grab the best tag available currently
		// If the best tag is score 0 don't check it
		// We only want to fetch better than the current best.
		var imagesToFetch []*imageToFetch
		iw.ConfigLock.Lock()
		for _, ctr := range iw.Config.Containers {
			image := &ctr.Image
			availableTags := imageMap[*image]
			bestAvailable := ""
			var bestAvailableScore uint
			bestAvailableScore = math.MaxUint16
			for _, avail := range availableTags {
				score := ctr.ContainerVersionScore(avail)
				if score < bestAvailableScore {
					bestAvailableScore = score
					bestAvailable = avail
				}
			}
			if bestAvailableScore == 0 || (len(ctr.Versions) == 0 && !ctr.UseAnyVersion) {
				continue
			}
			// fetch anything from index 0 to bestAvailableScore (non inclusive)
			var tagsToFetch []string
			if bestAvailable == "" {
				tagsToFetch = ctr.Versions
			} else {
				tagsToFetch = ctr.Versions[:bestAvailableScore]
			}
			fmt.Printf("We need to fetch images for %s\n", *image)
			fmt.Printf("Best available: %s score: %d\n", bestAvailable, bestAvailableScore)
			fmt.Printf("Versions to fetch: %v\n", tagsToFetch)
			if ctr.UseAnyVersion {
				fmt.Printf("... but we will settle for any version.\n")
			}
			toFetch := new(imageToFetch)
			toFetch.FetchAny = ctr.UseAnyVersion
			toFetch.NeededTags = tagsToFetch
			toFetch.Target = &ctr
			imagesToFetch = append(imagesToFetch, toFetch)
		}

		if len(imagesToFetch) == 0 {
			iw.ConfigLock.Unlock()
			continue
		}

		// Build registry client
		// Rebuild the registry list
		for _, rege := range iw.Config.Repos {
			urlParsed, err := url.Parse(rege.Url)
			if err != nil {
				fmt.Printf("Unable to parse url %s, %v\n", rege.Url, err)
				continue
			}
			var insecureRegs []string
			if rege.Insecure {
				insecureRegs = []string{urlParsed.Host}
			}
			service := registry.NewService(registry.ServiceOptions{InsecureRegistries: insecureRegs})
			for _, tf := range imagesToFetch {
				ref, err := reference.ParseNamed(tf.Target.Image)
				if err != nil {
					fmt.Printf("Error parsing reference %s, %v.\n", tf.Target.Image, err)
					continue
				}
				info, err := registry.ParseRepositoryInfo(ref)
				if err != nil {
					fmt.Printf("Error parsing repository info %s, %v.\n", tf.Target.Image, err)
					continue
				}
				endpoints, err := service.LookupPullEndpoints(urlParsed.Host)
				if err != nil {
					fmt.Printf("Error parsing endpoints %s, %v.\n", rege.Url, err)
					continue
				}
				metaHeaders := rege.MetaHeaders
				authConfig := &types.AuthConfig{Username: rege.Username, Password: rege.Password}
				successfullyConnected := false
				// var endpoint registry.APIEndpoint
				var reg distribution.Repository
				for _, endp := range endpoints {
					reg, _, err = ddistro.NewV2Repository(iw.RegistryContext, info, endp, metaHeaders, authConfig, "pull")
					if err != nil {
						fmt.Printf("Error connecting to '%s', %v\n", rege.Url, err)
						continue
					}
					successfullyConnected = true
					break
				}
				if !successfullyConnected {
					fmt.Printf("Unable to connect successfully to %s.\n", rege.Url)
					continue
				}
				// tags is the tag service
				tags, err := reg.Tags(iw.RegistryContext).All(iw.RegistryContext)
				if err != nil {
					fmt.Printf("Error checking '%s' for %s, %v\n", rege.Url, tf.Target.Image, err)
					continue
				}
				for _, tag := range tags {
					fmt.Printf("%s, available tag %s\n", tf.Target.Image, tag)
					tf.AvailableAt[tag] = append(tf.AvailableAt[tag], &reg)
				}
			}
			iw.ConfigLock.Unlock()

			for _, tf := range imagesToFetch {
				for _, tag := range tf.NeededTags {
					for _, reg := range tf.AvailableAt[tag] {
						_ = reg
						fmt.Printf("%s:%s: available, downloading...\n", tf.Target.Image, tag)
					}
				}
			}

			// Flush the wake channel
			hasEvents := true
			for hasEvents {
				select {
				case _ = <-iw.WakeChannel:
					continue
				default:
					hasEvents = false
					break
				}
			}
		}
	}
	fmt.Printf("ImageSyncWorker exiting...\n")
}

func (iw *ImageSyncWorker) Quit() {
	if !iw.Running {
		return
	}
	iw.Running = false
	iw.QuitChannel <- true
}
