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
	git clone https://github.com/openconfig/public.git openconfig_public

.PHONY: validate_paths
validate_paths: openconfig_public
	go run -v tools/validate_paths.go \
		--feature_root=$(CURDIR)/feature/ \
		--yang_roots=$(CURDIR)/openconfig_public/release/models/,$(CURDIR)/openconfig_public/third_party/ \
		--yang_skip_roots=$(CURDIR)/openconfig_public/release/models/wifi

.PHONY: kne_arista_setup
kne_arista_setup:
	kne_cli create $(CURDIR)/topologies/kne/arista_ceos.textproto
	@echo "username: admin" >$(CURDIR)/topologies/kne/testbed.kne.yml
	@echo "password: admin" >>$(CURDIR)/topologies/kne/testbed.kne.yml
	@echo "topology: $(CURDIR)/topologies/kne/arista_ceos.textproto" >>$(CURDIR)/topologies/kne/testbed.kne.yml
	@echo "cli: `which kne_cli`" >>$(CURDIR)/topologies/kne/testbed.kne.yml

.PHONY: kne_arista_cleanup
kne_arista_cleanup:
	kne_cli delete $(CURDIR)/topologies/kne/arista_ceos.textproto

.PHONY: kne_nokia_setup
kne_nokia_setup:
	kne_cli create $(CURDIR)/topologies/kne/nokia_srl.textproto
	@echo "username: admin" >$(CURDIR)/topologies/kne/testbed.kne.yml
	@echo "password: admin" >>$(CURDIR)/topologies/kne/testbed.kne.yml
	@echo "topology: $(CURDIR)/topologies/kne/nokia_srl.textproto" >>$(CURDIR)/topologies/kne/testbed.kne.yml
	@echo "cli: `which kne_cli`" >>$(CURDIR)/topologies/kne/testbed.kne.yml

.PHONY: kne_nokia_cleanup
kne_nokia_cleanup:
	kne_cli delete topologies/kne/nokia_srl.textproto

.PHONY: kne_tests
kne_tests:
	go test -v github.com/openconfig/featureprofiles/feature/... -config $(CURDIR)/topologies/kne/testbed.kne.yml -testbed $(CURDIR)/topologies/single_device.textproto
