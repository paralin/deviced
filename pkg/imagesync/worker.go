package imagesync

import (
	"fmt"
	"math"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/reference"
	"github.com/docker/engine-api/types"
	ddistro "github.com/synrobo/deviced/pkg/distribution"

	dc "github.com/fsouza/go-dockerclient"
	"github.com/synrobo/deviced/pkg/arch"
	"github.com/synrobo/deviced/pkg/config"
	"github.com/synrobo/deviced/pkg/registry"
	"github.com/synrobo/deviced/pkg/utils"
)

type ImageSyncWorker struct {
	Config       *config.DevicedConfig
	ConfigLock   *sync.Mutex
	WorkerLock   *sync.Mutex
	DockerClient *dc.Client

	Running              bool
	WakeChannel          chan bool
	QuitChannel          chan bool
	WakeContainerChannel *chan bool
	RecheckTimer         *time.Timer
	UnsolvedReqs         bool

	RegistryContext context.Context
}

func (iw *ImageSyncWorker) Init() {
	iw.Running = true
	iw.WakeChannel = make(chan bool, 1)
	iw.QuitChannel = make(chan bool, 1)
	iw.RegistryContext = context.Background()
}

func (iw *ImageSyncWorker) killRecheckTimer() {
	if iw.RecheckTimer != nil {
		iw.RecheckTimer.Stop()
		iw.RecheckTimer = nil
	}
}

func (iw *ImageSyncWorker) initRecheckTimer() {
	// create with a dummy value initially
	iw.killRecheckTimer()
	iw.ConfigLock.Lock()
	iw.RecheckTimer = time.NewTimer(time.Second * time.Duration(10))
	if iw.Config.ImageConfig.RecheckPeriod < 1 || !iw.UnsolvedReqs {
		iw.RecheckTimer.Stop()
	} else {
		iw.RecheckTimer.Reset(time.Second * time.Duration(iw.Config.ImageConfig.RecheckPeriod))
	}
	iw.ConfigLock.Unlock()
}

func (iw *ImageSyncWorker) RecheckConfig() {
	iw.killRecheckTimer()
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
	AvailableAt map[string][]availableDownloadRepository
	Target      config.TargetContainer
}

type availableDownloadRepository struct {
	Repo    distribution.Repository
	RepoRef config.RemoteRepository
}

func (iw *ImageSyncWorker) processOnce() {
	iw.killRecheckTimer()
	iw.UnsolvedReqs = false
	iw.ConfigLock.Lock()
	defer iw.ConfigLock.Unlock()
	iw.WorkerLock.Lock()
	defer iw.WorkerLock.Unlock()
	shouldTriggerContainerCheck := false
	fmt.Printf("ImageSyncWorker checking repositories...\n")
	repoLen := len(iw.Config.Repos)
	if repoLen == 0 {
		fmt.Printf("No repositories given in config.\n")
		return
	}

	// Load the current image list
	liOpts := dc.ListImagesOptions{}
	images, err := iw.DockerClient.ListImages(liOpts)
	if err != nil {
		fmt.Printf("Error fetching images list %v\n", err)
		return
	}

	imageMap := utils.BuildImageMap(&images)

	// For each target container grab the best tag available currently
	// If the best tag is score 0 don't check it
	// We only want to fetch better than the current best.
	var imagesToFetch []*imageToFetch
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
		versionList := arch.AppendArchTagSuffix(ctr.Versions)
		// fetch anything from index 0 to bestAvailableScore (non inclusive)
		var tagsToFetch []string
		if bestAvailable == "" {
			tagsToFetch = versionList
		} else {
			tagsToFetch = versionList[:bestAvailableScore]
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
		toFetch.Target = *ctr
		toFetch.AvailableAt = make(map[string][]availableDownloadRepository)
		imagesToFetch = append(imagesToFetch, toFetch)
	}

	if len(imagesToFetch) == 0 {
		return
	}

	fmt.Printf("Preparing to fetch %d repos...\n", len(imagesToFetch))

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
			image := tf.Target.Image
			imagePts := strings.Split(image, "/")
			if len(imagePts) == 1 {
				image = strings.Join([]string{"library", image}, "/")
				tf.Target.Image = image
			}
			ref, err := reference.ParseNamed(image)
			if err != nil {
				fmt.Printf("Error parsing reference %s, %v.\n", image, err)
				continue
			}
			info, err := registry.ParseRepositoryInfo(ref)
			if err != nil {
				fmt.Printf("Error parsing repository info %s, %v.\n", image, err)
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
				fmt.Printf("Error checking '%s' for %s, %v\n", rege.Url, image, err)
				continue
			}
			fmt.Printf("From %s, %s is available with %d tags, pull prefix %s.\n", rege.Url, image, len(tags), rege.PullPrefix)
			for _, tag := range tags {
				tf.AvailableAt[tag] = append(tf.AvailableAt[tag], availableDownloadRepository{
					Repo:    reg,
					RepoRef: *rege,
				})
			}
		}

		for _, tf := range imagesToFetch {
			matchedOne := false
			matchedBest := false
			for idx, tag := range tf.NeededTags {
				for _, reg := range tf.AvailableAt[tag] {
					fmt.Printf("%s:%s available from %s, pulling...\n", tf.Target.Image, tag, reg.RepoRef.Url)
					imageWithPrefix := tf.Target.Image
					if reg.RepoRef.PullPrefix != "" {
						imageWithPrefix = strings.Join([]string{reg.RepoRef.PullPrefix, tf.Target.Image}, "/")
					}
					popts := dc.PullImageOptions{
						Repository: imageWithPrefix,
						Tag:        tag,
						// OutputStream: os.Stdout,
						Registry: reg.RepoRef.PullPrefix,
					}
					authopts := dc.AuthConfiguration{
						Username: reg.RepoRef.Username,
						Password: reg.RepoRef.Password,
					}
					err := iw.DockerClient.PullImage(popts, authopts)
					if err != nil {
						fmt.Printf("Failed to pull %s:%s from %s, %v\n", tf.Target.Image, tag, reg.RepoRef.Url, err)
						continue
					}
					if reg.RepoRef.PullPrefix != "" {
						tagopts := dc.TagImageOptions{
							Repo:  tf.Target.Image,
							Tag:   tag,
							Force: true,
						}
						imageWithPrefixAndTag := strings.Join([]string{imageWithPrefix, tag}, ":")
						err = iw.DockerClient.TagImage(imageWithPrefixAndTag, tagopts)
						if err != nil {
							fmt.Printf("Failed to tag %s as %s:%s, %v\n", imageWithPrefixAndTag, tf.Target.Image, tag, err)
							continue
						}
						shouldTriggerContainerCheck = true
						fmt.Printf("tagged %s as %s:%s\n", imageWithPrefixAndTag, tf.Target.Image, tag)
					}
					matchedOne = true
					if idx == 0 {
						matchedBest = true
					}
					break
				}
				if matchedOne {
					break
				}
			}
			if !matchedOne || !matchedBest {
				iw.UnsolvedReqs = true
				fmt.Printf("%s: dependencies unsolved, will recheck later.\n", tf.Target.Image)
			}
		}

		// trigger a wake
		if shouldTriggerContainerCheck {
			(*iw.WakeContainerChannel) <- true
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

func (iw *ImageSyncWorker) Run() {
	doRecheck := true
	for iw.Running {
		if !doRecheck {
			fmt.Printf("ImageSyncWorker sleeping...\n")
			iw.initRecheckTimer()
		}
		for !doRecheck {
			select {
			case <-iw.QuitChannel:
				fmt.Printf("ImageSyncWorker exiting...\n")
				return
			case <-iw.WakeChannel:
				fmt.Printf("ImageSyncWorker woken, re-checking...\n")
				doRecheck = true
				break
			case <-iw.RecheckTimer.C:
				fmt.Printf("ImageSyncWorker timer elapsed, re-checking...\n")
				doRecheck = true
				break
			}
		}
		doRecheck = false
		iw.processOnce()
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
