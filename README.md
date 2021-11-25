# tail

Tail a file programmatically just like you run shell command `tail -f /somefile`.

## features

* support tail a non-exist file until that file exists
* tail works even if file get removed or truncated

## basic usage

```
// closeFunc is used to stop tailing
lineCh, closeFunc, errCh := TailF("/path/to/file", true)
go func() {
    for {
        select {
        case <-errCh:
            return
        case line := <-lineCh:
            fmt.Println(line)
        }
    }
}()
```

## advance usage

by default, tail poll file status change(eg: file delete/truncate) event every 10 seconds, you can 
config poll interval to satisfy your own needs.

```
lineCh, closeFunc, errCh := TailWithConfig("/path/to/file", true, &Config{PollInterval: 500 * time.Millisecond})
go func() {
    for {
        select {
        case <-errCh:
            return
        case line := <-lineCh:
            fmt.Println(line)
        }
    }
}()
```
