import * as assert from 'assert'
import 'mocha'
import * as sinon from 'sinon'
const nodecache = require('node-cache')
import { CachingClient } from '../src/user-search/client'
import {UserAccount} from '../src/user-search/user'
import {
    getAccountHistoryResponse,
    getAccountsResponse,
    getFollowCountResponse,
    getFollowersResponse,
    getFollowingResponse,
    getIgnoredResponse,
    lookupAccountsResponse
} from './steemd_responses'

const expectedUserAccountJSON = {
    account: 'steemit',
    followers_count: 3202,
    following_count: 0,
    joined_at: '2016-03-24T17:00:21',
    reputation: '12944616889',
    tags: ['none'],
    value_sp: '2000000.580 STEEM',
    vote_sp: 0
}

describe('UserAccount', function(this) {
    beforeEach(async function(this) {
        const fakeSteemdClient = {
            call: async function fakeCall(
                api: string,
                method: string,
                params: any[]
            ) {
                if (method === 'get_accounts') {
                    return [getAccountsResponse]
                }
                if (method === 'get_follow_count') {
                    return getFollowCountResponse
                }
                if (method === 'get_followers' && params[2] === 1) {
                    return getFollowersResponse
                }
                if (method === 'get_followers' && params[2] === 2) {
                    return getIgnoredResponse
                }
                if (method === 'get_following') {
                    return getFollowingResponse
                }
                if (method === 'get_account_history') {
                    return getAccountHistoryResponse
                }
                if (method === 'lookup_accounts') {
                    return lookupAccountsResponse
                }
            }
        }
        const cache = new nodecache({ stdTTL: 600, checkperiod: 60 })
        this.fakeCacheClient = new CachingClient(
            cache,
            { stdTTL: 600, checkperiod: 60 },
            fakeSteemdClient
        )
        const [userAccount, userContext] = await this.fakeCacheClient.loadAccount('steemit')
        this.testUserAccount = userAccount
        this.testUserContext = userContext
    })
    it('UserAccount.isFollower should return false ', async function(this) {
        const [userAccount, userContext] = await this.fakeCacheClient.loadAccount('steemit')
        assert.deepEqual(userAccount.isFollower('steemit'), false)
    })
    it('UserAccount.isFollower should return true', async function(this) {
        const [userAccount, userContext] = await this.fakeCacheClient.loadAccount('steemit')
        assert.deepEqual(userAccount.isFollower('a-0-0'), true)
    })
    it('UserAccount.isFollowing should return false ', async function(this) {
        const [userAccount, userContext] = await this.fakeCacheClient.loadAccount('steemit')
        assert.deepEqual(userAccount.isFollowing('steemit'), false)
    })
    it('UserAccount.isFollowing should return true', async function(this) {
        const [userAccount, userContext] = await this.fakeCacheClient.loadAccount('steemit')
        assert.deepEqual(userAccount.isFollowing('aaronburt'), true)
    })
    it('UserAccount.isIgnored should return false ', async function(this) {
        const [userAccount, userContext] = await this.fakeCacheClient.loadAccount('steemit')
        assert.deepEqual(userAccount.isIgnored('steemit'), false)
    })
    it('UserAccount.isIgnored should return true', async function(this) {
        const [userAccount, userContext] = await this.fakeCacheClient.loadAccount('steemit')
        assert.deepEqual(userAccount.isIgnored('gthongo'), true)
    })
    it('UserAccount.recentSendAccounts should return Set of account names', async function(this) {
        const [userAccount, userContext] = await this.fakeCacheClient.loadAccount('steemit')
        assert.deepEqual(userAccount.recentSendAccounts('steemit'), new Set(['steemit','steemit2']))
    })
    it('UserAccount.recentSendsCount should return 0', async function(this) {
        const [userAccount, userContext] = await this.fakeCacheClient.loadAccount('steemit')
        assert.deepEqual(userAccount.recentSendsCount('---steemit'), 0)
    })
    it('UserAccount.recentSendsCount should return 612', async function(this) {
        const [userAccount, userContext] = await this.fakeCacheClient.loadAccount('steemit')
        assert.deepEqual(userAccount.recentSendsCount('steemit'), 612)
    })
    it('UserAccount.toJSON should JSON', async function(this) {
        const [userAccount, userContext] = await this.fakeCacheClient.loadAccount('steemit')
        assert.deepEqual(userAccount.toJSON(), expectedUserAccountJSON)
    })
    it('UserAccount.toJSONWithContext should JSON', async function(this) {
        const [userAccount, userContext] = await this.fakeCacheClient.loadAccount('steemit')
        assert.deepEqual(userAccount.toJSONWithContext(), expectedUserAccountJSON)
    })
    it('UserAccount.toString should a string', async function(this) {
        const [userAccount, userContext] = await this.fakeCacheClient.loadAccount('steemit')
        assert.deepEqual(JSON.parse(userAccount.toString()), expectedUserAccountJSON)
    })
})
