"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const config = require("config");
const users_1 = require("../../user-data/lists/gdpr/users");
const indexes_1 = require("./indexes");
const logger_1 = require("../logger");
const ADMIN_ACCOUNT = config.get('admin_role');
async function getAccount(account, contextAccount) {
    this.assert(this.account === ADMIN_ACCOUNT, 'Unauthorized');
    const client = this.ctx.cacheClient;
    if (users_1.users.has(account)) {
        return [];
    }
    logger_1.logger.info(`getAccount account:${account} contextAccount:${contextAccount}`);
    return await client.loadAccountJSON(account, contextAccount);
}
exports.getAccount = getAccount;
async function autocompleteAccount(account, accountSubstring) {
    this.assert(this.account === ADMIN_ACCOUNT, 'Unauthorized');
    const client = this.ctx.cacheClient;
    const userAccount = client.loadAccount(account);
    const globalAccountNames = indexes_1.matchPrefix(this.ctx.userAccountTrie, accountSubstring);
    const friendAccountNames = new Set(globalAccountNames.filter((x) => userAccount.following.has(x)));
    const recentAccountNames = new Set(globalAccountNames.filter((x) => userAccount.recentSendAccounts().has(x)));
    return {
        global: globalAccountNames.length < 10 ? await client.loadAccountsJSON(globalAccountNames) : [],
        friends: await client.loadAccountsJSON(friendAccountNames),
        recent: await client.loadAccountsJSON(recentAccountNames)
    };
}
exports.autocompleteAccount = autocompleteAccount;
