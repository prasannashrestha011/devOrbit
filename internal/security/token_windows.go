//go:build windows

package security

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

var advapi32 = syscall.NewLazyDLL("advapi32.dll")
var getNamedSecurityInfo = advapi32.NewProc("GetNamedSecurityInfoW")
var convertSDToString = advapi32.NewProc("ConvertSecurityDescriptorToStringSecurityDescriptorW")
var convertStringToSD = advapi32.NewProc("ConvertStringSecurityDescriptorToSecurityDescriptorW")

func currentUserSID() (string, error) {
	token, err := syscall.OpenCurrentProcessToken()
	if err != nil {
		return "", err
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil {
		return "", err
	}
	return user.User.Sid.String()
}

func privateDirectory(path string) error {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil || !strings.EqualFold(resolved, filepath.Clean(path)) {
		return errors.New("token directory redirects through a link")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() {
		return errors.New("token directory is unavailable")
	}
	return privateACL(path)
}

func privateACL(path string) error {
	sid, err := currentUserSID()
	if err != nil {
		return err
	}
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	var sd uintptr
	// SE_FILE_OBJECT, OWNER_SECURITY_INFORMATION | DACL_SECURITY_INFORMATION.
	result, _, _ := getNamedSecurityInfo.Call(uintptr(unsafe.Pointer(name)), 1, 5, 0, 0, 0, 0, uintptr(unsafe.Pointer(&sd)))
	if result != 0 {
		return syscall.Errno(result)
	}
	defer syscall.LocalFree(syscall.Handle(sd))
	var encoded *uint16
	var length uint32
	ok, _, callErr := convertSDToString.Call(sd, 1, 5, uintptr(unsafe.Pointer(&encoded)), uintptr(unsafe.Pointer(&length)))
	if ok == 0 {
		return callErr
	}
	defer syscall.LocalFree(syscall.Handle(unsafe.Pointer(encoded)))
	sddl := syscall.UTF16ToString(unsafe.Slice(encoded, length))
	ownerStart, daclStart := strings.Index(sddl, "O:"), strings.Index(sddl, "D:")
	if ownerStart < 0 || daclStart < 0 || ownerStart >= daclStart {
		return errors.New("token path has no private owner or DACL")
	}
	owner := sddl[ownerStart+2 : daclStart]
	if i := strings.Index(owner, "G:"); i >= 0 {
		owner = owner[:i]
	}
	if owner != sid {
		return errors.New("token path is owned by another account")
	}
	aces := sddl[daclStart+2:]
	if !strings.Contains(aces, "(") {
		return errors.New("token path has no DACL entries")
	}
	for start := strings.IndexByte(aces, '('); start >= 0; start = strings.IndexByte(aces, '(') {
		end := strings.IndexByte(aces[start:], ')')
		if end < 0 {
			return errors.New("invalid token path DACL")
		}
		fields := strings.Split(aces[start+1:start+end], ";")
		if len(fields) != 6 || fields[0] != "A" || fields[5] != sid && fields[5] != "SY" && fields[5] != "BA" && fields[5] != "CO" {
			return errors.New("token path grants access to another account")
		}
		aces = aces[start+end+1:]
	}
	return nil
}

func createToken(path string) (*os.File, error) {
	sid, err := currentUserSID()
	if err != nil {
		return nil, err
	}
	sddl, err := syscall.UTF16PtrFromString("O:" + sid + "D:P(A;;GA;;;SY)(A;;GA;;;" + sid + ")")
	if err != nil {
		return nil, err
	}
	var sd uintptr
	ok, _, callErr := convertStringToSD.Call(uintptr(unsafe.Pointer(sddl)), 1, uintptr(unsafe.Pointer(&sd)), 0)
	if ok == 0 {
		return nil, callErr
	}
	defer syscall.LocalFree(syscall.Handle(sd))
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	sa := syscall.SecurityAttributes{Length: uint32(unsafe.Sizeof(syscall.SecurityAttributes{})), SecurityDescriptor: sd}
	h, err := syscall.CreateFile(name, syscall.GENERIC_WRITE, 0, &sa, syscall.CREATE_NEW, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err == syscall.ERROR_FILE_EXISTS || err == syscall.ERROR_ALREADY_EXISTS {
		return nil, os.ErrExist
	}
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(h), path), nil
}

func openToken(path string) (*os.File, error) {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	h, err := syscall.CreateFile(name, syscall.GENERIC_READ|0x00020000, 0, nil, syscall.OPEN_EXISTING, syscall.FILE_FLAG_OPEN_REPARSE_POINT, 0) // READ_CONTROL
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(h), path)
	var info syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(h, &info); err != nil || info.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 || info.FileAttributes&syscall.FILE_ATTRIBUTE_DIRECTORY != 0 {
		_ = f.Close()
		return nil, errors.New("token path is not a regular file")
	}
	if err := privateACL(path); err != nil {
		_ = f.Close()
		return nil, err
	}
	return f, nil
}
