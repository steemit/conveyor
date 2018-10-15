"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const trie_prefix_tree2_1 = require("trie-prefix-tree2");
async function loadAccountNames(client) {
    let lowerBoundName = '';
    const limit = 1000;
    const names = new Set();
    while (true) {
        const results = await client.call('condenser_api', 'lookup_accounts', [
            lowerBoundName,
            limit
        ]);
        for (const name of results) {
            names.add(name);
        }
        if (lowerBoundName === results[results.length - 1]) {
            break;
        }
        lowerBoundName = results[results.length - 1];
    }
    return names;
}
exports.loadAccountNames = loadAccountNames;
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
