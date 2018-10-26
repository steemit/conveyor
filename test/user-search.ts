import * as assert from 'assert'
import 'mocha'
const request = require('supertest')
const nodecache = require('node-cache')
import {KoaAppWithCustomContext} from '../src/server'
import { CachingClient } from '../src/user-search/client'


import {JsonRpcAuth, requestLogger, rpcLogger} from '@steemit/koa-jsonrpc'
import * as config from 'config'
import * as Koa from 'koa'
import * as Router from 'koa-router'
import {logger} from '../src/logger'
import {AccountNameTrie} from '../src/user-search/indexes'
import * as userSearch from '../src/user-search/search'
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

const expectedUserAccountJSONWithContext = {
    account: 'steemit',
    followers_count: 3202,
    following_count: 0,
    joined_at: '2016-03-24T17:00:21',
    reputation: '12944616889',
    tags: ['none'],
    value_sp: '2000000.580 STEEM',
    vote_sp: 0,
    context_account: 'steemit',
    context_is_follower: false,
    context_is_following: false,
    context_is_muted: false,
    context_recent_sends: 612
}

function createApp(cacheClient, userAccountTrie?) {
    const app = new Koa() as KoaAppWithCustomContext
    app.context.cacheClient = cacheClient
    // app.context.userAccountTrie = userAccountTrie || loadAccountsTrie(userAccountNames)

    const router = new Router()
    const rpc = new JsonRpcAuth(config.get('rpc_node'), config.get('name'))

    app.proxy = true
    app.on('error', (error) => {
        logger.error(error, 'Application error')
    })

    app.use(requestLogger(logger))
    app.use(rpcLogger(logger))
    router.post('/', rpc.middleware)
    app.use(router.routes())
    rpc.register('get_account', userSearch.getAccount)
    rpc.register('autocomplete_account', userSearch.autocompleteAccount)
    return app
}

describe('user search functions', function() {
    this.slow(1000)
    this.timeout(10 * 1000)

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

    it('should return an account', async function() {
        const app = createApp(this.fakeCacheClient)
        const response = await request(app.callback()).post('/')
            .set('Content-Type', 'application/json')
            .send({id: 1, jsonrpc: '2.0', method: 'conveyor.get_account', params:{account: 'steemit'}})
        const jsonRPCResponse = JSON.parse(response.text)
        const result = jsonRPCResponse.result
        assert.deepEqual(result, expectedUserAccountJSON)
    })
    it('should return account suggestions', async function() {
        const app = createApp(this.fakeCacheClient)
        const acctNames = new Set(['steemit', 'steemit2', 'aaronburt'])
        app.context.cacheClient = this.fakeCacheClient
        app.context.userAccountTrie = new AccountNameTrie(acctNames, this.fakeCachingClient, 100000, true)
        const response = await request(app.callback()).post('/')
            .set('Content-Type', 'application/json')
            .send({id: 1,
                      jsonrpc: '2.0',
                      method: 'conveyor.autocomplete_account',
                      params: {accountSubstring: 'steemi', account: 'steemit'}})

        const jsonRPCResponse = JSON.parse(response.text)
        const result = jsonRPCResponse.result
        const expected = {
            global: [],
            friends: [],
            recent: [expectedUserAccountJSONWithContext]
        }
        try {
            assert.deepEqual(result['global'], [])
            assert.deepEqual(result['friends'], [])
            assert.deepEqual(result['recent'][0], expectedUserAccountJSONWithContext)
        } finally {
            app.context.userAccountTrie.stopRefreshing()
        }
    })
})
