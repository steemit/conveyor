
SHELL := /bin/bash
PATH  := ./node_modules/.bin:$(PATH)

SRC_FILES := $(shell find src -name '*.ts')

DOCS_ROOT := docs
API_TESTS_ROOT := api-tests
CONVEYOR_SCHEMA := conveyor_schema.json

all: lib

lib: $(SRC_FILES) node_modules tsconfig.json
	tsc -p tsconfig.json --outDir lib
	VERSION="$$(node -p 'require("./package.json").version')"; \
	BUILD="$$(git rev-parse --short HEAD)-$$(date +%s)"; \
	echo "module.exports = '$${VERSION}-$${BUILD}';" > lib/version.js
	touch lib

.PHONY: devserver
devserver: node_modules
	@onchange -i 'src/**/*.ts' 'config/*' -- ts-node src/server.ts | bunyan -o short

reports:
	mkdir reports

.PHONY: coverage
coverage: node_modules reports
	NODE_ENV=test nyc -r html -r text -e .ts -i ts-node/register \
		--report-dir reports/coverage \
		mocha --reporter nyan --require ts-node/register test/*.ts

.PHONY: test
test: node_modules
	NODE_ENV=test mocha --require ts-node/register test/*.ts --grep '$(grep)'

.PHONY: ci-test
ci-test: node_modules reports
	nsp check
	tslint -p tsconfig.json -c tslint.json
	NODE_ENV=test nyc -r lcov -e .ts -i ts-node/register \
		--report-dir reports/coverage \
		mocha --require ts-node/register \
		--reporter mocha-junit-reporter \
		--reporter-options mochaFile=./reports/unit-tests/junit.xml \
		test/*.ts

.PHONY: lint
lint: node_modules
	tslint -p tsconfig.json -c tslint.json -t stylish --fix

node_modules: package.json
	yarn install --non-interactive --frozen-lockfile

.PHONY: clean
clean:
	rm -rf lib/

.PHONY: distclean
distclean: clean
	rm -rf node_modules/

.PHONY:docs
docs: $(DOCS_ROOT)/Conveyor.html $(DOCS_ROOT)/Conveyor.md

$(DOCS_ROOT)/Conveyor.html:$(CONVEYOR_SCHEMA)
	-mkdir -p $(HTML_DOCS_ROOT)
	./node_modules/.bin/jrgen --outdir $(DOCS_ROOT) docs/html $<


$(DOCS_ROOT)/Conveyor.md: $(CONVEYOR_SCHEMA)
	-mkdir -p $(MD_DOCS_ROOT)
	./node_modules/.bin/jrgen --outdir $(DOCS_ROOT) docs/md $<


.PHONY: api-tests
api-tests: $(CONVEYOR_SCHEMA)
	-mkdir -p $(API_TESTS_ROOT)
	./node_modules/.bin/jrgen --outdir $(API_TESTS_ROOT) test/jasmine $<