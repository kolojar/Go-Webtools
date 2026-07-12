//go:build linux
// +build linux

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
	inotifyBase            int
	subDirectoriesWatchers helpertools.SafeMap[string, *fileSystemWatcherInstance]
	mainParent             *FileSystemWatcher
	isRunning              bool
	stopped                bool
	path                   string
	recursive              bool
	inotifyWatcher         int
}

func newFileSystemWatcherInstance(path string, parent *FileSystemWatcher, recursive bool) *fileSystemWatcherInstance {
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	return &fileSystemWatcherInstance{isRunning: false, mainParent: parent, subDirectoriesWatchers: helpertools.MakeSafeMap[string, *fileSystemWatcherInstance](), path: path, recursive: recursive}
}

// WARNING: CONTAINS UNSAFE POINTER CALLING
func (watcher *fileSystemWatcherInstance) watchingLoop() error {
	//Setp INotify FS watcher
	var err error
	watcher.inotifyBase, err = syscall.InotifyInit()
	if err != nil {
		return err
	}

	//Add watcher to syscall
	watcher.inotifyWatcher, err = syscall.InotifyAddWatch(watcher.inotifyBase, watcher.path, syscall.IN_MODIFY|syscall.IN_CREATE|syscall.IN_DELETE|syscall.IN_OPEN|syscall.IN_MOVE|syscall.IN_DELETE_SELF)
	if err != nil {
		return err
	}
	defer syscall.InotifyRmWatch(watcher.inotifyBase, uint32(watcher.inotifyWatcher))
	buffer := make([]byte, 8192)

	//Start loop
	for watcher.isRunning {
		//Read data
		n, err := syscall.Read(watcher.inotifyBase, buffer)
		if err != nil {
			return err
		}

		//Sort operations
		var offset uint32
		for offset <= uint32(n-syscall.SizeofInotifyEvent) {
			rawEvent := (*syscall.InotifyEvent)(unsafe.Pointer(&buffer[offset]))
			nameOfFileLenght := rawEvent.Len
			filePath := ""
			if nameOfFileLenght > 0 {
				filePath = string(buffer[offset+syscall.SizeofInotifyEvent : offset+syscall.SizeofInotifyEvent+uint32(nameOfFileLenght)])
			}
			filePath = watcher.path + filePath
			filePath = strings.ReplaceAll(filePath, "\x00", "")

			//Sort
			isDir := rawEvent.Mask&syscall.IN_ISDIR != 0
			if rawEvent.Mask&syscall.IN_MODIFY != 0 {
				watcher.reportEvent(1, filePath, isDir)
			} else if rawEvent.Mask&syscall.IN_CREATE != 0 {
				watcher.reportEvent(2, filePath, isDir)
			} else if rawEvent.Mask&syscall.IN_DELETE != 0 {
				watcher.reportEvent(3, filePath, isDir)
			} else if rawEvent.Mask&syscall.IN_MOVED_FROM != 0 {
				watcher.reportEvent(100, filePath, isDir)
			} else if rawEvent.Mask&syscall.IN_MOVED_TO != 0 {
				watcher.reportEvent(101, filePath, isDir)
			} else if rawEvent.Mask&syscall.IN_DELETE_SELF != 0 {
				watcher.reportEvent(3, filePath, isDir)
				go watcher.StopWatching()
			} else {
				//decodeMask(rawEvent.Mask)
			}
			offset += syscall.SizeofInotifyEvent + uint32(nameOfFileLenght)
		}
	}
	watcher.stopped = true
	return nil
}

func (watcher *fileSystemWatcherInstance) reportEvent(operation uint8, path string, isDir bool) {
	if isDir && watcher.recursive {
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
	}
	watcher.mainParent.reportEvent(operation, path, isDir)
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

	//Start subwatchers
	entries, err := os.ReadDir(watcher.path)
	if err != nil {
		watcher.mainParent.Logger.Log(3, "Error started watching in path: "+watcher.path+" | Error: "+err.Error())
		return
	}
	for i := 0; i < len(entries); i++ {
		if entries[i].IsDir() {
			//Add
			w := newFileSystemWatcherInstance(watcher.path+entries[i].Name(), watcher.mainParent, watcher.mainParent.recursive)
			watcher.subDirectoriesWatchers.Set(watcher.path+entries[i].Name(), w)
			go w.StartWatching()
		}
	}

	//Start watcher
	watcher.mainParent.Logger.Log(2, "Started watching in path: "+watcher.path)
	err = watcher.watchingLoop()
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
	//syscall.InotifyRmWatch(watcher.inotifyBase, uint32(watcher.inotifyWatcher))
	err := syscall.Close(watcher.inotifyBase)
	time.Sleep(1 * time.Second)
	if err != nil {
		watcher.mainParent.Logger.Log(3, "Error stopping watcher for path: "+watcher.path+" | Error: "+err.Error())
		watcher.stopped = true
	}
	for !watcher.stopped {
		time.Sleep(1 * time.Second)
	}

	//Stop subwatchers
	for i := 0; i < len(watcher.subDirectoriesWatchers.GetValues()); i++ {
		watcher.subDirectoriesWatchers.GetValues()[i].StopWatching()
	}
	watcher.mainParent.Logger.Log(1, "Watching stopped for path: "+watcher.path)
}

//func decodeMask(mask uint32) {
//	//fmt.Printf("Mask: %d (0x%08x)\n", mask, mask)
//
//	flags := map[uint32]string{
//		syscall.IN_ACCESS:        "IN_ACCESS",
//		syscall.IN_MODIFY:        "IN_MODIFY",
//		syscall.IN_ATTRIB:        "IN_ATTRIB",
//		syscall.IN_CLOSE_WRITE:   "IN_CLOSE_WRITE",
//		syscall.IN_CLOSE_NOWRITE: "IN_CLOSE_NOWRITE",
//		syscall.IN_OPEN:          "IN_OPEN",
//		syscall.IN_MOVED_FROM:    "IN_MOVED_FROM",
//		syscall.IN_MOVED_TO:      "IN_MOVED_TO",
//		syscall.IN_CREATE:        "IN_CREATE",
//		syscall.IN_DELETE:        "IN_DELETE",
//		syscall.IN_DELETE_SELF:   "IN_DELETE_SELF",
//		syscall.IN_MOVE_SELF:     "IN_MOVE_SELF",
//		syscall.IN_ISDIR:         "IN_ISDIR",
//	}
//
//	//for bit, name := range flags {
//	//	if mask&bit != 0 {
//	//		//fmt.Println(" -", name)
//	//	}
//	//}
//}
