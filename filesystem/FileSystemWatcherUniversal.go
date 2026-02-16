package filesystem

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
	"webtools"
)

const MOVE_REQUEST_TIMEOUT = 1 //In seconds

type FileSystemEvent uint8

const FSEventNone FileSystemEvent = 0
const FSEventModified FileSystemEvent = 1
const FSEventCreated FileSystemEvent = 2
const FSEventDeleted FileSystemEvent = 3
const FSEventMoved FileSystemEvent = 4

// Operations: 0 = None/Error, 1 = Modified, 2 = Created, 3 = Deleted, 4 = Moved
type FileSystemWatcherEventFunc func(path string, operation FileSystemEvent, isDir bool, newPath string)

type FileSystemWatcher struct {
	path      string
	eventFunc FileSystemWatcherEventFunc
	isRunning bool
	recursive bool
	Logger    *webtools.ConsoleLogger
	watcher   *fileSystemWatcherInstance

	fileMoveEvent     bool
	filePath          string
	filePathEventTime time.Time
	dirMoveEvent      bool
	dirPath           string
	dirPathEventTime  time.Time
}

func NewFileSystemWatcher(path string, eventFunc FileSystemWatcherEventFunc, recursive bool, logEvents bool) *FileSystemWatcher {
	fsWatch := &FileSystemWatcher{path: path, eventFunc: eventFunc, isRunning: false, Logger: webtools.NewConsoleLoggerForTraffic("FSWatcher", logEvents), recursive: recursive}
	fsWatch.watcher = newFileSystemWatcherInstance(path, fsWatch, recursive)
	return fsWatch
}

/*
Starts watching for file changes, locks execution thread. Is is recommended to set "defer watcher.StopWatching()" before starting the watcher
*/
func (watcher *FileSystemWatcher) StartWatching() {
	watcher.isRunning = true
	go watcher.moveTimer()
	watcher.watcher.StartWatching()
}

/*
Requests stop for watching
*/
func (watcher *FileSystemWatcher) StopWatching() {
	watcher.watcher.StopWatching()
	watcher.isRunning = false
}

func (watcher *FileSystemWatcher) reportEvent(event uint8, filePath string, isDir bool) {
	watcher.Logger.Log(0, "Got event: "+strconv.FormatUint(uint64(event), 10)+" for "+webtools.FormatByBool(isDir, "folder", "file")+": "+filePath)
	if watcher.eventFunc == nil {
		return
	}
	if event == 100 {
		//Begin move
		if isDir {
			for watcher.dirMoveEvent {
				//One exists, sort out
				if time.Since(watcher.dirPathEventTime).Seconds() >= MOVE_REQUEST_TIMEOUT {
					watcher.eventFunc(watcher.dirPath, FSEventDeleted, true, watcher.dirPath)
					break
				} else {
					time.Sleep(1 * time.Second)
				}
			}
			watcher.dirPathEventTime = time.Now()
			watcher.dirPath = filePath
			watcher.dirMoveEvent = true
		} else {
			for watcher.fileMoveEvent {
				//One exists, sort out
				if time.Since(watcher.filePathEventTime).Seconds() >= MOVE_REQUEST_TIMEOUT {
					watcher.eventFunc(watcher.filePath, FSEventDeleted, false, watcher.filePath)
					break
				} else {
					time.Sleep(1 * time.Second)
				}
			}
			watcher.filePathEventTime = time.Now()
			watcher.filePath = filePath
			watcher.fileMoveEvent = true
		}
		return
	}
	if event == 101 {
		//End move
		if isDir {
			if watcher.dirMoveEvent {
				watcher.eventFunc(watcher.dirPath, FSEventMoved, true, filePath)
			} else {
				watcher.eventFunc(watcher.dirPath, FSEventModified, true, filePath)
			}
			watcher.dirMoveEvent = false
		} else {
			if watcher.fileMoveEvent {
				watcher.eventFunc(watcher.filePath, FSEventMoved, false, filePath)
			} else {
				watcher.eventFunc(watcher.filePath, FSEventModified, false, filePath)
			}
			watcher.fileMoveEvent = false

		}
		return
	}

	//Sort times
	eventSend := FSEventNone
	if event == 1 {
		eventSend = FSEventModified
	} else if event == 2 {
		eventSend = FSEventCreated
	} else if event == 3 {
		eventSend = FSEventDeleted
	} else if event == 4 {
		eventSend = FSEventMoved
	}
	watcher.eventFunc(filePath, eventSend, isDir, filePath)
}

func (watcher *FileSystemWatcher) moveTimer() {
	if watcher.dirMoveEvent && time.Since(watcher.dirPathEventTime).Seconds() >= MOVE_REQUEST_TIMEOUT {
		watcher.eventFunc(watcher.dirPath, 3, true, watcher.dirPath)
		watcher.dirMoveEvent = false
	}
	if watcher.fileMoveEvent && time.Since(watcher.filePathEventTime).Seconds() >= MOVE_REQUEST_TIMEOUT {
		watcher.eventFunc(watcher.filePath, 3, false, watcher.filePath)
		watcher.fileMoveEvent = false
	}
	if watcher.isRunning {
		time.Sleep(1 * time.Second)
		watcher.moveTimer()
	}
}

var iNVALID_NAMES = [...]string{"..", "."}

/*
Checks if path constains some invalid names (server protection) -> Returns TRUE if value is INVALID
! Must be present in every operation with files on server !
*/
func CheckInvalidNames(path string) error {
	if !strings.HasPrefix(path, "/") {
		fmt.Println("Path does not start with /")
		return os.ErrInvalid
	}
	split := strings.Split(path, "/")
	for i := 0; i < len(iNVALID_NAMES); i++ {
		for k := 0; k < len(split); k++ {
			if strings.EqualFold(split[k], iNVALID_NAMES[i]) {
				fmt.Println("Found invalid name: " + iNVALID_NAMES[i] + " in: " + path)
				return os.ErrInvalid
			}
		}
	}
	return nil
}
