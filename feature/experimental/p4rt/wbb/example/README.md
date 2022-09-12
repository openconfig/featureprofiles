# go-wbb
Example WBB specific P4Info Go Driver.

## Building the binary

For bash, setup go env:
```
export GOROOT=<path to Go tools>
export GOCACHE=<path to Go cache>
export PATH=$GOROOT/bin:$PATH
```

At the top of the git repo:
```
go build -o go-wbb feature/experimental/p4rt/wbb/example/main.go
```

## Help
```
./go-wbb -h
```

