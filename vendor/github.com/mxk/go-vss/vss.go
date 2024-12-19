//go:build windows

// Package vss exposes Windows Volume Shadow Copy API.
//
// Operations on shadow copies require the process to be running with elevated
// privileges of a user who is a member of the Administrators group. Returned
// errors will contain os.ErrPermission in their tree to indicate insufficient
// privileges.
package vss

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/go-ole/go-ole"
	"golang.org/x/sys/windows"
)

// errNotAdmin is returned when the current user lacks admin privileges.
var errNotAdmin = fmt.Errorf("vss: do not have Administrators group privileges (%w)",
	os.ErrPermission)

// Create creates a new shadow copy of the specified volume and returns its ID.
// The volume can be specified by its drive letter (e.g. "C:"), mount point, or
// globally unique identifier (GUID) name (`\\?\Volume{GUID}\`). The returned
// error will contain os.ErrPermission if the current user does not have
// Administrators group privileges.
func Create(vol string) (string, error) {
	if !isAdmin() {
		return "", errNotAdmin
	}
	var id *ole.GUID
	err := wmiExec(func(s *sWbemServices) (err error) {
		id, err = create(s, vol)
		return
	})
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

// CreateLink creates a new shadow copy and symlinks it at the specified path.
// The shadow copy is removed if symlinking fails.
func CreateLink(link, vol string) (err error) {
	id, err := Create(vol)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = Remove(id)
		}
	}()
	sc, _, err := get(id)
	if err != nil {
		return err
	}
	return sc.Link(link)
}

// IsShadowCopy returns whether name is a path referring to the contents of a
// shadow copy.
func IsShadowCopy(name string) (bool, error) {
	// https://github.com/golang/go/issues/63703#issuecomment-1872960199
	if isShadowPath(name) {
		return true, nil
	}
	if target, err := readlink(name); err == nil {
		if name = target; isShadowPath(target) {
			return true, nil
		}
	}
	name, err := resolveDevice(name)
	return isShadowPath(name), err
}

// Remove removes a shadow copy by ID, DeviceObject, or symlink path. If a valid
// symlink is specified, then it is also removed.
func Remove(name string) error {
	if !isAdmin() {
		return errNotAdmin
	}
	if id := ole.NewGUID(name); id != nil {
		return (&ShadowCopy{ID: id.String()}).Remove()
	}
	sc, symlink, err := get(name)
	if err != nil {
		return err
	}
	if err = sc.Remove(); err == nil && symlink != "" {
		err = rmdir(symlink)
	}
	return err
}

// SplitVolume splits an absolute file path into its volume mount point and the
// path relative to the mount. For example, "C:\Windows\System32" returns "C:\"
// and "Windows\System32".
func SplitVolume(name string) (vol, rel string, err error) {
	if name = filepath.Clean(name); !filepath.IsAbs(name) {
		// We don't want GetVolumePathName returning the boot volume for
		// relative paths.
		return "", "", fmt.Errorf("vss: non-absolute path: %s", name)
	}
	buf := make([]uint16, max(len(name), syscall.MAX_PATH))
	if err = windows.GetVolumePathName(utf16Ptr(name), &buf[0], uint32(len(buf))); err != nil {
		return "", "", fmt.Errorf("vss: GetVolumePathName failed for: %s (%w)", name, err)
	}
	vol = syscall.UTF16ToString(buf[:])
	rel, err = filepath.Rel(vol, name)
	return
}

// ShadowCopy is an instance of Win32_ShadowCopy class. See:
// https://learn.microsoft.com/en-us/previous-versions/windows/desktop/legacy/aa394428(v=vs.85)
type ShadowCopy struct {
	ID           string
	InstallDate  time.Time
	DeviceObject string
	VolumeName   string
}

const scSelect = "SELECT ID,InstallDate,DeviceObject,VolumeName FROM Win32_ShadowCopy"

// unpack converts Win32_ShadowCopy object into ShadowCopy.
func unpack(v *ole.IDispatch) (*ShadowCopy, error) {
	sc := new(ShadowCopy)
	if err := getProp(v, "ID", &sc.ID); err != nil {
		return nil, err
	}
	tryGetProp(v, "DeviceObject", &sc.DeviceObject)
	tryGetProp(v, "InstallDate", &sc.InstallDate)
	tryGetProp(v, "VolumeName", &sc.VolumeName)
	return sc, nil
}

// Get returns a ShadowCopy by ID, DeviceObject, or symlink path.
func Get(name string) (*ShadowCopy, error) {
	if !isAdmin() {
		return nil, errNotAdmin
	}
	sc, _, err := get(name)
	return sc, err
}

// get returns a ShadowCopy by ID, DeviceObject, or symlink path. If name is a
// symlink, then it also returns the cleaned path.
func get(name string) (sc *ShadowCopy, symlink string, err error) {
	var wql string
	if id := ole.NewGUID(name); id != nil {
		wql = fmt.Sprintf(scSelect+" WHERE ID=%q", id.String())
	} else {
		if name = filepath.Clean(name); !isShadowPath(name) {
			dev, err := readlink(name)
			if err != nil {
				return nil, "", fmt.Errorf("vss: not a symlink: %s (%w)", name, err)
			}
			if !isShadowPath(dev) {
				return nil, "", fmt.Errorf("vss: not a shadow copy symlink: %s", name)
			}
			symlink, name = name, dev
		}
		wql = fmt.Sprintf(scSelect+" WHERE DeviceObject=%q", strings.TrimSuffix(normShadowPath(name), `\`))
	}
	err = wmiExec(func(s *sWbemServices) (err error) {
		sc, err = queryOne(s, wql, unpack)
		return
	})
	return
}

// List returns existing shadow copies. If vol is non-empty, only shadow copies
// for the specified volume are turned.
func List(vol string) ([]*ShadowCopy, error) {
	if !isAdmin() {
		return nil, errNotAdmin
	}
	var wql = scSelect
	if vol != "" {
		vol, err := volumeName(vol)
		if err != nil {
			return nil, err
		}
		wql = fmt.Sprintf(scSelect+" WHERE VolumeName=%q", vol)
	}
	var all []*ShadowCopy
	err := wmiExec(func(s *sWbemServices) error {
		return s.execQuery(wql, func(v *ole.IDispatch) error {
			sc, err := unpack(v)
			if err == nil {
				all = append(all, sc)
			}
			return err
		})
	})
	return all, err
}

// Link creates a directory symlink pointing to the contents of the shadow copy.
func (sc *ShadowCopy) Link(name string) error {
	return syscall.CreateSymbolicLink(utf16Ptr(name), utf16Ptr(sc.DeviceObject+`\`),
		syscall.SYMBOLIC_LINK_FLAG_DIRECTORY)
}

// Remove removes the shadow copy.
func (sc *ShadowCopy) Remove() error {
	if !isAdmin() {
		return errNotAdmin
	}
	return wmiExec(func(s *sWbemServices) error {
		_, err := s.CallMethod("Delete", fmt.Sprintf("Win32_ShadowCopy.ID=%q", sc.ID))
		if err != nil {
			err = fmt.Errorf("vss: failed to remove shadow copy ID %s (%w)", sc.ID, err)
		}
		return err
	})
}

// VolumePath returns the drive letter and/or folder where the shadow copy's
// original volume is mounted. If the volume is mounted at multiple locations,
// only the first one is returned.
func (sc *ShadowCopy) VolumePath() (string, error) {
	m, err := volumePaths(sc.VolumeName)
	if err != nil || len(m) == 0 {
		return "", err
	}
	return m[0], nil
}

// isAdmin returns whether the current thread is a member of the Administrators
// group.
var isAdmin = sync.OnceValue(func() bool {
	// https://learn.microsoft.com/en-us/windows/win32/api/securitybaseapi/nf-securitybaseapi-checktokenmembership#examples
	var AdministratorsGroup *windows.SID
	err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0,
		&AdministratorsGroup,
	)
	if err != nil {
		return false
	}
	defer func() {
		if err := windows.FreeSid(AdministratorsGroup); err != nil {
			panic(err)
		}
	}()
	ok, err := windows.Token(0).IsMember(AdministratorsGroup)
	return ok && err == nil
})

// CreateError is an error code returned by Win32_ShadowCopy.Create. See:
// https://learn.microsoft.com/en-us/previous-versions/windows/desktop/vsswmi/create-method-in-class-win32-shadowcopy#return-value
type CreateError uint32

// Error implements the error interface.
func (e CreateError) Error() string {
	switch e {
	case 0:
		return "Success"
	case 1:
		return "Access denied"
	case 2:
		return "Invalid argument"
	case 3:
		return "Specified volume not found"
	case 4:
		return "Specified volume not supported"
	case 5:
		return "Unsupported shadow copy context"
	case 6:
		return "Insufficient storage"
	case 7:
		return "Volume is in use"
	case 8:
		return "Maximum number of shadow copies reached"
	case 9:
		return "Another shadow copy operation is already in progress"
	case 10:
		return "Shadow copy provider vetoed the operation"
	case 11:
		return "Shadow copy provider not registered"
	case 12:
		return "Shadow copy provider failure"
	case 13:
		return "Unknown error"
	}
	return ""
}

// Unwrap implements errors.Unwrap interface.
func (e CreateError) Unwrap() error {
	switch e {
	case 1:
		return os.ErrPermission
	case 2:
		return os.ErrInvalid
	case 3:
		return os.ErrNotExist
	}
	return nil
}

// create creates a new shadow copy of the specified volume and returns its ID.
func create(s *sWbemServices, vol string) (*ole.GUID, error) {
	if vol = filepath.FromSlash(vol); vol != "" && vol[len(vol)-1] != '\\' {
		vol += `\` // Trailing separator is required
	}
	sc, err := s.CallMethod("Get", "Win32_ShadowCopy")
	if err != nil {
		return nil, fmt.Errorf("vss: failed to get Win32_ShadowCopy (%w)", err)
	}
	defer mustClear(sc)
	var id string
	rc, err := sc.ToIDispatch().CallMethod("Create", vol, "ClientAccessible", &id)
	if err != nil {
		return nil, fmt.Errorf("vss: Win32_ShadowCopy.Create(%#q) failed (%w)", vol, err)
	}
	if g := ole.NewGUID(id); rc.Val == 0 && g != nil {
		return g, nil
	}
	return nil, fmt.Errorf("vss: Win32_ShadowCopy.Create(%#q) returned %d (%w)",
		vol, rc.Val, CreateError(rc.Val))
}

// readlink returns the destination of the named symbolic link.
func readlink(name string) (string, error) {
	for bufSize := syscall.MAX_PATH; ; bufSize *= 2 {
		buf := make([]byte, bufSize)
		n, err := syscall.Readlink(name, buf)
		if err != nil {
			return "", err
		}
		if n < len(buf) {
			if name = filepath.Clean(string(buf[:n])); isShadowPath(name) {
				name = normShadowPath(name)
			}
			return name, nil
		}
	}
}

// rmdir removes the named directory, which may be a symlink.
func rmdir(name string) error {
	return syscall.RemoveDirectory(utf16Ptr(name))
}

// resolveDevice resolves any symbolic links in name and returns the full
// canonical path using device volume name.
func resolveDevice(name string) (string, error) {
	const access = 0
	const share = syscall.FILE_SHARE_READ | syscall.FILE_SHARE_WRITE
	const create = syscall.OPEN_EXISTING
	// See https://docs.microsoft.com/en-us/windows/desktop/FileIO/symbolic-link-effects-on-file-systems-functions#createfile-and-createfiletransacted
	const flag = syscall.FILE_FLAG_BACKUP_SEMANTICS | syscall.FILE_FLAG_OPEN_REPARSE_POINT
	h, err := windows.CreateFile(utf16Ptr(name), access, share, nil, create, flag, 0)
	if err != nil {
		return "", err
	}
	defer func() { _ = windows.CloseHandle(h) }()
	bufSize := uint32(2 * syscall.MAX_PATH)
	for range [2]struct{}{} {
		buf := make([]uint16, bufSize)
		n, err := windows.GetFinalPathNameByHandle(h, &buf[0], bufSize, 0x2) // VOLUME_NAME_NT
		if err != nil {
			return "", err
		}
		if bufSize = n; n < uint32(len(buf)) {
			return syscall.UTF16ToString(buf[:n]), nil
		}
	}
	panic("vss: should not happen")
}

// volumeName converts a drive letter or a mounted folder to `\\?\Volume{GUID}\`
// format. If vol is already in the GUID format, it is returned unmodified,
// except for the addition of a trailing slash.
func volumeName(name string) (string, error) {
	const volLen = len(`\\?\Volume{xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx}\`)
	if name = filepath.FromSlash(name); name != "" && name[len(name)-1] != '\\' {
		name += `\` // Trailing separator is required
	}
	if len(name) != volLen || !hasPrefixFold(name, `\\?\Volume{`) {
		var buf [volLen + 1]uint16
		err := windows.GetVolumeNameForVolumeMountPoint(utf16Ptr(name), &buf[0], uint32(len(buf)))
		if err != nil {
			return "", fmt.Errorf("vss: failed to get volume name of %#q (%w)", name, err)
		}
		name = syscall.UTF16ToString(buf[:])
	}
	return name, nil
}

// volumePaths returns all mount points for the specified volume name.
func volumePaths(vol string) ([]string, error) {
	var buf [2 * syscall.MAX_PATH]uint16
	var n uint32
	err := windows.GetVolumePathNamesForVolumeName(utf16Ptr(vol), &buf[0], uint32(len(buf)), &n)
	if err != nil || len(buf) < int(n) {
		return nil, fmt.Errorf("vss: failed to get volume paths for %#q (%w)", vol, err)
	}
	var all []string
	for b := buf[:n]; len(b) > 1; {
		i := 0
		for i < len(b) && b[i] != 0 {
			i++
		}
		all = append(all, syscall.UTF16ToString(b[:i]))
		b = b[min(i+1, len(b)):]
	}
	return all, nil
}

// isShadowPath returns whether s is a shadow copy path.
func isShadowPath(s string) bool {
	return trimShadowPath(s) != ""
}

// normShadowPath ensures that s starts with the canonical shadow copy
// DeviceObject name. It returns an empty string if s does not refer to a shadow
// copy.
func normShadowPath(s string) string {
	const prefix = `\\?\GLOBALROOT\Device\HarddiskVolumeShadowCopy`
	if hasPrefixFold(s, prefix) {
		return s
	}
	if s = trimShadowPath(s); s != "" {
		return prefix + s
	}
	return ""
}

// trimShadowPath removes the shadow copy device prefix just before the volume
// number. It returns an empty string if s does not refer to a shadow copy.
func trimShadowPath(s string) string {
	s, _ = strings.CutPrefix(s, `\\?\`)
	if hasPrefixFold(s, `GLOBALROOT\`) {
		s = s[len(`GLOBALROOT`):]
	}
	const prefix = `\Device\HarddiskVolumeShadowCopy`
	if hasPrefixFold(s, prefix) {
		return s[len(prefix):]
	}
	return ""
}

// hasPrefixFold tests whether s begins with an ASCII-only prefix ignoring case.
func hasPrefixFold(s, prefix string) bool {
	return len(s) >= len(prefix) && strings.EqualFold(s[0:len(prefix)], prefix)
}

// utf16Ptr converts s to UTF-16 format for Windows API calls. It panics if s
// contains any NUL bytes.
func utf16Ptr(s string) *uint16 {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		panic("vss: string with NUL passed to UTF16PtrFromString")
	}
	return p
}
