/*
	Package types includes Docker API types extracted from the package:

		github.com/docker/docker/api/types

	These types have omitempty tags and problematic fields removed.
*/
package types

import (
	"github.com/docker/docker/api/types/blkiodev"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/go-connections/nat"
	"github.com/docker/go-units"
	"github.com/imdario/mergo"
)

// PidMode represents the pid namespace of the container.
type PidMode string

// CgroupSpec represents the cgroup to use for the container.
type CgroupSpec string

// NetworkMode represents the container network stack.
type NetworkMode string

// UsernsMode represents userns mode in the container.
type UsernsMode string

// UTSMode represents the UTS namespace of the container.
type UTSMode string

// IpcMode represents the container ipc stack.
type IpcMode string

// DeviceMapping represents the device mapping between the host and the container.
type DeviceMapping struct {
	PathOnHost        string `yaml:"pathOnHost,omitempty"`
	PathInContainer   string `yaml:"pathInContainer,omitempty"`
	CgroupPermissions string `yaml:"cgroupPermissions,omitempty"`
}

// Config contains the configuration data about a container.
// It should hold only portable information about the container.
// Here, "portable" means "independent from the host we are running on".
// Non-portable information *should* appear in HostConfig.
// All fields added to this struct must be marked `omitempty` to keep getting
// predictable hashes from the old `v1Compatibility` configuration.
type Config struct {
	Hostname        string                  `json:",omitempty" yaml:"hostname,omitempty"`        // Hostname
	Domainname      string                  `json:",omitempty" yaml:"domainname,omitempty"`      // Domainname
	User            string                  `json:",omitempty" yaml:"user,omitempty"`            // User that will run the command(s) inside the container, also support user:group
	AttachStdin     bool                    `json:",omitempty" yaml:"attachStdin,omitempty"`     // Attach the standard input, makes possible user interaction
	AttachStdout    bool                    `json:",omitempty" yaml:"attachStdout,omitempty"`    // Attach the standard output
	AttachStderr    bool                    `json:",omitempty" yaml:"attachStderr,omitempty"`    // Attach the standard error
	ExposedPorts    nat.PortSet             `json:",omitempty" yaml:"exposedPorts,omitempty"`    // List of exposed ports
	Tty             bool                    `json:",omitempty" yaml:"tty,omitempty"`             // Attach standard streams to a tty, including stdin if it is not closed.
	OpenStdin       bool                    `json:",omitempty" yaml:"openStdin,omitempty"`       // Open stdin
	StdinOnce       bool                    `json:",omitempty" yaml:"stdinOnce,omitempty"`       // If true, close stdin after the 1 attached client disconnects.
	Env             []string                `json:",omitempty" yaml:"env,omitempty"`             // List of environment variable to set in the container
	Cmd             strslice.StrSlice       `json:",omitempty" yaml:"cmd,omitempty"`             // Command to run when starting the container
	Healthcheck     *container.HealthConfig `json:",omitempty" yaml:"healthCheck,omitempty"`     // Healthcheck describes how to check the container is healthy
	Image           string                  `json:",omitempty" yaml:"image,omitempty"`           // Name of the image as it was passed by the operator (e.g. could be symbolic)
	Volumes         map[string]struct{}     `json:",omitempty" yaml:"volumes,omitempty"`         // List of volumes (mounts) used for the container
	WorkingDir      string                  `json:",omitempty" yaml:"workingDir,omitempty"`      // Current directory (PWD) in the command will be launched
	Entrypoint      strslice.StrSlice       `json:",omitempty" yaml:"entrypoint,omitempty"`      // Entrypoint to run when starting the container
	NetworkDisabled bool                    `json:",omitempty" yaml:"networkDisabled,omitempty"` // Is network disabled
	Labels          map[string]string       `json:",omitempty" yaml:"labels,omitempty"`          // List of labels set to this container
	StopSignal      string                  `json:",omitempty" yaml:"stopSignal,omitempty"`      // Signal to stop a container
	StopTimeout     *int                    `json:",omitempty" yaml:"timeout,omitempty"`         // Timeout (in seconds) to stop a container
}

func (c *Config) ToAPI() *container.Config {
	res := &container.Config{}
	if err := mergo.Map(res, c); err != nil {
		panic(err)
	}
	return res
}

// Resources contains container's resources (cgroups config, ulimits...)
type Resources struct {
	// Applicable to all platforms
	CPUShares int64 `json:"CpuShares" yaml:"cpuShares"` // CPU shares (relative weight vs. other containers)
	Memory    int64 `yaml:"memory,omitempty"`           // Memory limit (in bytes)
	NanoCPUs  int64 `json:"NanoCpus" yaml:"nanoCpus"`   // CPU quota in units of 10<sup>-9</sup> CPUs.

	// Applicable to UNIX platforms
	CgroupParent         string                     `yaml:"cgroupParent,omitempty"`  // Parent cgroup.
	BlkioWeight          uint16                     `yaml:"blockIoWeight,omitempty"` // Block IO weight (relative weight vs. other containers)
	BlkioWeightDevice    []*blkiodev.WeightDevice   `yaml:"blockIoWeightDevice,omitempty"`
	BlkioDeviceReadBps   []*blkiodev.ThrottleDevice `yaml:"blockIoDeviceReadBps,omitempty"`
	BlkioDeviceWriteBps  []*blkiodev.ThrottleDevice `yaml:"blockIoDeviceWriteBps,omitempty"`
	BlkioDeviceReadIOps  []*blkiodev.ThrottleDevice `yaml:"blockIoDeviceReadIOps,omitempty"`
	BlkioDeviceWriteIOps []*blkiodev.ThrottleDevice `yaml:"blockIoDeviceWriteIOps,omitempty"`
	CPUPeriod            int64                      `json:"CpuPeriod" yaml:"cpuPeriod,omitempty"`                   // CPU CFS (Completely Fair Scheduler) period
	CPUQuota             int64                      `json:"CpuQuota" yaml:"cpuQuota,omitempty"`                     // CPU CFS (Completely Fair Scheduler) quota
	CPURealtimePeriod    int64                      `json:"CpuRealtimePeriod" yaml:"cpuRealtimePeriod,omitempty"`   // CPU real-time period
	CPURealtimeRuntime   int64                      `json:"CpuRealtimeRuntime" yaml:"cpuRealtimeRuntime,omitempty"` // CPU real-time runtime
	CpusetCpus           string                     `yaml:"cpusetCpus,omitempty"`                                   // CpusetCpus 0-2, 0,1
	CpusetMems           string                     `yaml:"cpusetMems,omitempty"`                                   // CpusetMems 0-2, 0,1
	Devices              []DeviceMapping            `yaml:"devices,omitempty"`                                      // List of devices to map inside the container
	DiskQuota            int64                      `yaml:"diskQuota,omitempty"`                                    // Disk limit (in bytes)
	KernelMemory         int64                      `yaml:"kernelMemory,omitempty"`                                 // Kernel memory limit (in bytes)
	MemoryReservation    int64                      `yaml:"memoryReservation,omitempty"`                            // Memory soft limit (in bytes)
	MemorySwap           int64                      `yaml:"memorySwap,omitempty"`                                   // Total memory usage (memory + swap); set `-1` to enable unlimited swap
	MemorySwappiness     *int64                     `yaml:"memorySwappiness,omitempty"`                             // Tuning container memory swappiness behaviour
	OomKillDisable       *bool                      `yaml:"oomKillDisable,omitempty"`                               // Whether to disable OOM Killer or not
	PidsLimit            int64                      `yaml:"pidsLimit,omitempty"`                                    // Setting pids limit for a container
	Ulimits              []*units.Ulimit            `yaml:"ulimits,omitempty"`                                      // List of ulimits to be set in the container
}

// RestartPolicy represents the restart policies of the container.
type RestartPolicy struct {
	Name              string `yaml:"name,omitempty"`
	MaximumRetryCount int    `yaml:"maximumRetryCount,omitempty"`
}

// HostConfig the non-portable Config structure of a container.
// Here, "non-portable" means "dependent of the host we are running on".
// Portable information *should* appear in Config.
type HostConfig struct {
	// Applicable to all platforms
	Binds           []string      `yaml:"binds,omitempty"`           // List of volume bindings for this container
	ContainerIDFile string        `yaml:"containerIdFile,omitempty"` // File (path) where the containerId is written
	LogConfig       LogConfig     `yaml:"logConfig,omitempty"`       // Configuration of the logs for this container
	NetworkMode     NetworkMode   `yaml:"networkMode,omitempty"`     // Network mode to use for the container
	PortBindings    nat.PortMap   `yaml:"portBindings,omitempty"`    // Port mapping between the exposed port (container) and the host
	RestartPolicy   RestartPolicy `yaml:"restartPolicy,omitempty"`   // Restart policy to be used for the container
	AutoRemove      bool          `yaml:"autoRemove,omitempty"`      // Automatically remove container when it exits
	VolumeDriver    string        `yaml:"volumeDriver,omitempty"`    // Name of the volume driver used to mount volumes
	VolumesFrom     []string      `yaml:"volumesFrom,omitempty"`     // List of volumes to take from other container

	// Applicable to UNIX platforms
	CapAdd          strslice.StrSlice `yaml:"capAdd,omitempty"`                       // List of kernel capabilities to add to the container
	CapDrop         strslice.StrSlice `yaml:"capDrop,omitempty"`                      // List of kernel capabilities to remove from the container
	DNS             []string          `json:"Dns" yaml:"dns,omitempty"`               // List of DNS server to lookup
	DNSOptions      []string          `json:"DnsOptions" yaml:"dnsOptions,omitempty"` // List of DNSOption to look for
	DNSSearch       []string          `json:"DnsSearch" yaml:"dnsSearch,omitempty"`   // List of DNSSearch to look for
	ExtraHosts      []string          `yaml:"extraHosts,omitempty"`                   // List of extra hosts
	GroupAdd        []string          `yaml:"groupAdd,omitempty"`                     // List of additional groups that the container process will run as
	IpcMode         IpcMode           `yaml:"ipcMode,omitempty"`                      // IPC namespace to use for the container
	Cgroup          CgroupSpec        `yaml:"cgroup,omitempty"`                       // Cgroup to use for the container
	Links           []string          `yaml:"links,omitempty"`                        // List of links (in the name:alias form)
	OomScoreAdj     int               `yaml:"oomScoreAdj,omitempty"`                  // Container preference for OOM-killing
	PidMode         PidMode           `yaml:"pidMode,omitempty"`                      // PID namespace to use for the container
	Privileged      bool              `yaml:"privileged,omitempty"`                   // Is the container in privileged mode
	PublishAllPorts bool              `yaml:"publishAllPorts,omitempty"`              // Should docker publish all exposed port for the container
	ReadonlyRootfs  bool              `yaml:"readonlyRootfs,omitempty"`               // Is the container root filesystem in read-only
	SecurityOpt     []string          `yaml:"securityOpt,omitempty"`                  // List of string values to customize labels for MLS systems, such as SELinux.
	StorageOpt      map[string]string `json:",omitempty" yaml:"storageOpt,omitempty"` // Storage driver options per container.
	Tmpfs           map[string]string `json:",omitempty" yaml:"tmpfs,omitempty"`      // List of tmpfs (mounts) used for the container
	UTSMode         UTSMode           `yaml:"utsMode,omitempty"`                      // UTS namespace to use for the container
	UsernsMode      UsernsMode        `yaml:"userNSMode,omitempty"`                   // The user namespace to use for the container
	ShmSize         int64             `yaml:"shmSize,omitempty"`                      // Total shm memory usage
	Sysctls         map[string]string `json:",omitempty" yaml:"sysCtls,omitempty"`    // List of Namespaced sysctls used for the container
	Runtime         string            `json:",omitempty" yaml:"runtime,omitempty"`    // Runtime to use with this container

	// Contains container's resources (cgroups, ulimits)
	Resources

	// Mounts specs used by the container
	Mounts []mount.Mount `json:",omitempty" yaml:"mounts,omitempty"`

	// Run a custom init inside the container, if null, use the daemon's configured settings
	Init *bool `json:",omitempty" yaml:"init,omitempty"`
}

func (c *HostConfig) ToAPI() *container.HostConfig {
	res := &container.HostConfig{}
	if err := mergo.Map(res, c); err != nil {
		panic(err)
	}
	return res
}

type LogConfig struct {
	Type   string            `yaml:"type,omitempty"`
	Config map[string]string `yaml:"config,omitempty"`
}

// EndpointIPAMConfig represents IPAM configurations for the endpoint
type EndpointIPAMConfig struct {
	IPv4Address  string   `json:",omitempty" yaml:"ipv4Address,omitempty"`
	IPv6Address  string   `json:",omitempty" yaml:"ipv6Address,omitempty"`
	LinkLocalIPs []string `json:",omitempty" yaml:"linkLocalIps,omitempty"`
}

// EndpointSettings stores the network endpoint details
type EndpointSettings struct {
	// Configurations
	IPAMConfig *EndpointIPAMConfig `yaml:"ipamConfig,omitempty"`
	Links      []string            `yaml:"links,omitempty"`
	Aliases    []string            `yaml:"aliases,omitempty"`
	// Operational data
	NetworkID           string `yaml:"networkId,omitempty"`
	EndpointID          string `yaml:"endpointId,omitempty"`
	Gateway             string `yaml:"gateway,omitempty"`
	IPAddress           string `yaml:"ipAddress,omitempty"`
	IPPrefixLen         int    `yaml:"ipPrefixLen,omitempty"`
	IPv6Gateway         string `yaml:"ipv6Gateway,omitempty"`
	GlobalIPv6Address   string `yaml:"globalIpv6Address,omitempty"`
	GlobalIPv6PrefixLen int    `yaml:"globalIpv6PrefixLen,omitempty"`
	MacAddress          string `yaml:"macAddress,omitempty"`
}

// NetworkingConfig represents the container's networking configuration for each of its interfaces
// Carries the networking configs specified in the `docker run` and `docker network connect` commands
type NetworkingConfig struct {
	EndpointsConfig map[string]*EndpointSettings `yaml:"endpointsConfig,omitempty"` // Endpoint configs for each connecting network
}

func (c *NetworkingConfig) ToAPI() *network.NetworkingConfig {
	res := &network.NetworkingConfig{}
	if err := mergo.Map(res, c); err != nil {
		panic(err)
	}
	return res
}
