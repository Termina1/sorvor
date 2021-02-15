clean:
	@git clean -fdX

upgrade:
	@go get -t -u  ./...

build: main.go
	@go build -o /usr/local/bin/sorvor

start:
	@make build && cd ${CURDIR}/examples/preact-counter && npm install && npm run start

test:
	@make clean && make build && \
	cd ${CURDIR}/examples/minimal && sorvor && \
	cd ${CURDIR}/examples/minimal-css && sorvor && \
	cd ${CURDIR}/examples/minimal-typescript && sorvor && \
	cd ${CURDIR}/examples/preact-counter && npm install && npm run build && \
	cd ${CURDIR}/examples/react-counter && npm install && npm run build

release:
	@make test && \
	printf "current version: " && git tag | tail -1
	@read -p "enter new version: " version; git tag v$$version
	@git push
	@git push --tags
	@make verify

verify:
	@curl -sf https://gobinaries.com/osdevisnot/sorvor | sh
	@cd ${CURDIR}/examples/minimal && sorvor && \
	cd ${CURDIR}/examples/minimal-css && sorvor && \
	cd ${CURDIR}/examples/minimal-typescript && sorvor && \
	cd ${CURDIR}/examples/preact-counter && npm install && npm run build && \
	cd ${CURDIR}/examples/react-counter && npm install && npm run build