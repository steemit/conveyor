import 'mocha'
import * as assert from 'assert'
import * as http from 'http'
import * as UUID from 'uuid/v4'
import * as fs from 'fs'
import {utils} from '@steemit/jsonrpc'
import {join as joinPath} from 'path'

import {makeClient} from './common'

// first 1000 steem accounts for every letter of the alphabet
const usernames = fs.readFileSync(joinPath(__dirname, 'usernames.txt'))
    .toString('utf8').split('\n')

import {app} from './../src/server'

describe('feature flags', function() {
    const port = process.env['TEST_HTTP_PORT'] ? parseInt(process.env['TEST_HTTP_PORT'] as string) : 63205
    const server = http.createServer(app.callback())

    before((done) => { server.listen(port, 'localhost', done) })
    after((done) => { server.close(done) })

    const call = makeClient(port)
    const uuid = UUID()

    it('should set flag probabilities', async function() {
        await call('steemitapi.set_feature_flag_probability', 'unicorn_mode', 0.05)
        await call('steemitapi.set_feature_flag_probability', 'darkness', 0.99)
        await call('steemitapi.set_feature_flag_probability', 'obsolete', 0)
        await call('steemitapi.set_feature_flag_probability', 'always', 1)
        await call('steemitapi.set_feature_flag_probability', 'coin_toss', 0.5)
        const probs = await call('steemitapi.get_feature_flag_probabilities')
        assert.deepEqual(probs, {unicorn_mode: 0.05, darkness: 0.99, always: 1, coin_toss: 0.5})
    })

    it('should set flag for user', async function() {
        await call('steemitapi.set_feature_flag', 'foo', 'always', false)
        await call('steemitapi.set_feature_flag', 'foo', 'unicorn_mode', true)
        await call('steemitapi.set_feature_flag', 'foo', 'special_snowflake', true)
        const flags = await call('steemitapi.get_feature_flags', 'foo')
        assert.deepEqual(flags, {always: false, unicorn_mode: true, special_snowflake: true, darkness: true, coin_toss: true})
    })

    it('should resolve specific flag', async function() {
        const coin_toss = await call('steemitapi.get_feature_flag', 'bar', 'coin_toss')
        const never_heard_of = await call('steemitapi.get_feature_flag', 'bar', 'never_heard_of')
        assert.deepEqual({coin_toss, never_heard_of}, {coin_toss: true, never_heard_of: false})
    })

    it('should have correct random distributions', async function() {
        this.slow(10 * 1000)
        this.timeout(20 * 1000)
        let total = 0
        const distr = {}
        for (let i = 0; i < usernames.length; i+=50) { // takes way to long to sample all
            const username = usernames[i]
            const flags = await call('steemitapi.get_feature_flags', username)
            for (const flag of Object.keys(flags)) {
                if (!distr[flag]) distr[flag] = 0
                if (flags[flag]) distr[flag]++
            }
            total++
        }
        const result = {}
        for (const flag of Object.keys(distr)) {
            result[flag] = Math.round((distr[flag] / total) * 100)
        }
        // close enough
        assert.deepEqual(result, {unicorn_mode: 6, darkness: 99, always: 100, coin_toss: 52})
    })

})
