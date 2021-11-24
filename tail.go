package tail

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"io"
	"os"
	"strings"
	"syscall"
	"time"
)

var openFileError = errors.New("open file error")
var closeError = errors.New("close")

func TailF(filePath string, waitFileExist bool) (chan string, chan int, chan error) {
	lineCh := make(chan string)
	closeCh := make(chan int, 1)
	errorCh := make(chan error, 1)
	tk := time.NewTicker(10 * time.Second)

	go func() {
		defer tk.Stop()
		for {
			select {
			case <-closeCh:
				return
			default:
				err := tail(filePath, waitFileExist, lineCh, closeCh)
				if err != nil {
					if err == openFileError {
						errorCh <- err
						return
					} else if err == closeError {
						return
					}
				}
			}
			<-tk.C
		}
	}()

	return lineCh, closeCh, errorCh
}

func tail(filePath string, waitFileExist bool, lineCh chan string, closeCh chan int) error {
	file, err := os.Open(filePath)
	if os.IsNotExist(err) && waitFileExist {
		file, err = ensureOpenFile(filePath)
	}
	if err != nil {
		return openFileError
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat file error: %s", err.Error())
	}
	size := stat.Size()
	stat2 := stat.Sys().(*syscall.Stat_t)
	cTime := stat2.Ctim

	start := size
	if size >= 1000 {
		start = size - 1000
	}
	_, err = file.Seek(-start, io.SeekEnd)
	if err != nil {
		return fmt.Errorf("seek file error: %s", err.Error())
	}

	done := make(chan struct{}, 1)
	watcher, removeCh, writeCh, errorCh, regWatchErr := watchFile(filePath, cTime, done)
	if regWatchErr != nil {
		return regWatchErr
	}
	defer watcher.Close()

	reader := bufio.NewReader(file)
	var drainErr error
	halfLine, err := drainFile(reader, lineCh, "")
	if err != nil {
		return fmt.Errorf("drain file error: %s", err.Error())
	}

	pollTicker := time.NewTicker(10 * time.Second)
	defer pollTicker.Stop()
	oneMoreTry := make(chan struct{}, 1)
	defer func() {
		emitLastHalfLine(halfLine, lineCh)
		done <- struct{}{}
	}()

	for {
		select {
		case <-closeCh:
			return closeError
		case watchErr := <-errorCh:
			return watchErr
		case <-removeCh:
			return errors.New("file removed")
		case <-oneMoreTry:
			halfLine, drainErr = drainFile(reader, lineCh, halfLine)
			if drainErr != nil {
				return drainErr
			}
		case <-writeCh:
			if drainWriteCh(writeCh) {
				oneMoreTry <- struct{}{}
			}
			// check whether file is truncated or size shrink
			stat, err := os.Stat(filePath)
			if err != nil {
				return errors.New("stat file error")
			}
			if stat.Size() < size {
				// file may be truncated
				return errors.New("file shrink error")
			}
			size = stat.Size()

			halfLine, drainErr = drainFile(reader, lineCh, halfLine)
			if drainErr != nil {
				return drainErr
			}
		case <-pollTicker.C:
			halfLine, drainErr = drainFile(reader, lineCh, halfLine)
			if drainErr != nil {
				return drainErr
			}
		}
	}
}

func emitLastHalfLine(halfLine string, lineCh chan string) {
	if halfLine != "" {
		lineCh <- halfLine
	}
}

func drainWriteCh(ch chan struct{}) bool {
	ret := false
	for {
		select {
		case <-ch:
			ret = true
		default:
			return ret
		}
	}
}

func drainFile(reader *bufio.Reader, lineCh chan string, halfLine string) (string, error) {
	halfLineUsed := false
	if halfLine == "" {
		halfLineUsed = true
	}
	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			return line, nil
		}
		if err == nil {
			if !halfLineUsed {
				lineCh <- strings.TrimSuffix(halfLine+line, "\n")
				halfLineUsed = true
			} else {
				lineCh <- strings.TrimSuffix(line, "\n")
			}
		} else {
			return "", nil
		}
	}
}

func ensureOpenFile(filePath string) (*os.File, error) {
	tk := time.NewTicker(10 * time.Second)
	defer tk.Stop()
	for {
		<-tk.C
		file, err := os.Open(filePath)
		if err == nil {
			return file, nil
		}
		if os.IsNotExist(err) {
			continue
		}
		return nil, err
	}
}

func watchFile(filePath string, timespec syscall.Timespec, done chan struct{}) (*fsnotify.Watcher, chan struct{}, chan struct{}, chan error, error) {
	removeCh := make(chan struct{})
	writeCh := make(chan struct{})
	errorCh := make(chan error)
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return watcher, nil, nil, nil, err
	}

	go func() {
		tk := time.NewTicker(10 * time.Second)
		defer tk.Stop()
		for {
			select {
			case <-done:
				return
			default:
				<-tk.C
				stat, err := os.Stat(filePath)
				if err != nil {
					errorCh <- err
					return
				}
				stat2 := stat.Sys().(*syscall.Stat_t)
				cTime := stat2.Ctim
				if cTime != timespec {
					removeCh <- struct{}{}
					return
				}
			}
		}
	}()

	go func() {
		for {
			select {
			case <-done:
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					writeCh <- struct{}{}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				if err != nil {
					errorCh <- err
					return
				}
			}
		}
	}()

	return watcher, removeCh, writeCh, errorCh, watcher.Add(filePath)
}
