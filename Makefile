.PHONY : build run

args = `arg="$(filter-out $@,$(MAKECMDGOALS))" && echo $${arg:-${1}}`

$(eval $(service):;@:)

check:
	@[ "${service}" ] || ( echo "\x1b[31;1mERROR: 'service' is not set\x1b[0m"; exit 1 )
	@if [ ! -d "services/$(service)" ]; then  echo "\x1b[31;1mERROR: service '$(service)' undefined\x1b[0m"; exit 1; fi

prepare: check
	@if [ ! -f services/$(service)/.env ]; then cp services/$(service)/.env.sample services/$(service)/.env; fi;

init:
	@candi --init

add-module: check
	@candi --add-module --service=$(service)

proto: check
	@if [ ! -d "sdk/$(service)/proto" ]; then echo "creating new proto files..." && mkdir sdk/$(service)/proto; fi
	$(foreach proto_file, $(shell find services/$(service)/api/proto -name '*.proto'),\
	protoc --proto_path=services/$(service)/api/proto --go_out=plugins=grpc:sdk/$(service)/proto \
	--go_opt=paths=source_relative $(proto_file);)

migration: check
	@WORKDIR="services/$(service)/" go run services/$(service)/cmd/migration/migration.go $(call args,up)

build: check
	@go build -o services/$(service)/bin services/$(service)/*.go

run: build
	@WORKDIR="services/$(service)/" ./services/$(service)/bin

docker: check
	docker build --build-arg SERVICE_NAME=$(service) -t $(service):latest .

run-container: check
	docker run --rm --name=$(service) --network="host" -d $(service)

clear-docker: check
	docker rm -f $(service)
	docker rmi -f $(service)

# mocks all interfaces from selected service for unit test
mocks: check
	@mockery --all --keeptree --dir=sdk --output=./sdk/mocks
	@if [ -f sdk/mocks/Option.go ]; then rm sdk/mocks/Option.go; fi;
	@mockery --all --keeptree --dir=globalshared --output=./globalshared/mocks
	@mockery --all --keeptree --dir=services/$(service)/internal --output=services/$(service)/pkg/mocks --case underscore
	@mockery --all --keeptree --dir=services/$(service)/pkg --output=services/$(service)/pkg/mocks --case underscore

# unit test & calculate code coverage from selected service (please run mocks before run this rule)
test: check
	@echo "\x1b[32;1m>>> running unit test and calculate coverage for service $(service)\x1b[0m"
	@if [ -f services/$(service)/coverage.txt ]; then rm services/$(service)/coverage.txt; fi;
	@go test ./services/$(service)/... -cover -coverprofile=services/$(service)/coverage.txt -covermode=count \
		-coverpkg=$$(go list ./services/$(service)/... | grep -v mocks | tr '\n' ',')
	@go tool cover -func=services/$(service)/coverage.txt

sonar: check
	@echo "\x1b[32;1m>>> running sonar scanner for service $(service)\x1b[0m"
	@[ "${level}" ] || ( echo "\x1b[31;1mERROR: 'level' is not set\x1b[0m" ; exit 1 )
	@sonar-scanner -Dsonar.projectKey=$(service)-$(level) \
	-Dsonar.projectName=$(service)-$(level) \
	-Dsonar.sources=./services/$(service) \
	-Dsonar.exclusions=**/mocks/**,**/module.go \
	-Dsonar.test.inclusions=**/*_test.go \
	-Dsonar.test.exclusions=**/mocks/** \
	-Dsonar.coverage.exclusions=**/mocks/**,**/*_test.go,**/main.go \
	-Dsonar.go.coverage.reportPaths=./services/$(service)/coverage.txt

clear:
	rm -rf sdk/mocks services/$(service)/mocks services/$(service)/bin bin
