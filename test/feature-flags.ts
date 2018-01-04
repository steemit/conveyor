import 'mocha'
import * as assert from 'assert'
import * as http from 'http'
import * as UUID from 'uuid/v4'
import * as fs from 'fs'
import {utils} from '@steemit/koa-jsonrpc'
import {PrivateKey} from 'dsteem'
import {join as joinPath} from 'path'

import {RPC} from './common'

// first 1000 steem accounts for every letter of the alphabet
const usernames = fs.readFileSync(joinPath(__dirname, 'usernames.txt'))
    .toString('utf8').split('\n')

import {app} from './../src/server'
import {getFlags} from './../src/feature-flags'

describe('feature flags', function() {
    this.slow(3000)
    this.timeout(6000)

    const port = process.env['TEST_HTTP_PORT'] ? parseInt(process.env['TEST_HTTP_PORT'] as string) : 63205
    const server = http.createServer(app.callback())

    before((done) => { server.listen(port, 'localhost', done) })
    after((done) => { server.close(done) })

    const rpc = new RPC(port)
    const uuid = UUID()

    const adminSigner = {
        account: 'foo',
        key: PrivateKey.fromLogin('foo', 'barman', 'posting').toString(),
    }

    const userSigner = {
        account: 'bar',
        key: PrivateKey.fromLogin('bar', 'fooman', 'posting').toString(),
    }

    it('should set flag probabilities', async function() {
        await rpc.signedCall('conveyor.set_feature_flag_probability', adminSigner, 'unicorn_mode', 0.05)
        await rpc.signedCall('conveyor.set_feature_flag_probability', adminSigner, 'darkness', 0.99)
        await rpc.signedCall('conveyor.set_feature_flag_probability', adminSigner, 'obsolete', 0)
        await rpc.signedCall('conveyor.set_feature_flag_probability', adminSigner, 'always', 1)
        await rpc.signedCall('conveyor.set_feature_flag_probability', adminSigner, 'coin_toss', 0.5)
        const probs = await rpc.signedCall('conveyor.get_feature_flag_probabilities', adminSigner)
        assert.deepEqual(probs, {unicorn_mode: 0.05, darkness: 0.99, always: 1, coin_toss: 0.5})
    })

    it('should set flag for user', async function() {
        await rpc.signedCall('conveyor.set_feature_flag', adminSigner, 'foo', 'always', false)
        await rpc.signedCall('conveyor.set_feature_flag', adminSigner, 'foo', 'unicorn_mode', true)
        await rpc.signedCall('conveyor.set_feature_flag', adminSigner, 'foo', 'special_snowflake', true)
        const flags = await rpc.signedCall('conveyor.get_feature_flags', adminSigner, 'foo')
        assert.deepEqual(flags, {always: false, unicorn_mode: true, special_snowflake: true, darkness: true, coin_toss: true})
    })

    it('should resolve specific flag', async function() {
        const coin_toss = await rpc.signedCall('conveyor.get_feature_flag', userSigner, 'bar', 'coin_toss')
        const never_heard_of = await rpc.signedCall('conveyor.get_feature_flag', userSigner, 'bar', 'never_heard_of')
        assert.deepEqual({coin_toss, never_heard_of}, {coin_toss: true, never_heard_of: false})
    })

    it('should have correct random distributions', async function() {
        this.timeout(30 * 1000)

        let total = 0
        const distr = {}
        const fakeCtx = {account: 'foo', assert: () => {}}
        for (let i = 0; i < usernames.length; i+=2) { // takes way to long to sample all
            const username = usernames[i]
            const flags = await getFlags.call(fakeCtx, username)
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
        assert.deepEqual(result, {unicorn_mode: 5, darkness: 99, always: 100, coin_toss: 50})
    })

})
