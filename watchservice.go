package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"code.google.com/p/go.exp/fsnotify"
)

const (
	eventBufSize = 1024 * 1024
	fsnCreate    = 1
	fsnModify    = 2
	fsnDelete    = 4
	fsnRename    = 8

	fsnAll = fsnModify | fsnDelete | fsnRename | fsnRename
)

// EventBit is a simple way to track what filesytem events are valid.
type EventBit struct {
	Name  string
	Value uint32
	Desc  string
}

// CreateEvent is used to represent a fsnotify "create" event
var CreateEvent = EventBit{Name: "create", Value: fsnCreate, Desc: "File/directory created in watched directory"}

// DeleteEvent is used to represent a fsnotify "delete" event
var DeleteEvent = EventBit{Name: "delete", Value: fsnDelete, Desc: "File/directory deleted from watched directory"}

// ModifyEvent is used to represent a fsnotify "modify or attrib" event
var ModifyEvent = EventBit{Name: "modify", Value: fsnModify, Desc: "File was modified or Metadata changed"}

// RenameEvent is used to represent a fsnotify "rename" event
var RenameEvent = EventBit{Name: "rename", Value: fsnRename, Desc: "File moved out of watched directory"}
var allEvent = EventBit{Value: fsnAll, Desc: "Create/Delete/Modify/Rename"}

// ValidEvents map those fsnotify events that can be watched
var ValidEvents = map[string]EventBit{
	"create": CreateEvent,
	"delete": DeleteEvent,
	"modify": ModifyEvent,
	"rename": RenameEvent,
}

// WatchService encapsulates all thats required to perform the 'watchf' operation
type WatchService struct {
	path   string
	config *Config

	watcher              *fsnotify.Watcher
	watchFlags           map[string]EventBit
	includePatternRegexp *regexp.Regexp

	executor *Executor

	dirs    map[string]bool
	entries map[string]*FileEntry
}

// NewWatchService creates a new WatchService.
func NewWatchService(path string, config *Config) (service *WatchService, err error) {
	watchFlags, err := validateWatchFlags(config.Events)
	if err != nil {
		return
	}

	includePatternRegexp, err := regexp.Compile(config.IncludePattern)
	if err != nil {
		return
	}

	service = &WatchService{
		path,
		config,
		nil,
		watchFlags,
		includePatternRegexp,
		&Executor{os.Stdout, os.Stderr},
		make(map[string]bool),
		make(map[string]*FileEntry),
	}
	return
}

func validateWatchFlags(events []string) (watchedEvents map[string]EventBit, err error) {
	Logln("validating watch flags:")

	var watchedFlags map[string]EventBit

	// confirm that some events were asked to be watched
	if len(events) == 0 {
		err = fmt.Errorf("%s events is simply not enough", "zero")
		return
	}

	// and they are valid events
	containsAll := false
	for _, event := range events {
		var eevent = strings.ToLower(event)
		_, ok := ValidEvents[eevent]

		if eevent == "all" {
			containsAll = true
		} else if !ok {
			err = fmt.Errorf("the event %s was not found", eevent)
			return
		}
	}

	// populate our map of watched events
	if containsAll {
		watchedFlags = ValidEvents
	} else {
		watchedFlags = make(map[string]EventBit)
		for _, event := range events {
			var lcEvent = strings.ToLower(event)
			switch {
			case lcEvent == CreateEvent.Name:
				watchedFlags[CreateEvent.Name] = CreateEvent
			case lcEvent == DeleteEvent.Name:
				watchedFlags[DeleteEvent.Name] = DeleteEvent
			case lcEvent == ModifyEvent.Name:
				watchedFlags[ModifyEvent.Name] = ModifyEvent
			case lcEvent == RenameEvent.Name:
				watchedFlags[RenameEvent.Name] = RenameEvent
			}
		}
	}

	return watchedFlags, nil
}

// Start the WatchService
func (w *WatchService) Start() (err error) {
	events := make(chan *fsnotify.FileEvent, eventBufSize)
	w.startWatcher(events) // events producer
	w.startWorker(events)  // events consumer
	return
}

func (w *WatchService) startWatcher(events chan<- *fsnotify.FileEvent) (err error) {
	w.watcher, err = fsnotify.NewWatcher()
	if err != nil {
		return
	}

	go func() {
		for {
			select {
			case evt, ok := <-w.watcher.Event:
				if ok {
					// emit events from watcher.Event to buffered channel in order to non-ignored events
					events <- evt
				} else {
					close(events)
					return
				}
			case err, ok := <-w.watcher.Error:
				if ok {
					log.Println("watcher err:", err)
				} else {
					return
				}
			}
		}
	}()

	err = w.watchFolders()
	return
}

func (w *WatchService) watchFolders() (err error) {
	if w.config.Recursive {
		err = filepath.Walk(w.path, func(path string, info os.FileInfo, errPath error) error {
			if info.IsDir() {
				relativePath := "./" + path
				if errPath == nil {
					w.dirs[relativePath] = true
					Logln("watching: ", relativePath)
					errWatcher := w.watcher.Watch(path)
					if errWatcher != nil {
						return errWatcher
					}
				} else {
					log.Printf("skip dir %s, caused by: %s\n", relativePath, errPath)
					return filepath.SkipDir
				}
			}
			return nil
		})
	} else {
		err = w.watcher.Watch(w.path)
	}
	return
}

func (w *WatchService) startWorker(events <-chan *fsnotify.FileEvent) {
	go func() {
		var lastExec time.Time
		for evt := range events {
			Logf("%s: %s", getEventType(evt), evt.Name)

			w.syncWatchersAndCaches(evt)

			if checkPatternMatching(w.includePatternRegexp, evt) {
				if checkEventType(w.watchFlags, evt) {
					if checkExecInterval(lastExec, w.config.Interval, time.Now()) {
						if w.isDir(evt.Name) {
							lastExec = time.Now()
							w.run(evt)
						} else {
							// ignore file attributes changed
							if evt.IsModify() && !checkFileContentChanged(w.entries, evt.Name) {
								continue
							}
							lastExec = time.Now()
							w.run(evt)
						}
					} else {
						Logf("%s: %s dropped", getEventType(evt), evt.Name)
					}
				} // if event match
			} // if pattern match
		} // for each event
	}()
}

func getEventType(evt *fsnotify.FileEvent) string {
	eventType := ""

	switch {
	case evt.IsCreate():
		eventType = "ENTRY_CREATE"
	case evt.IsModify():
		eventType = "ENTRY_MODIFY"
	case evt.IsDelete():
		eventType = "ENTRY_DELETE"
	case evt.IsRename():
		eventType = "ENTRY_RENAME"
	}
	return eventType
}

func (w *WatchService) syncWatchersAndCaches(evt *fsnotify.FileEvent) {
	path := evt.Name
	switch {
	case evt.IsCreate():
		stat, err := os.Stat(path)
		if err != nil {
			Logln(err)
		} else {
			if stat.IsDir() {
				Logln("watching: ", path)
				w.dirs[path] = true
				w.watcher.Watch(path)
			}
		}

	case evt.IsRename(), evt.IsDelete():
		if w.isDir(path) {
			Logln("remove watching: ", path)
			delete(w.dirs, path)
			w.watcher.RemoveWatch(path)

			dirPath := path + string(os.PathSeparator)
			for entryPath := range w.entries {
				if strings.HasPrefix(entryPath, dirPath) {
					delete(w.entries, entryPath)
				}
			}
		} else {
			delete(w.entries, path)
		}
	}
}

func (w *WatchService) isDir(path string) bool {
	_, ok := w.dirs[path]
	return ok
}

func (w *WatchService) run(evt *fsnotify.FileEvent) {
	for _, command := range w.config.Commands {
		err := w.executor.execute(command, evt)
		if err != nil && !ContinueOnError {
			break
		}
	}
}

// Stop the WatchService
func (w *WatchService) Stop() error {
	return w.watcher.Close()
}
