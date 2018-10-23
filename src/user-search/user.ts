import { ExtendedAccount } from 'dsteem'
import * as _ from 'lodash'
import { FollowCountReturn, FollowReturn, TransferTargetCounts } from './client'
import { getUserTags } from './lists'

export interface UserAccountJSON {
    account: string
    vote_sp: number
    joined_at: string
    reputation: string | number
    tags: string[]
    value_sp: string
    followers_count: number
    following_count: number
    context_account?: string
    context_recent_sends?: number
    context_is_following?: boolean
    context_is_follower?: boolean
    context_is_muted?: boolean
}

export class UserAccount {
    /* tslint:disable:variable-name */
    public readonly account: string
    public readonly vote_sp: number
    public readonly joined_at: string
    public readonly reputation: string | number
    public readonly tags: string[] // abuse, exchange, verified, none
    public readonly value_sp: string
    public readonly followers_count: number
    public readonly following_count: number
    /* tslint:enable:variable-name */
    private readonly extendedAccount: ExtendedAccount
    private readonly followCount: FollowCountReturn
    private readonly accountTransferTargetCount: TransferTargetCounts
    private readonly followers: Set<string>
    private readonly following: Set<string>
    private readonly ignored: Set<string>

    constructor(
        extendedAccount: ExtendedAccount,
        followCount: FollowCountReturn,
        accountTransferTargetCount: TransferTargetCounts,
        followers: FollowReturn[],
        following: FollowReturn[],
        ignored: FollowReturn[]
    ) {
        this.account = extendedAccount['name']
        this.extendedAccount = extendedAccount

        this.followCount = followCount
        this.accountTransferTargetCount = accountTransferTargetCount
        this.followers = new Set(
            _.map(followers, (value) => {
                return _.get(value, ['follower'])
            })
        )
        this.following = new Set(
            _.map(following, (value) => {
                return _.get(value, 'following')
            })
        )
        this.ignored = new Set(
            _.map(ignored, (value) => {
                return _.get(value, 'follower')
            })
        )

        this.vote_sp = extendedAccount.voting_power
        this.joined_at = extendedAccount.created
        this.reputation = extendedAccount.reputation
        this.tags = getUserTags(this.account)
        this.value_sp = `${extendedAccount.balance}` // FIXME
        this.followers_count = this.followCount.follower_count
        this.following_count = this.followCount.following_count
    }

    public isFollower(account: string): boolean {
        return this.followers.has(account)
    }

    public isFollowing(account: string): boolean {
        return this.following.has(account)
    }

    public isIgnored(account: string): boolean {
        return this.ignored.has(account)
    }

    public recentSendsCount(account: string): number {
        const sends = this.accountTransferTargetCount[account]
        if (sends !== undefined) {
            return sends
        } else {
            return 0
        }
    }

    public recentSendAccounts(): Set<string> {
        return new Set(_.keys(this.accountTransferTargetCount))
    }

    public userContext(account: string) {
        return new UserContext(
            this.account,
            account,
            this.recentSendsCount(account),
            this.isFollowing(account),
            this.isFollower(account),
            this.isIgnored(account)
        )
    }

    public toJSONWithContext(userContext?: UserContext): UserAccountJSON {
        if (userContext !== undefined) {
            return _.merge(this.toJSON(),
                    userContext.toJSON())
        } else {
            return this.toJSON()
        }
    }

    public toJSON(): UserAccountJSON {
        return {
            account: this.account,
            vote_sp: this.vote_sp,
            joined_at: this.joined_at,
            reputation: this.reputation,
            tags: this.tags,
            value_sp: this.value_sp,
            followers_count: this.followers_count,
            following_count: this.following_count
        }
    }

    public toString(): string {
        return JSON.stringify(this.toJSON())
    }
}

export interface UserContextJSON {
    context_account: string
    context_recent_sends: number
    context_is_following: boolean
    context_is_follower: boolean
    context_is_muted: boolean
}

export class UserContext {
    public readonly cacheKey: string
    /* tslint:disable:variable-name */
    constructor(
        public readonly account: string,
        public readonly context_account: string,
        public readonly recent_sends: number,
        public readonly is_following: boolean,
        public readonly is_follower: boolean,
        public readonly is_muted: boolean
    ) {
        this.cacheKey = `UserContext__${account}__${context_account}`
    }
    /* tslint:enable:variable-name */

    public toJSON(): UserContextJSON {
        return {
            context_account: this.context_account,
            context_recent_sends: this.recent_sends,
            context_is_following: this.is_following,
            context_is_follower: this.is_follower,
            context_is_muted: this.is_muted
        }
    }
}
