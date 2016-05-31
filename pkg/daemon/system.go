package daemon

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	dc "github.com/fsouza/go-dockerclient"
	"github.com/synrobo/deviced/pkg/config"
	"github.com/synrobo/deviced/pkg/containersync"
	"github.com/synrobo/deviced/pkg/imagesync"
	"github.com/synrobo/deviced/pkg/reflection"
	"github.com/synrobo/deviced/pkg/state"
)

type System struct {
	HomeDir    string
	ConfigPath string
	StatePath  string

	Config        config.DevicedConfig
	ConfigLock    sync.Mutex
	WorkerLock    sync.Mutex
	ConfigWatcher *config.DevicedConfigWatcher
	State         state.DevicedState
	DockerClient  *dc.Client

	ContainerWorker *containersync.ContainerSyncWorker
	ImageWorker     *imagesync.ImageSyncWorker
	Reflection      *reflection.DevicedReflection
}

func (s *System) initHomeDir() int {
	fmt.Printf("Using home directory %s...\n", s.HomeDir)
	// Check if home dir exists
	if _, err := os.Stat(s.HomeDir); os.IsNotExist(err) {
		fmt.Printf("Creating config directory %s...\n", s.HomeDir)
		if err := os.MkdirAll(s.HomeDir, 0777); err != nil {
			fmt.Printf("Unable to create config dir %s, error %s.\n", s.HomeDir, err)
			return 1
		}
	}

	s.ConfigPath = filepath.Join(s.HomeDir, "config.yaml")
	if !s.Config.CreateOrRead(s.ConfigPath) {
		fmt.Printf("Failed to create/read config at %s", s.ConfigPath)
		return 1
	}

	s.StatePath = filepath.Join(s.HomeDir, "state.yaml")
	if !s.State.CreateOrRead(s.StatePath) {
		fmt.Printf("Failed to create/read state at %s", s.StatePath)
		return 1
	}
	return 0
}

func (s *System) initWorkers() int {
	fmt.Printf("Initializing workers...\n")
	var err error

	s.DockerClient, err = s.Config.DockerConfig.BuildClient()
	if err != nil {
		fmt.Printf("Unable to create docker client, %v\n", err)
		return 1
	}

	err = s.DockerClient.Ping()
	if err != nil {
		fmt.Printf("Unable to ping Docker, %v\n", err)
		return 1
	}

	refl, err := reflection.BuildReflection(s.DockerClient)
	if err != nil || refl == nil {
		fmt.Printf("Unable to locate our container, continuing without reflection.\n")
		fmt.Printf("Error locating container was: %v\n", err)
	} else {
		fmt.Printf("Located our container, continuing with reflection.\n")
		s.Reflection = refl
	}

	s.ImageWorker = &imagesync.ImageSyncWorker{
		ConfigLock:   &s.ConfigLock,
		WorkerLock:   &s.WorkerLock,
		DockerClient: s.DockerClient,
		Config:       &s.Config,
	}
	s.ImageWorker.Init()

	s.ContainerWorker = &containersync.ContainerSyncWorker{
		ConfigLock:   &s.ConfigLock,
		WorkerLock:   &s.WorkerLock,
		DockerClient: s.DockerClient,
		Config:       &s.Config,
		State:        &s.State.ContainerWorkerState,
		Reflection:   s.Reflection,
	}
	if err = s.ContainerWorker.Init(); err != nil {
		fmt.Printf("Error initializing ContainerWorker, %v\n", err)
		return 1
	}

	return 0
}

func (s *System) initWatchers() int {
	s.ConfigWatcher = new(config.DevicedConfigWatcher)
	s.ConfigWatcher.ConfigPath = &s.ConfigPath
	if res := s.ConfigWatcher.Init(); res != 0 {
		return res
	}
	return 0
}

// Wake the workers upon a config change
func (s *System) wakeWorkers() {
	fmt.Printf("Config changed, waking workers...\n")
	s.ContainerWorker.WakeChannel <- true
	s.ImageWorker.WakeChannel <- true
}

func (s *System) triggerConfRecheck() {
	fmt.Printf("Config changed, rechecking config...\n")
	s.ImageWorker.RecheckConfig()
}

func (s *System) closeWorkers() {
	s.ContainerWorker.Quit()
	s.ImageWorker.Quit()
}

func (s *System) closeWatchers() {
	s.ConfigWatcher.Close()
}

func (s *System) Main() int {
	if res := s.initHomeDir(); res != 0 {
		return res
	}

	if res := s.initWorkers(); res != 0 {
		return res
	}

	if res := s.initWatchers(); res != 0 {
		return res
	}

	fmt.Printf("Starting image worker...\n")
	go s.ImageWorker.Run()
	fmt.Printf("Starting container worker...\n")
	go s.ContainerWorker.Run()

	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	keepRunning := true
	for keepRunning {
		select {
		case <-c:
			keepRunning = false
			break
		case event := <-s.ConfigWatcher.ConfigWatcher.Events:
			fmt.Printf("event:%s\n", event)
			s.closeWatchers()
			time.Sleep(1 * time.Second)
			s.ConfigLock.Lock()
			didread := s.Config.ReadFrom(s.ConfigPath)
			s.ConfigLock.Unlock()
			if didread {
				s.triggerConfRecheck()
				s.wakeWorkers()
			}
			s.initWatchers()
			continue
		case <-s.ContainerWorker.StateChangedChannel:
			s.State.WriteState(s.StatePath)
			continue
		}
	}
	fmt.Println("Exiting...\n")
	s.closeWorkers()
	s.closeWatchers()
	return 0
}
