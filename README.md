# tail

Tail a file programmatically just like you run shell command `tail -f /somefile`.


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
