// Package filenames defines the names of the files that appear under an instance dir
// or inside the config directory.
//
// See docs/internal.md .
package filenames

// Instance names starting with an underscore are reserved for lima internal usage

const (
	ConfigDir   = "_config"
	CacheDir    = "_cache" // not yet implemented
	NetworksDir = "_networks" // network log files are stored here
)

// Filenames used inside the ConfigDir

const (
	UserPrivateKey = "user"
	UserPublicKey  = UserPrivateKey + ".pub"
	NetworksConfig = "networks.yaml"
)

// Filenames that may appear under an instance directory

const (
	LimaYAML           = "lima.yaml"
	CIDataISO          = "cidata.iso"
	BaseDisk           = "basedisk"
	DiffDisk           = "diffdisk"
	QemuPID            = "qemu.pid"
	QMPSock            = "qmp.sock"
	SerialLog          = "serial.log"
	SerialSock         = "serial.sock"
	SSHSock            = "ssh.sock"
	GuestAgentSock     = "ga.sock"
	HostAgentPID       = "ha.pid"
	HostAgentStdoutLog = "ha.stdout.log"
	HostAgentStderrLog = "ha.stderr.log"
)

// LongestSock is the longest socket name.
// On macOS, the full path of the socket (excluding the NUL terminator) must be less than 104 characters.
// See unix(4).
//
// On Linux, the full path must be less than 108 characters.
//
// ssh appends 16 bytes of random characters when it first creates the socket:
// https://github.com/openssh/openssh-portable/blob/V_8_7_P1/mux.c#L1271-L1285
const LongestSock = SSHSock + ".1234567890123456"
