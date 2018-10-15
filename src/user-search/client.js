"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const dsteem_1 = require("dsteem");
const _ = require("lodash");
const moment = require("moment");
const nodecache = require('node-cache');
const logger_1 = require("../logger");
const user_1 = require("./user");
var FollowType;
(function (FollowType) {
    FollowType[FollowType["undefined"] = 0] = "undefined";
    FollowType[FollowType["blog"] = 1] = "blog";
    FollowType[FollowType["ignore"] = 2] = "ignore";
})(FollowType || (FollowType = {}));
class CachingClient {
    constructor(cache, cacheOptions = { stdTTL: 600, checkperiod: 60 }, client, address = 'https://api.steemit.com') {
        this.cache = cache;
        this.cacheOptions = cacheOptions;
        this.client = client;
        this.address = address;
        this.pageSize = 1000;
        if (cache === undefined) {
            this.cache = new nodecache(cacheOptions);
        }
        else {
            this.cache = cache;
        }
        if (client === undefined) {
            this.client = new dsteem_1.Client(address);
        }
        else {
            this.client = client;
        }
    }
    async getExtendedAccount(account) {
        const [result] = await this.call('condenser_api', 'get_accounts', [[account]]);
        return result;
    }
    async getFollowCount(account) {
        return await this.call('condenser_api', 'get_follow_count', [account]);
    }
    async *followGen(method, account, followType, start = '') {
        let pageCount = 0;
        while (true) {
            pageCount += 1;
            const results = await this.call('condenser_api', method, [
                account,
                start,
                followType,
                this.pageSize
            ]);
            const lastResult = results[results.length - 1];
            yield* results;
            logger_1.logger.info(`p:${pageCount} start:${start} last:${lastResult} length:${results.length}`);
            if (start === lastResult || results.length < this.pageSize) {
                break;
            }
            if (method === 'get_followers') {
                start = lastResult['follower'];
            }
            else {
                start = lastResult['following'];
            }
        }
    }
    async getFollowers(account, start = 0, limit = 1000) {
        const followType = FollowType.blog;
        const followers = [];
        for await (const results of this.followGen('get_followers', account, followType)) {
            followers.push(results);
        }
        return followers;
    }
    async getIgnored(account, start = 0, limit = 1000) {
        const followType = FollowType.ignore;
        const followers = [];
        for await (const results of this.followGen('get_followers', account, followType)) {
            followers.push(results);
        }
        return followers;
    }
    async getFollowing(account, start = 0, limit = 1000) {
        const followType = FollowType.blog;
        const followers = [];
        for await (const results of this.followGen('get_following', account, followType)) {
            followers.push(results);
        }
        return followers;
    }
    async *accountHistoryGenerator(account) {
        let pointer = -1;
        let pageCount = 0;
        let accountHistoryLength;
        let totalPages = 1;
        while (true) {
            pageCount += 1;
            const resultsPage = await this.call('condenser_api', 'get_account_history', [account, pointer, this.pageSize]);
            if (pageCount === 1) {
                accountHistoryLength = resultsPage[resultsPage.length - 1][0];
                pointer = accountHistoryLength;
                totalPages = Math.ceil(accountHistoryLength / this.pageSize);
            }
            yield* _.reverse(resultsPage);
            if (pageCount === totalPages) {
                break;
            }
            else {
                pointer = pointer - this.pageSize;
                if (pointer <= this.pageSize) {
                    pointer = this.pageSize;
                }
            }
        }
    }
    async *accountHistoryNewerThanGenerator(account, days = 30) {
        const maxAge = moment.utc().subtract(days, 'days');
        for await (const result of this.accountHistoryGenerator(account)) {
            if (moment.utc(_.get(result, [1, 'timestamp'])).isAfter(maxAge)) {
                yield result;
            }
            else {
                break;
            }
        }
    }
    async getAccountTransferTargetCounts(account, days = 30) {
        const key = `accountTransferTargetCounts__${account}__${days}`;
        const cachedResult = this.cache.get(key);
        if (cachedResult !== undefined) {
            logger_1.logger.info(`hit ${key}`);
            return cachedResult;
        }
        else {
            const accounts = [];
            for await (const item of this.accountHistoryNewerThanGenerator(account, days)) {
                if (_.get(item, [1, 'op', 0]) === 'transfer') {
                    accounts.push(_.get(item, [1, 'op', 1, 'to']));
                }
            }
            const counts = _.countBy(accounts);
            this.cache.set(key, counts);
            logger_1.logger.info(`set ${key}`);
            return counts;
        }
    }
    async loadAccountInfo(account, days = 30) {
        return await Promise.all([
            this.getExtendedAccount(account),
            this.getFollowCount(account),
            this.getAccountTransferTargetCounts(account, days),
            this.getFollowers(account),
            this.getFollowing(account),
            this.getIgnored(account)
        ]);
    }
    async call(api, method, params, ttl) {
        const key = `call__${api}__${method}__${params}`;
        const cachedResult = this.cache.get(key);
        if (cachedResult !== undefined) {
            logger_1.logger.info(`hit ${key}`);
            return cachedResult;
        }
        else {
            const result = await this.client.call(api, method, params);
            if (ttl !== undefined) {
                this.cache.set(key, result, ttl);
                logger_1.logger.info(`set ttl:${ttl} ${key}`);
            }
            else {
                this.cache.set(key, result);
                logger_1.logger.info(`set ttl:${ttl} ${key}`);
            }
            return result;
        }
    }
    cacheUserAccount(userAccount) {
        const key = `UserAccount_${userAccount.account}`;
        this.cache.set(key, userAccount);
        logger_1.logger.info(`set ${key}`);
    }
    loadUserAccount(account) {
        const key = `UserAccount_${account}`;
        return this.cache.get(key);
    }
    cacheUserContext(userContext) {
        const key = `UserContext__${userContext.account}__${userContext.context_account}`;
        this.cache.set(key, userContext);
        logger_1.logger.info(`set ${key}`);
    }
    loadUserContext(account, contextAccount) {
        const key = `UserContext__${account}__${contextAccount}`;
        return this.cache.get(key);
    }
    async loadAccount(account, contextAccount, days = 30) {
        const userAccountKey = `UserAccount__${account}`;
        let userAccount = this.cache.get(userAccountKey);
        if (userAccount === undefined) {
            const [extendedAccount, followCount, accountTransferTargetCount, followers, following, ignored] = await this.loadAccountInfo(account, days);
            userAccount = new user_1.UserAccount(extendedAccount, followCount, accountTransferTargetCount, followers, following, ignored);
            this.cache.set(userAccountKey, userAccount);
        }
        let userAccountJSON = userAccount.toJSON();
        if (contextAccount !== undefined) {
            const userContextKey = `UserContext__${account}__${contextAccount}`;
            let userContext = this.cache.get(userContextKey);
            if (userContext === undefined) {
                userContext = userAccount.userContext(contextAccount);
                this.cache.set(userContextKey, userContext);
            }
            userAccountJSON = _.merge(userAccountJSON, userContext.toJSON());
        }
        return userAccountJSON;
    }
    async loadAccounts(accounts, contextAccount, days = 30) {
        const promises = [];
        for (const account of accounts) {
            promises.push(this.loadAccount(account, contextAccount, days));
        }
        return Promise.all(promises);
    }
}
exports.CachingClient = CachingClient;
