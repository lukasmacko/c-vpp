include Makeroutines.mk

VERSION=$(shell git rev-parse HEAD)
DATE=$(shell date +'%Y-%m-%dT%H:%M%:z')
LDFLAGS=-ldflags '-X github.com/contiv/contiv-vpp/vendor/github.com/ligato/cn-infra/core.BuildVersion=$(VERSION) -X github.com/contiv/contiv-vpp/vendor/github.com/ligato/cn-infra/core.BuildDate=$(DATE)'
COVER_DIR=/tmp/


# generate go structures from proto files
define generate_sources
	$(call install_generators)
	@echo "# generating sources"
	@echo "# done"
endef

# install-only binaries
define install_only
	@echo "# installing contiv-vpp"
	@cd cmd/contiv-vpp && go install -v ${LDFLAGS}
	@cd cmd/contiv-cni && go install -v ${LDFLAGS}
	@echo "# done"
endef

# run all tests
define test_only
	@echo "# running unit tests"
	@echo "# done"
endef

# run all tests with coverage
define test_cover_only
	@echo "# running unit tests with coverage analysis"
    @echo "# done"
endef

# run all tests with coverage and display HTML report
define test_cover_html
    $(call test_cover_only)
endef

# run all tests with coverage and display XML report
define test_cover_xml
	$(call test_cover_only)
endef

# run code analysis
define lint_only
   @echo "# running code analysis"
    @./scripts/golint.sh
    @./scripts/govet.sh
    @echo "# done"
endef

# run code formatter
define format_only
    @echo "# formatting the code"
    @./scripts/gofmt.sh
    @echo "# done"
endef


# build contiv-vpp only
define build_contiv_vpp_only
    @echo "# building contiv-vpp"
    @cd cmd/contiv-vpp && go build -v -i ${LDFLAGS}
    @echo "# done"
endef

# build contiv-cni only
define build_contiv_cni_only
    @echo "# building contiv-cni"
    @cd cmd/contiv-cni && go build -v -i ${LDFLAGS}
    @echo "# done"
endef

# build cni-grpc-client only
define build_cni_grpc_client_only
    @echo "# building cni-grpc-client"
    @cd cmd/cni-grpc-client && go build -v -i ${LDFLAGS}
    @echo "# done"
endef

# verify that links in markdown files are valid
# requires npm install -g markdown-link-check
define check_links_only
    @echo "# checking links"
    @./scripts/check_links.sh
    @echo "# done"
endef


# build all binaries
build:
	$(call build_contiv_vpp_only)
	$(call build_contiv_cni_only)
	$(call build_cni_grpc_client_only)

# build contiv-vpp
contiv-vpp:
	$(call build_contiv_vpp_only)

# build contiv-cni
contiv-cni:
	$(call build_contiv_cni_only)

# install binaries
install:
	$(call install_only)

# install dependencies
install-dep:
	$(call install_dependencies)

# update dependencies
update-dep:
	$(call update_dependencies)

# generate structures
generate:
	$(call generate_sources)

# run tests
test:
	$(call test_only)

# run tests with coverage report
test-cover:
	$(call test_cover_only)

# run tests with HTML coverage report
test-cover-html:
	$(call test_cover_html)

# run tests with XML coverage report
test-cover-xml:
	$(call test_cover_xml)

# run & print code analysis
lint:
	$(call lint_only)

# format the code
format:
	$(call format_only)

# validate links in markdown files
check_links:
	$(call check_links_only)

# clean
clean:
	rm -f cmd/contiv-vpp/contiv-vpp
	rm -f cmd/contiv-cni/contiv-cni
	rm -f cmd/cni-grpc-client/cni-grpc-client
	@echo "# cleanup completed"

# run all targets
all:
	$(call lint_only)
	$(call build)
	$(call test_only)
	$(call install_only)

.PHONY: build update-dep install-dep test lint clean
