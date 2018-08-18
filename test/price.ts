import 'mocha'
import * as assert from 'assert'
import * as http from 'http'
import * as UUID from 'uuid/v4'
import {utils} from '@steemit/koa-jsonrpc'
import {PrivateKey} from 'dsteem'

import {RPC, assertThrows} from './common'

import {app} from './../src/server'

describe('price', function() {
    this.slow(1000)

    const port = process.env['TEST_HTTP_PORT'] ? parseInt(process.env['TEST_HTTP_PORT'] as string) : 63205
    const server = http.createServer(app.callback())

    before((done) => { server.listen(port, 'localhost', done) })
    after((done) => { server.close(done) })

    const rpc = new RPC(port)

    it('should get prices', async function() {
        const rv = await rpc.call('conveyor.get_prices')
        assert.deepEqual(rv, { steem_sbd: 0.2, steem_usd: 5, steem_vest: 42 })
    })
})
