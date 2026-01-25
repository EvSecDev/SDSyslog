package ebpf

const (
	DrainSocket           int    = 1
	DrainMapName          string = "draining_sockets"
	DrainFuncName         string = "reuseport_select"
	KernelDrainMapPath    string = "/sys/fs/bpf/" + DrainMapName
	KernelSocketRouteFunc string = "/sys/fs/bpf/" + DrainFuncName
)
