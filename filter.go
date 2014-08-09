package main

import (
	"bufio"
	"hash/adler32"
	"io"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"code.google.com/p/go.exp/fsnotify"
)

const (
	// FileCloseCheckInterval is the sleep interval used while checking if a file is officially closed.
	FileCloseCheckInterval = time.Duration(20) * time.Millisecond
	// FileCloseCheckThreshold indicates the number of times we check a file when considering a file officially closed?
	FileCloseCheckThreshold = 2
)

// FileEntry is used to track which files have been watched.
type FileEntry struct {
	size int64
	hash uint32
}

func checkEventType(watchedEvents map[string]EventBit, evt *fsnotify.FileEvent) bool {

	return decorator("check filesystem event is matching watch events flag", func() bool {

		keys := make([]string, 0, len(watchedEvents))
		for k := range watchedEvents {
			keys = append(keys, k)
		}
		joinedWatchedEvents := strings.Join(keys, ", ")

		matched := false
		switch {
		case evt.IsCreate():
			Logf("Does watched events of '%s' contain the '%s' fsnotify event?", joinedWatchedEvents, "create")
			_, matched = watchedEvents[CreateEvent.Name]
		case evt.IsAttrib():
		case evt.IsModify():
			Logf("Does watched events of '%s' contain the '%s' fsnotify event?", joinedWatchedEvents, "modify | attrib")
			_, matched = watchedEvents[ModifyEvent.Name]
		case evt.IsDelete():
			Logf("Does watched events of '%s' contain the '%s' fsnotify event?", joinedWatchedEvents, "delete")
			_, matched = watchedEvents[DeleteEvent.Name]
		case evt.IsRename():
			Logf("Does watched events of '%s' contain the '%s' fsnotify event?", joinedWatchedEvents, "rename")
			_, matched = watchedEvents[RenameEvent.Name]
		}

		return matched
	})
}

func checkPatternMatching(pattern *regexp.Regexp, evt *fsnotify.FileEvent) bool {
	return decorator("check filename is matching the pattern", func() bool {
		Logf("%s ~= %s", pattern, evt.Name)
		matched := pattern.MatchString(evt.Name)
		return matched
	})
}

func decorator(title string, fun func() bool) bool {
	startTime := time.Now()
	Logln("[" + title + "]")
	result := fun()
	Logf("[pass: %v, time: %s]", result, time.Since(startTime))

	return result
}

func checkExecInterval(lastExec time.Time, interval time.Duration, now time.Time) bool {
	return decorator("check execution interval", func() bool {
		if interval == 0 {
			return true
		}
		nextExec := lastExec.Add(interval)
		delta := now.Sub(nextExec)
		Logf("next execution time: %s, now: %s, delta:%s", nextExec, now, delta)
		return delta > 0
	})
}

func checkFileContentChanged(entries map[string]*FileEntry, path string) bool {
	return decorator("check the file content is changed", func() bool {
		contentChanged := false
		// THINK: handle continues event from writing a big file
		err := waitForFileClose(path)
		if err != nil {
			log.Println(err)
			return false
		}

		cachedEntry, found := entries[path]
		if !found {
			// THINK: preload all file entries
			newEntry, err := newFileEntry(path)
			if err != nil {
				log.Println(err)
				return false
			}
			entries[path] = newEntry
			contentChanged = true
		} else {

			contentSize, err := getFileSize(path)
			if err != nil {
				log.Println(err)
				return false
			}
			Logf("file %s, size: %d", path, contentSize)

			if cachedEntry.size != contentSize {
				cachedEntry.size = contentSize
				contentChanged = true
			}

			contentHash, err := getContentHash(path)
			if err != nil {
				log.Println(err)
				return false
			}
			Logf("file %s, hash: %d", path, contentHash)

			if cachedEntry.hash != contentHash {
				cachedEntry.hash = contentHash
				contentChanged = true
			}
		}

		return contentChanged
	})
}

func waitForFileClose(path string) (err error) {
	Logf("wait for the file %s close", path)
	var lastSize int64
	var counter int

	for {
		currentSize, errFilesize := getFileSize(path)
		if errFilesize != nil {
			return errFilesize
		}

		if lastSize == currentSize {
			counter++
			if counter >= FileCloseCheckThreshold {
				return
			}
		} else {
			counter = 0
		}

		lastSize = currentSize
		time.Sleep(FileCloseCheckInterval)
	}
}

func newFileEntry(filename string) (entry *FileEntry, err error) {
	contentSize, err := getFileSize(filename)
	if err != nil {
		return
	}

	sum, err := getContentHash(filename)
	if err != nil {
		return
	}

	entry = &FileEntry{contentSize, sum}
	return
}

func getFileSize(filename string) (size int64, err error) {
	st, err := os.Stat(filename)
	if err != nil {
		return
	}
	size = st.Size()
	return
}

func getContentHash(filename string) (sum uint32, err error) {
	f, err := os.Open(filename)
	defer f.Close()
	if err != nil {
		return
	}

	writer := adler32.New()
	reader := bufio.NewReader(f)

	_, err = io.Copy(writer, reader)
	if err != nil {
		return
	}

	sum = writer.Sum32()
	return
}
