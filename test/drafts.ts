import 'mocha'
import * as assert from 'assert'
import * as http from 'http'
import * as UUID from 'uuid/v4'
import {utils} from '@steemit/koa-jsonrpc'
import {PrivateKey} from 'dsteem'

import {RPC, assertThrows} from './common'

import {app} from './../src/server'

describe('drafts', function() {
    this.slow(1000)
    this.timeout(10 * 1000)

    const port = process.env['TEST_HTTP_PORT'] ? parseInt(process.env['TEST_HTTP_PORT'] as string) : 63205
    const server = http.createServer(app.callback())

    before((done) => { server.listen(port, 'localhost', done) })
    after((done) => { server.close(done) })

    const rpc = new RPC(port)
    const uuid = UUID()

    const signer = {
        account: 'foo',
        key: PrivateKey.fromLogin('foo', 'barman', 'posting').toString(),
    }

    it('should create draft', async function() {
        const rv = await rpc.signedCall('conveyor.save_draft', signer, 'foo', {title: 'foo', body: 'bar'})
        assert.equal(typeof rv.uuid, 'string')
    })

    it('should create draft with uuid', async function() {
        const rv = await rpc.signedCall('conveyor.save_draft', signer, 'foo', {uuid, title: 'foo', body: 'bar'})
        assert.equal(rv.uuid, uuid)
    })

    it('should update draft', async function() {
        const rv = await rpc.signedCall('conveyor.save_draft', signer, 'foo', {uuid, title: 'foo2', body: 'bar2'})
        assert.equal(rv.uuid, uuid)
    })

    it('should delete draft', async function() {
        const rv1 = await rpc.signedCall('conveyor.save_draft', signer, 'foo', {title: 'foo', body: 'bar'})
        const rv2 = await rpc.signedCall('conveyor.remove_draft', signer, 'foo', rv1.uuid)
        assert.equal(rv2.uuid, rv1.uuid)
    })

    it('should list drafts', async function() {
        const rv = await rpc.signedCall('conveyor.list_drafts', signer, 'foo')
        assert.equal(rv.length, 2)
        assert.deepEqual(rv[1], {uuid, title: 'foo2', body: 'bar2'})
    })

    it('should reject unauthorized', async function() {
        const error = await assertThrows(async () => {
            await rpc.signedCall('conveyor.list_drafts', signer, 'notfoo')
        })
        assert.equal(error.message, 'Unauthorized')
    })

})
