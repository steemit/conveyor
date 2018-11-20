import * as assert from 'assert'
import 'mocha'
const nodecache = require('node-cache')
import { CachingClient } from '../src/user-search/client'
import {AccountNameTrie} from '../src/user-search/indexes'
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

describe('AccountNameTrie', function(this) {
    before(function(this) {
        this.accountSet = new Set(lookupAccountsResponse)
    })
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
    })
    it('should create a new trie from Set', async function(this) {
        const accounts = new AccountNameTrie(this.accountSet, this.fakeCacheclient)
        try {
            assert.strictEqual(accounts.trie.hasWord('a-0'), true)
        } finally {
            accounts.stopRefreshing()
        }
    })
    it('should not be refreshing by default', async function(this) {
        const accounts = new AccountNameTrie(this.accountSet, this.fakeCacheclient)
        try {
            assert.strictEqual(accounts.stopRefreshLoop, true)
            assert.strictEqual(accounts.timerId, undefined)
        } finally {
            accounts.stopRefreshing()
        }
    })
    it('should have a refresh interval of 100000ms', async function(this) {
        const accounts = new AccountNameTrie(this.accountSet, this.fakeCacheclient, 100000)
        assert.strictEqual(accounts.refreshInterval, 100000)
    })
    it('should stop refreshing', async function(this) {
        const accounts = new AccountNameTrie(this.accountSet, this.fakeCacheclient)
        try {
            assert.strictEqual(accounts.stopRefreshLoop, true)
            accounts.startRefreshing()
            assert.strictEqual(accounts.stopRefreshLoop, false)
            accounts.stopRefreshing()
            assert.strictEqual(accounts.stopRefreshLoop, true)
        } finally {
            accounts.stopRefreshing()
        }
    })
    it('should start refreshing', async function(this) {
        const accounts = new AccountNameTrie(this.accountSet, this.fakeCacheclient)
        try {
            assert.strictEqual(accounts.stopRefreshLoop, true)
            accounts.startRefreshing()
            assert.strictEqual(accounts.stopRefreshLoop, false)
        } finally {
            accounts.stopRefreshing()
        }
    })
    it('should store timerId', async function(this) {
        const accounts = new AccountNameTrie(this.accountSet, this.fakeCacheclient)
        try {
            assert.strictEqual(accounts.timerId, undefined)
            accounts.startRefreshing()
            assert.strictEqual(typeof(accounts.timerId), 'object')
        } finally {
            accounts.stopRefreshing()
        }
    })
})
