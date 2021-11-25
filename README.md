# tail

Tail a file programmatically just like you run shell command `tail -f /somefile`.

## Features

* support tail a non-exist file until that file exists
* tail works even if file get removed or truncated
* tail from specified offset

## Basic usage

```
// closeFunc is used to stop tailing
lineCh, closeFunc, errCh := tail.TailF("/path/to/file", true)
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

## Advance usage

by default, tail poll file delete/truncate-event every 10 seconds, you can 
config poll interval to satisfy your own needs.

```
lineCh, closeFunc, errCh := tail.TailWithConfig("/path/to/file", true, &tail.Config{PollInterval: 500 * time.Millisecond})
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

## More configs

### config tail offset

```
// tail.SeekFromStart/tail.SeekFromEnd/tail.SeekOffsetAuto
lineCh, closeFunc, errCh := tail.TailWithConfig("/path/to/file", true, &tail.Config{
		PollInterval: 500 * time.Millisecond, 
		Offset: tail.SeekFromStart,
})
```

## Limitations

Not tested on windows.
