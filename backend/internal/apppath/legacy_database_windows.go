package apppath

import (
	"fmt"
	"golang.org/x/sys/windows"
)

func holdLegacyDatabaseFiles(paths []string) (func(), error) {
	handles := make([]windows.Handle, 0, len(paths))
	release := func() {
		for _, handle := range handles {
			_ = windows.CloseHandle(handle)
		}
	}
	for _, path := range paths {
		name, err := windows.UTF16PtrFromString(path)
		if err != nil {
			release()
			return nil, err
		}
		handle, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
		if err != nil {
			release()
			return nil, fmt.Errorf("cannot freeze legacy database for copy: %w", err)
		}
		handles = append(handles, handle)
	}
	return release, nil
}
