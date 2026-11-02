package storebridge

import (
	"context"
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

func listenerOwnershipSupported() bool { return true }
func safeRuntimeAncestors(path string) bool {
	if len(path) > 4096 || !filepath.IsAbs(path) || strings.HasPrefix(path, `\\`) {
		return false
	}
	root := filepath.VolumeName(path) + string(filepath.Separator)
	parts := strings.Split(strings.TrimPrefix(path, root), string(filepath.Separator))
	if len(parts) > 64 {
		return false
	}
	for _, part := range parts {
		root = filepath.Join(root, part)
		p, err := syscall.UTF16PtrFromString(root)
		if err != nil {
			return false
		}
		attributes, err := syscall.GetFileAttributes(p)
		if err != nil || attributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
			return false
		}
	}
	return true
}
func hideOwnedProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
func openRuntime(path string) (*os.File, error) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, ErrRequired
	}
	h, err := syscall.CreateFile(p, syscall.GENERIC_READ, syscall.FILE_SHARE_READ, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, ErrRequired
	}
	return os.NewFile(uintptr(h), path), nil
}

var tcpTable = syscall.NewLazyDLL("iphlpapi.dll").NewProc("GetExtendedTcpTable")

func ownsListener(ctx context.Context, pid, port int) bool {
	if ctx.Err() != nil {
		return false
	}
	var size uint32
	result, _, _ := tcpTable.Call(0, uintptr(unsafe.Pointer(&size)), 0, 2, 3, 0)
	if result != 122 && result != 0 || size < 4 || size > 1<<20 {
		return false
	}
	buffer := make([]byte, size)
	result, _, _ = tcpTable.Call(uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&size)), 0, 2, 3, 0)
	if result != 0 || size < 4 || int(size) > len(buffer) {
		return false
	}
	count := int(binary.LittleEndian.Uint32(buffer[:4]))
	if count > (int(size)-4)/24 {
		return false
	}
	for i := 0; i < count; i++ {
		row := buffer[4+i*24 : 4+(i+1)*24]
		if binary.LittleEndian.Uint32(row[:4]) == 2 && row[4] == 127 && row[5] == 0 && row[6] == 0 && row[7] == 1 && int(binary.BigEndian.Uint16(row[8:10])) == port && int(binary.LittleEndian.Uint32(row[20:24])) == pid {
			return true
		}
	}
	return false
}
