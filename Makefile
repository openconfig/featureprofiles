openconfig_public:
	git clone https://github.com/openconfig/public.git openconfig_public

.PHONY: validate_paths
validate_paths: openconfig_public
	go run -v tools/validate_paths.go \
		--feature_root=$(CURDIR)/feature/ \
		--yang_roots=$(CURDIR)/openconfig_public/release/models/,$(CURDIR)/openconfig_public/third_party/ \
		--yang_skip_roots=$(CURDIR)/openconfig_public/release/models/wifi
