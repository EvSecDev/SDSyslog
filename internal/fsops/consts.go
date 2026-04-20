package fsops

const (
	// Linux Capabilities
	xattrName = "security.capability"

	// VFS capability revisions (v3 not used for our purposes)
	vfsCapRevision2 = 0x02000000

	// Flags (lower bits of magic)
	vfsCapFlagsEffective = 0x00000001

	CapEffective   CapMode = 1 << 0 // +e
	CapPermitted   CapMode = 1 << 1 // +p
	CapInheritable CapMode = 1 << 2 // +i

	CapChown          uint = 0
	CapDACOverride    uint = 1
	CapDACReadSearch  uint = 2
	CapFOwner         uint = 3
	CapFSETID         uint = 4
	CapKill           uint = 5
	CapSetGID         uint = 6
	CapSetUID         uint = 7
	CapSetPCAP        uint = 8
	CapLinuxImmutable uint = 9
	CapNetBindService uint = 10
	CapNetBroadcast   uint = 11
	CapNetAdmin       uint = 12
	CapNetRaw         uint = 13
	CapIPCLock        uint = 14
	CapIPCOwner       uint = 15
	CapSYSModule      uint = 16
	CapSYSRawio       uint = 17
	CapSYSChroot      uint = 18
	CapSYSPtrace      uint = 19
	CapSYSPAcct       uint = 20
	CapSYSAdmin       uint = 21
	CapSYSBoot        uint = 22
	CapSYSNice        uint = 23
	CapSYSResource    uint = 24
	CapSYSTime        uint = 25
	CapSYSTTYConfig   uint = 26
	CapMKNOD          uint = 27
	CapLease          uint = 28
	CapAuditWrite     uint = 29
	CapAuditControl   uint = 30
	CapSetFCAP        uint = 31
	CapMACOverride    uint = 32
	CapMACAdmin       uint = 33
	CapSyslog         uint = 34
	CapWakeAlarm      uint = 35
	CapBlockSuspend   uint = 36
	CapAuditRead      uint = 37

	CapBPF uint = 39
)
