"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const _ = require("lodash");
const trie_prefix_tree2_1 = require("trie-prefix-tree2");
const logger_1 = require("../logger");
async function loadAccountNames(client, start = '', end) {
    const limit = 1000;
    const names = new Set();
    while (true) {
        logger_1.logger.debug(`loading ${limit} account names start with "${start}"`);
        const results = await client.call('condenser_api', 'lookup_accounts', [
            start,
            limit
        ]);
        for (const name of results) {
            names.add(name);
        }
        logger_1.logger.debug(`added ${results.length} account names`);
        const lastResult = results[results.length - 1];
        if (start === lastResult) {
            logger_1.logger.debug(`${start} === ${lastResult} so loading complete`);
            break;
        }
        if (results.length < limit) {
            logger_1.logger.debug(`results.length of ${results.length} <= ${limit} so loading complete`);
            break;
        }
        if (end !== undefined && lastResult.startsWith(end)) {
            logger_1.logger.debug(`${lastResult}.startsWith(${end}) is true  <= so loading complete`);
            break;
        }
        start = results[results.length - 1];
    }
    return names;
}
exports.loadAccountNames = loadAccountNames;
function concatSets(set, ...iterables) {
    for (const iterable of iterables) {
        for (const item of iterable) {
            set.add(item);
        }
    }
}
async function loadAllAccountNames(client) {
    const starts = [''].concat(Array.from('bcdefghijklmnopqrstuvwxyz'));
    const ends = starts.slice(1);
    const pairs = _.zip(starts, ends);
    const promises = _.map(pairs, (pair) => {
        const [start, end] = pair;
        logger_1.logger.debug(`adding promise loadAccountNames(client,${start},${end}`);
        return loadAccountNames(client, start, end);
    });
    const resultSets = await Promise.all(promises);
    const combined = new Set();
    concatSets(combined, ...resultSets);
    return combined;
}
exports.loadAllAccountNames = loadAllAccountNames;
function loadAccountsTrie(accounts) {
    return new trie_prefix_tree2_1.Trie(Array.from(accounts));
}
exports.loadAccountsTrie = loadAccountsTrie;
async function buildAccountsTrie(client) {
    const names = await loadAccountNames(client);
    return new trie_prefix_tree2_1.Trie(Array.from(names));
}
exports.buildAccountsTrie = buildAccountsTrie;
function matchPrefix(trie, prefix) {
    return trie.getPrefix(prefix, false);
}
exports.matchPrefix = matchPrefix;
function intersectMatches(trie, prefix, otherSet) {
    const matches = trie.getPrefix(prefix, false);
    return new Set(matches.filter((x) => otherSet.has(x)));
}
exports.intersectMatches = intersectMatches;
