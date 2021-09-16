package limayaml

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"errors"

	"github.com/docker/go-units"
	"github.com/lima-vm/lima/pkg/localpathutil"
	"github.com/lima-vm/lima/pkg/networks"
	"github.com/lima-vm/lima/pkg/osutil"
	"github.com/lima-vm/lima/pkg/qemu/const"
	"github.com/sirupsen/logrus"
)

func Validate(y LimaYAML) error {
	switch y.Arch {
	case X8664, AARCH64:
	default:
		return fmt.Errorf("field `arch` must be %q or %q , got %q", X8664, AARCH64, y.Arch)
	}

	if len(y.Images) == 0 {
		return errors.New("field `images` must be set")
	}
	for i, f := range y.Images {
		if !strings.Contains(f.Location, "://") {
			if _, err := localpathutil.Expand(f.Location); err != nil {
				return fmt.Errorf("field `images[%d].location` refers to an invalid local file path: %q: %w", i, f.Location, err)
			}
			// f.Location does NOT need to be accessible, so we do NOT check os.Stat(f.Location)
		}
		switch f.Arch {
		case X8664, AARCH64:
		default:
			return fmt.Errorf("field `images.arch` must be %q or %q, got %q", X8664, AARCH64, f.Arch)
		}
		if f.Digest != "" {
			if !f.Digest.Algorithm().Available() {
				return fmt.Errorf("field `images[%d].digest` refers to an unavailable digest algorithm", i)
			}
			if err := f.Digest.Validate(); err != nil {
				return fmt.Errorf("field `images[%d].digest` is invalid: %s: %w", i, f.Digest.String(), err)
			}
		}
	}

	if y.CPUs == 0 {
		return errors.New("field `cpus` must be set")
	}

	if _, err := units.RAMInBytes(y.Memory); err != nil {
		return fmt.Errorf("field `memory` has an invalid value: %w", err)
	}

	if _, err := units.RAMInBytes(y.Disk); err != nil {
		return fmt.Errorf("field `memory` has an invalid value: %w", err)
	}

	u, err := osutil.LimaUser(false)
	if err != nil {
		return fmt.Errorf("internal error (not an error of YAML): %w", err)
	}
	// reservedHome is the home directory defined in "cidata.iso:/user-data"
	reservedHome := fmt.Sprintf("/home/%s.linux", u.Username)

	for i, f := range y.Mounts {
		if !filepath.IsAbs(f.Location) && !strings.HasPrefix(f.Location, "~") {
			return fmt.Errorf("field `mounts[%d].location` must be an absolute path, got %q",
				i, f.Location)
		}
		loc, err := localpathutil.Expand(f.Location)
		if err != nil {
			return fmt.Errorf("field `mounts[%d].location` refers to an unexpandable path: %q: %w", i, f.Location, err)
		}
		switch loc {
		case "/", "/bin", "/dev", "/etc", "/home", "/opt", "/sbin", "/tmp", "/usr", "/var":
			return fmt.Errorf("field `mounts[%d].location` must not be a system path such as /etc or /usr", i)
		case reservedHome:
			return fmt.Errorf("field `mounts[%d].location` is internally reserved", i)
		}

		st, err := os.Stat(loc)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("field `mounts[%d].location` refers to an inaccessible path: %q: %w", i, f.Location, err)
			}
		} else if !st.IsDir() {
			return fmt.Errorf("field `mounts[%d].location` refers to a non-directory path: %q: %w", i, f.Location, err)
		}
	}

	if err := validatePort("ssh.localPort", y.SSH.LocalPort); err != nil {
		return err
	}

	// y.Firmware.LegacyBIOS is ignored for aarch64, but not a fatal error.

	for i, p := range y.Provision {
		switch p.Mode {
		case ProvisionModeSystem, ProvisionModeUser:
		default:
			return fmt.Errorf("field `provision[%d].mode` must be either %q or %q",
				i, ProvisionModeSystem, ProvisionModeUser)
		}
	}
	for i, p := range y.Probes {
		switch p.Mode {
		case ProbeModeReadiness:
		default:
			return fmt.Errorf("field `probe[%d].mode` can only be %q",
				i, ProbeModeReadiness)
		}
	}
	for i, rule := range y.PortForwards {
		field := fmt.Sprintf("portForwards[%d]", i)
		if rule.GuestPort != 0 {
			if rule.GuestPort != rule.GuestPortRange[0] {
				return fmt.Errorf("field `%s.guestPort` must match field `%s.guestPortRange[0]`", field, field)
			}
			// redundant validation to make sure the error contains the correct field name
			if err := validatePort(field+".guestPort", rule.GuestPort); err != nil {
				return err
			}
		}
		if rule.HostPort != 0 {
			if rule.HostPort != rule.HostPortRange[0] {
				return fmt.Errorf("field `%s.hostPort` must match field `%s.hostPortRange[0]`", field, field)
			}
			// redundant validation to make sure the error contains the correct field name
			if err := validatePort(field+".hostPort", rule.HostPort); err != nil {
				return err
			}
		}
		for j := 0; j < 2; j++ {
			if err := validatePort(fmt.Sprintf("%s.guestPortRange[%d]", field, j), rule.GuestPortRange[j]); err != nil {
				return err
			}
			if err := validatePort(fmt.Sprintf("%s.hostPortRange[%d]", field, j), rule.HostPortRange[j]); err != nil {
				return err
			}
		}
		if rule.GuestPortRange[0] > rule.GuestPortRange[1] {
			return fmt.Errorf("field `%s.guestPortRange[1]` must be greater than or equal to field `%s.guestPortRange[0]`", field, field)
		}
		if rule.HostPortRange[0] > rule.HostPortRange[1] {
			return fmt.Errorf("field `%s.hostPortRange[1]` must be greater than or equal to field `%s.hostPortRange[0]`", field, field)
		}
		if rule.GuestPortRange[1]-rule.GuestPortRange[0] != rule.HostPortRange[1]-rule.HostPortRange[0] {
			return fmt.Errorf("field `%s.hostPortRange` must specify the same number of ports as field `%s.guestPortRange`", field, field)
		}
		if rule.Proto != TCP {
			return fmt.Errorf("field `%s.proto` must be %q", field, TCP)
		}
		// Not validating that the various GuestPortRanges and HostPortRanges are not overlapping. Rules will be
		// processed sequentially and the first matching rule for a guest port determines forwarding behavior.
	}

	if err := validateNetwork(y); err != nil {
		return err
	}

	return nil
}

func validateNetwork(y LimaYAML) error {
	if len(y.Network.VDEDeprecated) > 0 {
		if y.Network.migrated {
			logrus.Warnf("field `network.VDE` is deprecated; please use `networks` instead")
		} else {
			return fmt.Errorf("you cannot use deprecated field `network.VDE` together with replacement field `networks`")
		}
	}
	interfaceName := make(map[string]int)
	for i, nw := range y.Networks {
		field := fmt.Sprintf("networks[%d]", i)
		if nw.Lima != "" {
			if runtime.GOOS != "darwin" {
				return fmt.Errorf("field `%s.lima` is only supported on macOS right now", field)
			}
			if nw.VNL != "" {
				return fmt.Errorf("field `%s.lima` and field `%s.vnl` are mutually exclusive", field, field)
			}
			if nw.SwitchPort != 0 {
				return fmt.Errorf("field `%s.switchPort` cannot be used with field `%s.lima`", field, field)
			}
			config, err := networks.Config()
			if err != nil {
				return err
			}
			if config.Check(nw.Lima) != nil {
				return fmt.Errorf("field `%s.lima` references network %q which is not defined in networks.yaml", field, nw.Lima)
			}
		} else {
			if nw.VNL == "" {
				return fmt.Errorf("field `%s.lima` or field `%s.vnl` must be set", field, field)
			}
			// The field is called VDE.VNL in anticipation of QEMU upgrading VDE2 to VDEplug4,
			// but right now the only valid value on macOS is a path to the vde_switch socket directory,
			// optionally with vde:// prefix.
			if !strings.Contains(nw.VNL, "://") || strings.HasPrefix(nw.VNL, "vde://") {
				vdeSwitch := strings.TrimPrefix(nw.VNL, "vde://")
				if fi, err := os.Stat(vdeSwitch); err != nil {
					// negligible when the instance is stopped
					logrus.WithError(err).Debugf("field `%s.vnl` %q failed stat", field, vdeSwitch)
				} else {
					if fi.IsDir() {
						/* Switch mode (vdeSwitch is dir, port != 65535) */
						ctlSocket := filepath.Join(vdeSwitch, "ctl")
						// ErrNotExist during os.Stat(ctlSocket) can be ignored. ctlSocket does not need to exist until actually starting the VM
						if fi, err = os.Stat(ctlSocket); err == nil {
							if fi.Mode()&os.ModeSocket == 0 {
								return fmt.Errorf("field `%s.vnl` file %q is not a UNIX socket", field, ctlSocket)
							}
						}
						if nw.SwitchPort == 65535 {
							return fmt.Errorf("field `%s.vnl` points to a non-PTP switch, so the port number must not be 65535", field)
						}
					} else {
						/* PTP mode (vdeSwitch is socket, port == 65535) */
						if fi.Mode()&os.ModeSocket == 0 {
							return fmt.Errorf("field `%s.vnl` %q is not a directory nor a UNIX socket", field, vdeSwitch)
						}
						if nw.SwitchPort != 65535 {
							return fmt.Errorf("field `%s.vnl` points to a PTP (switchless) socket %q, so the port number has to be 65535 (got %d)",
								field, vdeSwitch, nw.SwitchPort)
						}
					}
				}
			} else if runtime.GOOS != "linux" {
				logrus.Warnf("field `%s.vnl` is unlikely to work for %s (unless libvdeplug4 has been ported to %s and is installed)",
					field, runtime.GOOS, runtime.GOOS)
			}
		}
		if nw.MACAddress != "" {
			hw, err := net.ParseMAC(nw.MACAddress)
			if err != nil {
				return fmt.Errorf("field `vmnet.mac` invalid: %w", err)
			}
			if len(hw) != 6 {
				return fmt.Errorf("field `%s.macAddress` must be a 48 bit (6 bytes) MAC address; actual length of %q is %d bytes", field, nw.MACAddress, len(hw))
			}
		}
		// FillDefault() will make sure that nw.Interface is not the empty string
		if len(nw.Interface) >= 16 {
			return fmt.Errorf("field `%s.interface` must be less than 16 bytes, but is %d bytes: %q", field, len(nw.Interface), nw.Interface)
		}
		if strings.ContainsAny(nw.Interface, " \t\n/") {
			return fmt.Errorf("field `%s.interface` must not contain whitespace or slashes", field)
		}
		if nw.Interface == qemu.SlirpNICName {
			return fmt.Errorf("field `%s.interface` must not be set to %q because it is reserved for slirp", field, qemu.SlirpNICName)
		}
		if prev, ok := interfaceName[nw.Interface]; ok {
			return fmt.Errorf("field `%s.interface` value %q has already been used by field `network.vde[%d].name`", field, nw.Interface, prev)
		}
		interfaceName[nw.Interface] = i
	}
	return nil
}

func validatePort(field string, port int) error {
	switch {
	case port < 0:
		return fmt.Errorf("field `%s` must be > 0", field)
	case port == 0:
		return fmt.Errorf("field `%s` must be set", field)
	case port == 22:
		return fmt.Errorf("field `%s` must not be 22", field)
	case port > 65535:
		return fmt.Errorf("field `%s` must be < 65536", field)
	}
	return nil
}
