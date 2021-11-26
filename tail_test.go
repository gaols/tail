package tail

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestTailF(t *testing.T) {
	file, _ := ioutil.TempFile("/tmp", "test")
	defer os.Remove(file.Name())
	if _, err := file.WriteString("this is 1 line\nthis is 2 line\nthis is the 3 line\n"); err != nil {
		panic("write file error")
	}
	_ = file.Close()
	eofCh := make(chan string)
	lineCh, closeFunc, errCh := tailF(file.Name(), true, eofCh, &Config{
		PollInterval: 500 * time.Millisecond,
		Offset:       SeekFromStart,
		Follow:       true,
	})
	buf := bytes.Buffer{}
	done := make(chan int)
	go func() {
		for {
			select {
			case <-errCh:
				t.FailNow()
				return
			case line := <-lineCh:
				buf.WriteString(line)
			case ev := <-eofCh:
				if ev == EventEof {
					closeFunc()
					if "this is 1 linethis is 2 linethis is the 3 line" != buf.String() {
						t.FailNow()
					}
					done <- 1
				}
			}
		}
	}()
	<-done
}

func TestTailF_Remove(t *testing.T) {
	file, _ := ioutil.TempFile("/tmp", "test")
	if _, err := file.WriteString("this is 1 line\nthis is 2 line\nthis is the 3 line\n"); err != nil {
		panic("write file error")
	}
	_ = file.Close()
	inspectCh := make(chan string)
	lineCh, closeFunc, errCh := tailF(file.Name(), true, inspectCh, &Config{
		PollInterval: 500 * time.Millisecond,
		Offset:       SeekFromStart,
		Follow:       true,
	})
	buf := bytes.Buffer{}
	done := make(chan int)

	go func() {
	l:
		for {
			select {
			case <-errCh:
				t.FailNow()
			case <-lineCh:
			case ev := <-inspectCh:
				if ev == EventEof {
					break l
				}
			}
		}
		filePath := file.Name()
		_ = os.Remove(filePath)

		file, err := os.Create(filePath)
		if err != nil {
			t.FailNow()
		}
		if _, err := file.WriteString("this is 11 line\nthis is 12 line\nthis is the 13 line\n"); err != nil {
			panic("write file error")
		}
		_ = file.Close()

		go func() {
			restarted := false
			for {
				select {
				case <-errCh:
					t.FailNow()
				case line := <-lineCh:
					t.Log(line)
					buf.WriteString(line)
				case ev := <-inspectCh:
					if ev == EventTailRestart {
						restarted = true
					} else if ev == EventEof {
						if restarted {
							closeFunc()
							if "this is 11 linethis is 12 linethis is the 13 line" != buf.String() {
								t.FailNow()
							}
							_ = os.Remove(filePath)
							done <- 1
							return
						}
					}
				}
			}
		}()
	}()

	<-done
}

func TestTailF_Truncate(t *testing.T) {
	file, _ := ioutil.TempFile("/tmp", "test")
	lines := `
1
2
3
4
5xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
6xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
7xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
8xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
9xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
10a
10b
10c
11c
`
	if _, err := file.WriteString(lines); err != nil {
		panic("write file error")
	}
	_ = file.Close()
	inspectCh := make(chan string)
	lineCh, closeFunc, errCh := tailF(file.Name(), true, inspectCh, &Config{
		PollInterval: 500 * time.Millisecond,
		Offset:       SeekFromStart,
		Follow:       true,
	})
	done := make(chan int)

	go func() {
	l:
		for {
			select {
			case <-errCh:
				t.FailNow()
			case line := <-lineCh:
				t.Log(line)
			case ev := <-inspectCh:
				if ev == EventEof {
					break l
				}
			}
		}
		filePath := file.Name()
		cmd := fmt.Sprintf(`echo ''> %s`, file.Name())
		err := exec.Command("/bin/bash", "-c", cmd).Run()
		if err != nil {
			panic(err)
		}

		file, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
		if err != nil {
			panic(err)
		}
		if _, err := file.WriteString("this is 11 line\nthis is 12 line\nthis is the 13 line\n"); err != nil {
			panic("write file error: " + err.Error())
		}
		_ = file.Close()

		go func() {
			buf := bytes.Buffer{}
			restarted := false
			for {
				select {
				case <-errCh:
					t.FailNow()
				case line := <-lineCh:
					t.Log(line)
					buf.WriteString(line)
				case ev := <-inspectCh:
					if ev == EventTailRestart {
						restarted = true
					} else if ev == EventEof {
						if restarted {
							closeFunc()
							_ = os.Remove(filePath)
							if "this is 11 linethis is 12 linethis is the 13 line" != buf.String() {
								t.FailNow()
							}
							done <- 1
							return
						}
					}
				}
			}
		}()
	}()

	<-done
}
