## Using the NOSImageProfile Validator Script

### Running on Example Files

```
cd $GOPATH/src/github.com/openconfig/featureprofiles/tools/nosimage
go run validate/validate.go -file example/example_nosimageprofile.textproto; rm -rf tmp
```

```
cd $GOPATH/src/github.com/openconfig/featureprofiles/tools/nosimage
go run validate/validate.go -file example/example_nosimageprofile_invalid.textproto; rm -rf tmp
```

### Re-generating Example Files

```
cd $GOPATH/src/github.com/openconfig/featureprofiles/tools/nosimage
go run example/generate_example.go -file-path example/example_nosimageprofile.textproto
go run example/generate_example.go -file-path example/example_nosimageprofile_invalid.textproto -invalid
```
