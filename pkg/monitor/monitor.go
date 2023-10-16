package monitor

/*
this is the file for the background event-listener daemon.

this will listen for events like a file being saved within the client's drive, which will then
automatically start a new sync index operation. whether the user wants to automatically sync or not
should be a setting, but the daemon will automatically make a new sync index with each file or directory
modification.

should also have a mechanism to interrupt a sync operation if a new event occurs.
*/

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

type Monitor struct {
	// path to the directory to monitor
	Path string

	// watcher for file and directory events
	Watcher *fsnotify.Watcher
}

// NOTE: must call defer watcher.Close() after instantiation!
func NewMonitor(path string) *Monitor {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	// add drive path to monitor
	if err := watcher.Add(path); err != nil {
		log.Fatal(err)
	}
	return &Monitor{
		Path:    path,
		Watcher: watcher,
	}
}

func (m *Monitor) watchDir(path string, fi os.FileInfo, err error) error {
	// since fsnotify can watch all the files in a directory, watchers only need
	// to be added to each nested directory
	if fi.Mode().IsDir() {
		return m.Watcher.Add(path)
	}
	return nil
}

// This monitors the entire drive file system by passing m.watchDir
// to filepath.Walk().
func (m *Monitor) WatchDrive(notify chan fsnotify.Event, drvPath string) {
	defer m.Watcher.Close()

	// add all subdirectories to the watcher
	if err := filepath.Walk(drvPath, m.watchDir); err != nil {
		fmt.Printf("failed to add directories to watcher: %v", err)
	}

	// start listening for events
	go func() {
		for {
			select {
			case event, ok := <-m.Watcher.Events:
				if !ok {
					log.Printf("[WARNING] monitoring failed: %v", event)
					return
				}
				// write event
				if event.Has(fsnotify.Write) {
					log.Println("[INFO] modified:", event.Name)
					notify <- event
				}
				// create event
				if event.Has(fsnotify.Create) {
					log.Println("[INFO] created:", event.Name)
					notify <- event
				}
				// delete event
				if event.Has(fsnotify.Remove) {
					log.Println("[INFO] renoved:", event.Name)
					notify <- event
				}
			case err, ok := <-m.Watcher.Errors:
				if !ok {
					log.Printf("[ERROR] monitoring failed: %v", err)
					return
				}
				log.Println("error:", err)
			}
		}
	}()
}
