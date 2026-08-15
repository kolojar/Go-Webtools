//go:build windows
// +build windows

package filesystem

import (
	"os"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/kolojar/Go-Webtools/helpertools"
)

type fileSystemWatcherInstance struct {
	mainParent *FileSystemWatcher
	path       string
	recursive  bool
	isRunning  bool
	stopped    bool
	handle     syscall.Handle
	pathsInFS  helpertools.SafeMap[string, bool]
	//subDirectoriesWatchers helpertools.SafeMap[string, *fileSystemWatcherInstance]
}

func newFileSystemWatcherInstance(path string, parent *FileSystemWatcher, recursive bool) *fileSystemWatcherInstance {
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	path = strings.ReplaceAll(path, "/", `\`)
	return &fileSystemWatcherInstance{isRunning: false, mainParent: parent, path: path, recursive: recursive, pathsInFS: helpertools.MakeSafeMap[string, bool]()}
}

// WARNING: CONTAINS UNSAFE POINTER CALLING
func (watcher *fileSystemWatcherInstance) watchingLoop() error {
	//Make path
	path, err := syscall.UTF16PtrFromString(watcher.path)
	if err != nil {
		return err
	}

	//Make handle
	watcher.handle, err = syscall.CreateFile(
		path,
		syscall.FILE_LIST_DIRECTORY,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil,
		syscall.OPEN_EXISTING,
		syscall.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		return err
	}
	defer syscall.CloseHandle(watcher.handle)

	//Start reading
	buffer := make([]byte, 4096)
	for watcher.isRunning {
		var bytesReturned uint32

		//Read changes
		err = syscall.ReadDirectoryChanges(
			watcher.handle,
			&buffer[0],
			uint32(len(buffer)),
			watcher.recursive,
			syscall.FILE_NOTIFY_CHANGE_FILE_NAME|syscall.FILE_NOTIFY_CHANGE_DIR_NAME|syscall.FILE_NOTIFY_CHANGE_ATTRIBUTES|syscall.FILE_NOTIFY_CHANGE_SIZE|syscall.FILE_NOTIFY_CHANGE_LAST_WRITE,
			&bytesReturned,
			nil, uintptr(0),
		)
		if err != nil {
			watcher.mainParent.Logger.Log(1, "Got runtime error, continuing: "+err.Error())
			//return err
			continue
		}

		//Handle type
		offset := 0
		for {
			if offset >= int(bytesReturned) {
				break
			}

			//Convert to file name
			info := (*syscall.FileNotifyInformation)(unsafe.Pointer(&buffer[offset]))
			fileName := syscall.UTF16ToString((*[4096]uint16)(unsafe.Pointer(&info.FileName))[:info.FileNameLength/2])

			//Check if it is dir
			pathToFile := watcher.path + fileName
			fileInfo, err := os.Stat(pathToFile)
			isDir := false
			if err == nil {
				//watcher.mainParent.Logger.Log(1, "Got runtime error, continuing: "+err.Error())
				//continue
				//return err
				isDir = fileInfo.IsDir()
				watcher.pathsInFS.Set(pathToFile, isDir)
			} else {
				isDir = watcher.pathsInFS.Get(pathToFile)
			}

			//Sort event types
			println(info.Action)
			switch info.Action {
			case syscall.FILE_ACTION_ADDED:
				{
					watcher.reportEvent(2, pathToFile, isDir)
				}
			case syscall.FILE_ACTION_REMOVED:
				{
					watcher.reportEvent(3, pathToFile, isDir)
				}
			case syscall.FILE_ACTION_MODIFIED:
				{
					watcher.reportEvent(1, pathToFile, isDir)
				}
			case syscall.FILE_ACTION_RENAMED_OLD_NAME:
				{
					watcher.reportEvent(100, pathToFile, isDir)
				}
			case syscall.FILE_ACTION_RENAMED_NEW_NAME:
				{
					watcher.reportEvent(101, pathToFile, isDir)
				}
			}
			// Finish loop
			if info.NextEntryOffset == 0 {
				break
			}
			offset += int(info.NextEntryOffset)
		}
	}
	return nil
}

/*
Starts watching for file changes, locks execution thread
*/
func (watcher *fileSystemWatcherInstance) StartWatching() {
	//Get watcher ready
	if watcher.isRunning {
		//Ignore running watcher
		return
	}
	watcher.stopped = false
	watcher.isRunning = true

	//List dirs and files
	watcher.listDirsAndFiles(watcher.path)

	//Start watcher
	watcher.mainParent.Logger.Log(2, "Started watching in path: "+watcher.path)
	err := watcher.watchingLoop()
	if err != nil {
		watcher.mainParent.Logger.Log(3, "Error while watching: "+err.Error())
	}
	watcher.mainParent.Logger.Log(1, "Watcher exited.")
	watcher.stopped = true
}

/*
Requests stop for watching
*/
func (watcher *fileSystemWatcherInstance) StopWatching() {
	if watcher == nil {
		return
	}
	//Stop main watcher
	if watcher.isRunning == false {
		return
	}
	watcher.mainParent.Logger.Log(2, "Requesting stop of watching for path: "+watcher.path)
	watcher.isRunning = false

	//Stop
	err := syscall.CloseHandle(watcher.handle)
	time.Sleep(1 * time.Second)
	if err != nil {
		watcher.mainParent.Logger.Log(3, "Error stopping watcher for path: "+watcher.path+" | Error: "+err.Error())
		watcher.stopped = true
	}
	for !watcher.stopped {
		time.Sleep(1 * time.Second)
	}
	watcher.mainParent.Logger.Log(1, "Watching stopped for path: "+watcher.path)
}

func (watcher *fileSystemWatcherInstance) reportEvent(operation uint8, path string, isDir bool) {
	/*if isDir && watcher.recursive {
		//Do operations with subListeners
		if operation == 3 || operation == 100 {
			//Remove
			go watcher.subDirectoriesWatchers.Get(path).StopWatching()
		} else if operation == 2 || operation == 101 {
			//Add
			w := newFileSystemWatcherInstance(path, watcher.mainParent, watcher.mainParent.recursive)
			watcher.subDirectoriesWatchers.Set(path, w)
			go w.StartWatching()
		}
	}*/
	watcher.mainParent.reportEvent(operation, path, isDir)
}

func (watcher *fileSystemWatcherInstance) listDirsAndFiles(path string) {
	//Get entries
	entries, err := os.ReadDir(path)
	if err != nil {
		watcher.mainParent.Logger.Log(3, "Error listing directory: "+path+" | Error: "+err.Error())
		return
	}

	//Make entries
	for i := 0; i < len(entries); i++ {
		entry := entries[i]
		watcher.pathsInFS.Set(path+`\`+entry.Name(), entry.IsDir())

		//Subentries
		if entry.IsDir() {
			watcher.listDirsAndFiles(path + `\` + entry.Name())
		}
	}
}
