
SHELL := /bin/bash
PATH  := ./node_modules/.bin:$(PATH)

SRC_FILES := $(shell find src -name '*.ts')

DOCS_ROOT := docs
API_TESTS_ROOT := api-tests
CONVEYOR_SCHEMA := conveyor_schema.json

# Bad Actor / GDPR User Lists
GDPR_LISTS := raw.githubusercontent.com/steemit/condenser/master/src/app/utils/GDPRUserList.js
LOCAL_GDPR_LISTS := $(addprefix lists/gdpr/srcs/, $(GDPR_LISTS))

BAD_ACTOR_LISTS := raw.githubusercontent.com/steemit/condenser/master/src/app/utils/DMCAUserList.js \
    raw.githubusercontent.com/steemit/condenser/master/src/app/utils/ImageUserBlockList.js \
    raw.githubusercontent.com/steemit/condenser/master/src/app/utils/BadActorList.js \
    raw.githubusercontent.com/steemit/redeemer-irredeemables/master/full.txt

LOCAL_BAD_ACTOR_LISTS := $(addprefix lists/bad_actors/srcs/, $(BAD_ACTOR_LISTS))

EXCHANGE_LISTS := raw.githubusercontent.com/steemit/condenser/master/src/app/utils/VerifiedExchangeList.js
LOCAL_EXCHANGE_LISTS := $(addprefix lists/exchanges/srcs/, $(EXCHANGE_LISTS))

VERIFIED_LISTS :=
LOCAL_VERIFIED_LISTS := $(addprefix lists/verified/srcs/, $(VERIFIED_LISTS))


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

$(DOCS_ROOT)/Conveyor.html: $(CONVEYOR_SCHEMA)
	-mkdir -p $(DOCS_ROOT)
	./node_modules/.bin/jrgen --outdir $(DOCS_ROOT) docs/html $<


$(DOCS_ROOT)/Conveyor.md: $(CONVEYOR_SCHEMA)
	-mkdir -p $(DOCS_ROOT)
	./node_modules/.bin/jrgen --outdir $(DOCS_ROOT) docs/md $<

.PHONY: clean-docs
clean-docs:
	-rm -rf $(DOCS_ROOT)/*

.PHONY: update-docs
update-docs: clean-docs docs

.PHONY: api-tests
api-tests: $(CONVEYOR_SCHEMA)
	-mkdir -p $(API_TESTS_ROOT)
	./node_modules/.bin/jrgen --outdir $(API_TESTS_ROOT) test/jasmine $<

.PHONY: bad-actors-list
bad-actors-list: lists/bad_actors/users.txt lists/bad_actors/users.json lists/bad_actors/users.ts

.PHONY: gdpr-list
gdpr-list: lists/gdpr/users.txt lists/gdpr/users.json lists/gdpr/users.ts

.PHONY: exchanges-list
exchanges-list: lists/exchanges/users.txt lists/exchanges/users.json lists/exchanges/users.ts

.PHONY: verified-list
exchanges-list: lists/verified/users.txt lists/verified/users.json lists/verified/users.ts


.PHONY: user-lists
user-lists: bad-actors-list gdpr-list exchanges-list verified-list



lists/bad_actors/users.txt: $(LOCAL_BAD_ACTOR_LISTS)
	cat $(LOCAL_BAD_ACTOR_LISTS) \
	    | awk '{$$1=$$1};1' \
	    | egrep --only-matching --line-regexp '^[\.a-zA-Z0-9_-]+$$' \
	    | LC_COLLATE=C sort \
	    | uniq > $@

lists/bad_actors/srcs/%:
	mkdir -p $(dir $@)
	wget -O $@ https://$*

lists/gdpr/users.txt: $(LOCAL_GDPR_LISTS)
	cat $(LOCAL_GDPR_LISTS) \
	    | awk '{$$1=$$1};1' \
	    | egrep --only-matching --line-regexp '^[\.a-zA-Z0-9_-]+$$' \
	    | LC_COLLATE=C sort \
	    | uniq > $@

lists/gdpr/srcs/%:
	mkdir -p $(dir $@)
	wget -O $@ https://$*

lists/exchanges/users.txt: $(LOCAL_EXCHANGE_LISTS)
	cat $(LOCAL_EXCHANGE_LISTS) \
	    | awk '{$$1=$$1};1' \
	    | egrep --only-matching --line-regexp '^[\.a-zA-Z0-9_-]+$$' \
	    | LC_COLLATE=C sort \
	    | uniq > $@

lists/exchanges/srcs/%:
	mkdir -p $(dir $@)
	wget -O $@ https://$*


lists/%.json: lists/%.txt
	cat $< | jq -R -s -c 'split("\n")|map(select(. != ""))' > $@

lists/%.ts: lists/%.json
	cat <(echo -n "export const users: Set<string> = new Set(") $<  <(echo  ")") | prettier --parser typescript --stdin --no-semi --single-quote > $@

.PHONY: clean-lists
clean-lists:
	-rm -rf lists/gdpr/* lists/bad_actors/* lists/exchanges/*