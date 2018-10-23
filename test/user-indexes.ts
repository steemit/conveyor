
import * as assert from 'assert'

import 'mocha'
import * as sinon from 'sinon'
const nodecache = require( 'node-cache' )
const trieLib = require('trie-prefix-tree')
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

import {buildAccountsTrie, loadAccountNames} from '../src/user-search/indexes'

describe('user indexes', function(this) {
    beforeEach( function(this) {
        const fakeSteemdClient = {
            call: async function fakeCall(api: string, method: string, params: any[]) {
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
        const cache = new nodecache({stdTTL: 600, checkperiod: 60})
        this.fakeCacheClient = new CachingClient(cache,
                                                 {stdTTL: 600, checkperiod: 60},
                                                 fakeSteemdClient)
    })
    it('should return an load all accounts', async function(this) {
        const results = await loadAccountNames(this.fakeCacheClient)
        const expected = new Set(lookupAccountsResponse)
        assert.deepEqual(results, expected)
    })
    it('should return the account names trie', async function(this) {
        const results = await buildAccountsTrie(this.fakeCacheClient)
        const expected = trieLib(Array.from(new Set(lookupAccountsResponse)))
        assert.deepEqual(results.tree(), expected.tree())
    })
})
