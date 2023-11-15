# Copyright 2022 Google LLC

# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at

#     https://www.apache.org/licenses/LICENSE-2.0

# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
openconfig_public:
	tools/clone_oc_public.sh openconfig_public

.PHONY: validate_paths
validate_paths: openconfig_public proto/feature_go_proto/feature.pb.go
	go run -v tools/validate_paths.go \
		-alsologtostderr \
		--feature_root=$(CURDIR)/feature/ \
		--yang_roots=$(CURDIR)/openconfig_public/release/models/,$(CURDIR)/openconfig_public/third_party/ \
		--yang_skip_roots=$(CURDIR)/openconfig_public/release/models/wifi \
		--feature_files=${FEATURE_FILES}

proto/feature_go_proto/feature.pb.go: proto/feature.proto
	mkdir -p proto/feature_go_proto
	protoc --proto_path=proto --go_out=./ --go_opt=Mfeature.proto=proto/feature_go_proto feature.proto

proto/metadata_go_proto/metadata.pb.go: proto/metadata.proto
	mkdir -p proto/metadata_go_proto
	# Set directory to hold symlink
	mkdir -p protobuf-import
	# Remove any existing symlinks & empty directories
	find protobuf-import -type l -delete
	find protobuf-import -type d -empty -delete
	# Download the required dependencies
	go mod download
	# Get ondatra modules we use and create required directory structure
	go list -f 'protobuf-import/{{ .Path }}' -m github.com/openconfig/ondatra | xargs -L1 dirname | sort | uniq | xargs mkdir -p
        # Create symlink
	go list -f '{{ .Dir }} protobuf-import/{{ .Path }}' -m github.com/openconfig/ondatra | xargs -L1 -- ln -s
	protoc -I='protobuf-import' --proto_path=proto --go_out=./ --go_opt=Mmetadata.proto=proto/metadata_go_proto metadata.proto
	goimports -w proto/metadata_go_proto/metadata.pb.go

proto/ocpaths_go_proto/ocpaths.pb.go: proto/ocpaths.proto
	mkdir -p proto/ocpaths_go_proto
	protoc --proto_path=proto --go_out=./ --go_opt=Mocpaths.proto=proto/ocpaths_go_proto ocpaths.proto
	goimports -w proto/ocpaths_go_proto/ocpaths.pb.go

proto/ocrpcs_go_proto/ocrpcs.pb.go: proto/ocrpcs.proto
	mkdir -p proto/ocrpcs_go_proto
	protoc --proto_path=proto --go_out=./ --go_opt=Mocrpcs.proto=proto/ocrpcs_go_proto ocrpcs.proto
	goimports -w proto/ocrpcs_go_proto/ocrpcs.pb.go

proto/nosimage_go_proto/nosimage.pb.go: proto/nosimage.proto
	mkdir -p proto/nosimage_go_proto
	protoc -I="${GOPATH}/src" --proto_path=proto --go_out=./proto/nosimage_go_proto --go_opt=paths=source_relative --go_opt=Mnosimage.proto=proto/nosimage_go_proto --go_opt=Mgithub.com/openconfig/featureprofiles/proto/ocpaths.proto=github.com/openconfig/featureprofiles/proto/ocpaths_go_proto --go_opt=Mgithub.com/openconfig/featureprofiles/proto/ocrpcs.proto=github.com/openconfig/featureprofiles/proto/ocrpcs_go_proto nosimage.proto
	goimports -w proto/nosimage_go_proto/nosimage.pb.go
