package config

import (
	"fmt"

	"github.com/fsnotify/fsnotify"
)

type DevicedConfigWatcher struct {
	ConfigPath    *string
	ConfigWatcher *fsnotify.Watcher
}

func (cw *DevicedConfigWatcher) Init() int {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Errorf("Unable to initialize filesystem watcher, %s\n", err)
		return 1
	}
	cw.ConfigWatcher = watcher
	err = watcher.Add(*cw.ConfigPath)
	if err != nil {
		fmt.Errorf("Unable to initialize filesystem watcher, %s\n", err)
		return 1
	}
	return 0
}

func (cw *DevicedConfigWatcher) Close() {
	cw.ConfigWatcher.Close()
}
