package daemon

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	dc "github.com/docker/docker/client"
	"github.com/fuserobotics/deviced/pkg/arch"
	"github.com/fuserobotics/deviced/pkg/config"
	"github.com/fuserobotics/deviced/pkg/containersync"
	"github.com/fuserobotics/deviced/pkg/imagesync"
	"github.com/fuserobotics/deviced/pkg/reflection"
)

type System struct {
	ConfigPath string

	Config        config.DevicedConfig
	ConfigLock    sync.Mutex
	WorkerLock    sync.Mutex
	ConfigWatcher *config.DevicedConfigWatcher
	DockerClient  *dc.Client

	ContainerWorker *containersync.ContainerSyncWorker
	ImageWorker     *imagesync.ImageSyncWorker
	Reflection      *reflection.DevicedReflection
}

func (s *System) initConfig() int {
	if !s.Config.CreateOrRead(s.ConfigPath) {
		fmt.Printf("Failed to create/read config at %s", s.ConfigPath)
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

	_, err = s.DockerClient.Ping(context.Background())
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

	s.ContainerWorker = &containersync.ContainerSyncWorker{
		ConfigLock:   &s.ConfigLock,
		WorkerLock:   &s.WorkerLock,
		DockerClient: s.DockerClient,
		Config:       &s.Config,
		Reflection:   s.Reflection,
	}
	if err = s.ContainerWorker.Init(); err != nil {
		fmt.Printf("Error initializing ContainerWorker, %v\n", err)
		return 1
	}

	s.ImageWorker = &imagesync.ImageSyncWorker{
		ConfigLock:           &s.ConfigLock,
		WorkerLock:           &s.WorkerLock,
		DockerClient:         s.DockerClient,
		Config:               &s.Config,
		WakeContainerChannel: &s.ContainerWorker.WakeChannel,
	}
	s.ImageWorker.Init()

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
	s.ImageWorker.WakeChannel <- true
	s.ContainerWorker.WakeChannel <- true
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
	if res := s.initConfig(); res != 0 {
		return res
	}

	if res := s.initWorkers(); res != 0 {
		return res
	}

	if res := s.initWatchers(); res != 0 {
		return res
	}

	archTag := arch.GetArchTagSuffix()
	if archTag != "" {
		fmt.Printf("Using arch tag suffix: %s\n", archTag)
	} else {
		fmt.Printf("Using no arch tag suffix, arch is %s\n", arch.GetArch())
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
			err := s.Config.ReadFrom(s.ConfigPath)
			s.ConfigLock.Unlock()
			if err == nil {
				s.triggerConfRecheck()
				s.wakeWorkers()
			}
			s.initWatchers()
			continue
		}
	}
	fmt.Println("Exiting...")
	s.closeWorkers()
	s.closeWatchers()
	return 0
}
