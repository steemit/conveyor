"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const _ = require("lodash");
const lists_1 = require("./lists");
class UserAccount {
    constructor(extendedAccount, followCount, accountTransferTargetCount, followers, following, ignored) {
        this.account = extendedAccount['name'];
        this.extendedAccount = extendedAccount;
        this.followCount = followCount;
        this.accountTransferTargetCount = accountTransferTargetCount;
        this.followers = new Set(_.map(followers, (value) => {
            return _.get(value, 'follower');
        }));
        this.following = new Set(_.map(following, (value) => {
            return _.get(value, 'following');
        }));
        this.ignored = new Set(_.map(ignored, (value) => {
            return _.get(value, 'following');
        }));
        this.vote_sp = extendedAccount.voting_power;
        this.joined_at = extendedAccount.created;
        this.reputation = extendedAccount.reputation;
        this.tags = lists_1.getUserTags(this.account);
        this.value_sp = `${extendedAccount.balance}`; // FIXME
        this.followers_count = this.followCount.follower_count;
        this.following_count = this.followCount.following_count;
    }
    isFollower(account) {
        return this.followers.has(account);
    }
    isFollowing(account) {
        return this.following.has(account);
    }
    isIgnored(account) {
        return this.ignored.has(account);
    }
    recentSendsCount(account) {
        const sends = this.accountTransferTargetCount[account];
        if (sends !== undefined) {
            return sends;
        }
        else {
            return 0;
        }
    }
    recentSendAccounts() {
        return new Set(_.keys(this.accountTransferTargetCount));
    }
    userContext(account) {
        return new UserContext(this.account, account, this.recentSendsCount(account), this.isFollowing(account), this.isFollower(account), this.isIgnored(account));
    }
    toJSONWithContext(userContext) {
        if (userContext !== undefined) {
            return _.merge(this.toJSON(), userContext.toJSON());
        }
        else {
            return this.toJSON();
        }
    }
    toJSON() {
        return {
            account: this.account,
            vote_sp: this.vote_sp,
            joined_at: this.joined_at,
            reputation: this.reputation,
            tags: this.tags,
            value_sp: this.value_sp,
            followers_count: this.followers_count,
            following_count: this.following_count
        };
    }
}
exports.UserAccount = UserAccount;
class UserContext {
    /* tslint:disable:variable-name */
    constructor(account, context_account, recent_sends, is_following, is_follower, is_muted) {
        this.account = account;
        this.context_account = context_account;
        this.recent_sends = recent_sends;
        this.is_following = is_following;
        this.is_follower = is_follower;
        this.is_muted = is_muted;
        this.cacheKey = `UserContext__${account}__${context_account}`;
    }
    /* tslint:enable:variable-name */
    toJSON() {
        return {
            context_account: this.context_account,
            context_recent_sends: this.recent_sends,
            context_is_following: this.is_following,
            context_is_follower: this.is_follower,
            context_is_muted: this.is_muted
        };
    }
}
exports.UserContext = UserContext;
