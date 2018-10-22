
SHELL := /bin/bash
PATH  := ./node_modules/.bin:$(PATH)

SRC_FILES := $(shell find src -name '*.ts')

DOCS_ROOT := docs
API_TESTS_ROOT := api-tests
CONVEYOR_SCHEMA := conveyor_schema.json

# user data and indexes
USER_DATA_ROOT := ./user-data

# user accounts
USER_ACCTS_ROOT := $(USER_DATA_ROOT)/accounts
USER_ACCTS_FILE := $(USER_ACCTS_ROOT)/accounts.ts

# bad actor / GDPR user lists
LISTS_ROOT := $(USER_DATA_ROOT)/lists
GDPR_LISTS := raw.githubusercontent.com/steemit/condenser/master/src/app/utils/GDPRUserList.js
LOCAL_GDPR_LISTS := $(addprefix $(LISTS_ROOT)/gdpr/srcs/, $(GDPR_LISTS))

BAD_ACTOR_LISTS := raw.githubusercontent.com/steemit/condenser/master/src/app/utils/DMCAUserList.js \
    raw.githubusercontent.com/steemit/condenser/master/src/app/utils/ImageUserBlockList.js \
    raw.githubusercontent.com/steemit/condenser/master/src/app/utils/BadActorList.js \
    raw.githubusercontent.com/steemit/redeemer-irredeemables/master/full.txt

LOCAL_BAD_ACTOR_LISTS := $(addprefix $(LISTS_ROOT)/bad_actors/srcs/, $(BAD_ACTOR_LISTS))

EXCHANGE_LISTS := raw.githubusercontent.com/steemit/condenser/master/src/app/utils/VerifiedExchangeList.js
LOCAL_EXCHANGE_LISTS := $(addprefix $(LISTS_ROOT)/exchanges/srcs/, $(EXCHANGE_LISTS))

VERIFIED_LISTS :=
LOCAL_VERIFIED_LISTS := $(addprefix $(LISTS_ROOT)/verified/srcs/, $(VERIFIED_LISTS))


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
bad-actors-list: $(LISTS_ROOT)/bad_actors/users.txt $(LISTS_ROOT)/bad_actors/users.json $(LISTS_ROOT)/bad_actors/users.ts

.PHONY: gdpr-list
gdpr-list: $(LISTS_ROOT)/gdpr/users.txt $(LISTS_ROOT)/gdpr/users.json $(LISTS_ROOT)/gdpr/users.ts

.PHONY: exchanges-list
exchanges-list: $(LISTS_ROOT)/exchanges/users.txt $(LISTS_ROOT)/exchanges/users.json $(LISTS_ROOT)/exchanges/users.ts

.PHONY: verified-list
exchanges-list: $(LISTS_ROOT)/verified/users.txt $(LISTS_ROOT)/verified/users.json $(LISTS_ROOT)/verified/users.ts

.PHONY: user-lists
user-lists: bad-actors-list gdpr-list exchanges-list verified-list


$(LISTS_ROOT)/bad_actors/users.txt: $(LOCAL_BAD_ACTOR_LISTS)
	cat $(LOCAL_BAD_ACTOR_LISTS) \
	    | awk '{$$1=$$1};1' \
	    | egrep --only-matching --line-regexp '^[\.a-zA-Z0-9_-]+$$' \
	    | LC_COLLATE=C sort \
	    | uniq > $@

$(LISTS_ROOT)/bad_actors/srcs/%:
	mkdir -p $(dir $@)
	wget -O $@ https://$*

$(LISTS_ROOT)/gdpr/users.txt: $(LOCAL_GDPR_LISTS)
	cat $(LOCAL_GDPR_LISTS) \
	    | awk '{$$1=$$1};1' \
	    | egrep --only-matching --line-regexp '^[\.a-zA-Z0-9_-]+$$' \
	    | LC_COLLATE=C sort \
	    | uniq > $@

$(LISTS_ROOT)/gdpr/srcs/%:
	mkdir -p $(dir $@)
	wget -O $@ https://$*

$(LISTS_ROOT)/exchanges/users.txt: $(LOCAL_EXCHANGE_LISTS)
	cat $(LOCAL_EXCHANGE_LISTS) \
	    | awk '{$$1=$$1};1' \
	    | egrep --only-matching --line-regexp '^[\.a-zA-Z0-9_-]+$$' \
	    | LC_COLLATE=C sort \
	    | uniq > $@

$(LISTS_ROOT)/exchanges/srcs/%:
	mkdir -p $(dir $@)
	wget -O $@ https://$*


$(LISTS_ROOT)/%.json: $(LISTS_ROOT)/%.txt
	cat $< | jq -R -s -c 'split("\n")|map(select(. != ""))' > $@
	-rm $<

$(LISTS_ROOT)/%.ts: $(LISTS_ROOT)/%.json
	cat <(echo -n "export const users: Set<string> = new Set(") $<  <(echo  ")") | prettier --parser typescript --stdin --no-semi --single-quote > $@
	-rm $<
	-rm -rf $(LISTS_ROOT)/$(*D)/srcs

.PHONY: clean-lists
clean-lists:
	-rm -rf $(LISTS_ROOT)/gdpr/* $(LISTS_ROOT)/bad_actors/* $(LISTS_ROOT)/exchanges/*

.PHONY: user-accounts
user-accounts: $(USER_ACCTS_FILE)

.PHONY: clean-user-accounts
clean-user-accounts:
	-rm $(USER_ACCTS_FILE)

$(USER_ACCTS_FILE):
	./node_modules/.bin/ts-node src/user-search/scripts/load_accounts.ts > $@