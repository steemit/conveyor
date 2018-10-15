"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const config = require("config");
const trie_prefix_tree2_1 = require("trie-prefix-tree2");
const users_1 = require("../../lists/gdpr/users");
const indexes_1 = require("./indexes");
const ADMIN_ACCOUNT = config.get('admin_role');
async function getAccount(client, account, context_account) {
    // this.assert(this.account === ADMIN_ACCOUNT, 'Unauthorized') FIXME
    if (users_1.users.has(account)) {
        return [];
    }
    return await client.loadAccount(account, context_account);
}
exports.getAccount = getAccount;
async function autocompleteAccount(client, account, account_substring) {
    this.assert(this.account === ADMIN_ACCOUNT, 'Unauthorized');
    const userAccount = client.loadAccount(account);
    const trie = new trie_prefix_tree2_1.Trie([]);
    const globalAccountNames = indexes_1.matchPrefix(trie, account_substring);
    const friendAccountNames = new Set(globalAccountNames.filter((x) => userAccount.following.has(x)));
    const recentAccountNames = new Set(globalAccountNames.filter((x) => userAccount.recentSendAccounts().has(x)));
    return {
        global: globalAccountNames.length < 10 ? globalAccountNames : [],
        friends: await client.loadAccounts(friendAccountNames),
        recent: await client.loadAccounts(recentAccountNames)
    };
}
exports.autocompleteAccount = autocompleteAccount;
