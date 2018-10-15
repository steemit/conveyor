import * as assert from 'assert'
import 'mocha'
import * as sinon from 'sinon'
const nodecache = require('node-cache')
import { CachingClient } from '../src/user-search/client'
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

describe('user client', function(this) {
    beforeEach(function(this) {
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
    })
    it('should return an ExtendedAccount', async function(this) {
        const results = await this.fakeCacheClient.getExtendedAccount('steemit')
        assert.deepEqual(results, getAccountsResponse)
    })
    it('should cache an ExtendedAccount', async function(this) {
        const results = await this.fakeCacheClient.getExtendedAccount('steemit')
        const key = 'call__condenser_api__get_accounts__steemit'
        assert.deepEqual(this.fakeCacheClient.cache.get(key), [
            getAccountsResponse
        ])
    })
    it('should return follow counts', async function(this) {
        const results = await this.fakeCacheClient.getFollowCount('steemit')
        assert.deepEqual(results, getFollowCountResponse)
    })
    it('should cache follow counts', async function(this) {
        const results = await this.fakeCacheClient.getFollowCount('steemit')
        const key = 'call__condenser_api__get_follow_count__steemit'
        assert.deepEqual(
            this.fakeCacheClient.cache.get(key),
            getFollowCountResponse
        )
    })
    it('should return account transfer target counts', async function(this) {
        const results = await this.fakeCacheClient.getAccountTransferTargetCounts(
            'steemit'
        )
        assert.deepEqual(results, { steemit: 1 })
    })
    it('should cache account transfer target counts', async function(this) {
        const results = await this.fakeCacheClient.getAccountTransferTargetCounts(
            'steemit'
        )
        const key = 'accountTransferTargetCounts__steemit__30'
        assert.deepEqual(this.fakeCacheClient.cache.get(key), {})
    })
    it('should return followers', async function(this) {
        // const client = new CachingClient()
        // const results = await client.getFollowers2('steemit')
        const results = await this.fakeCacheClient.getFollowers('steemit')
        assert.deepEqual(results, getFollowersResponse)
    })
    it('should cache followers', async function(this) {
        const results = await this.fakeCacheClient.getFollowers('steemit')
        const key = 'call__condenser_api__get_followers__steemit,,1,1000'
        assert.deepEqual(
            this.fakeCacheClient.cache.get(key),
            getFollowersResponse
        )
    })
    it('should return following', async function(this) {
        const results = await this.fakeCacheClient.getFollowing('steemit')
        assert.deepEqual(results, getFollowingResponse)
    })
    it('should cache following', async function(this) {
        const results = await this.fakeCacheClient.getFollowing('steemit')
        const key = 'call__condenser_api__get_following__steemit,,1,1000'
        assert.deepEqual(
            this.fakeCacheClient.cache.get(key),
            getFollowingResponse
        )
    })
    it('should return ignored', async function(this) {
        const results = await this.fakeCacheClient.getIgnored('steemit')
        assert.deepEqual(results, getIgnoredResponse)
    })
    it('should cache ignored', async function(this) {
        const results = await this.fakeCacheClient.getIgnored('steemit')
        const key = 'call__condenser_api__get_followers__steemit,,2,1000'
        assert.deepEqual(
            this.fakeCacheClient.cache.get(key),
            getIgnoredResponse
        )
    })
    it('should load a UserAccount', async function(this) {
        const results = await this.fakeCacheClient.loadAccount('steemit')
        assert.deepEqual(results, expectedUserAccountJSON)
    })
    it('should cache a UserAccount', async function(this) {
        const results = await this.fakeCacheClient.loadAccount('steemit')
        const key = 'UserAccount__steemit'
        assert.deepEqual(this.fakeCacheClient.cache.get(key).toJSON(), expectedUserAccountJSON)
    })
    it('should load array of UserAccounts', async function(this) {
        const accounts = ['steemit', 'steemit']
        const results = await this.fakeCacheClient.loadAccounts(accounts)
        assert.deepEqual(results, [expectedUserAccountJSON, expectedUserAccountJSON])
    })
    it('should load empty Array of UserAccounts', async function(this) {
        const accounts = []
        const results = await this.fakeCacheClient.loadAccounts(accounts)
        assert.deepEqual(results, [])
    })
    it('should load Set of UserAccounts', async function(this) {
        const accounts = new Set(['steemit'])
        const results = await this.fakeCacheClient.loadAccounts(accounts)
        assert.deepEqual(results, [expectedUserAccountJSON])
    })
    it('should load empty Set of UserAccounts', async function(this) {
        const accounts = new Set()
        const results = await this.fakeCacheClient.loadAccounts(accounts)
        assert.deepEqual(results, [])
    })
})
