import 'mocha'
import * as assert from 'assert'
import * as http from 'http'
import {PrivateKey} from 'dsteem'
import {utils} from '@steemit/koa-jsonrpc'

import {RPC, assertThrows} from './common'

import {app} from './../src/server'
import {db} from './../src/database'

describe('user tags', function() {
    this.slow(1000)
    this.timeout(10 * 1000)

    const port = process.env['TEST_HTTP_PORT'] ? parseInt(process.env['TEST_HTTP_PORT'] as string) : 63205
    const server = http.createServer(app.callback())

    before(async () => { await db.sync() })
    before((done) => { server.listen(port, 'localhost', done) })
    after((done) => { server.close(done) })

    const rpc = new RPC(port)

    const adminSigner = {
        account: 'foo',
        key: PrivateKey.fromLogin('foo', 'barman', 'posting').toString(),
    }

    const userSigner = {
        account: 'bar',
        key: PrivateKey.fromLogin('bar', 'fooman', 'posting').toString(),
    }

    it('should define tags', async function() {
        await rpc.signedCall('conveyor.define_tag', adminSigner, 'foo', 'this is my tag')
        await rpc.signedCall('conveyor.define_tag', adminSigner, 'bar', 'he had a drink at the bar')
        await rpc.signedCall('conveyor.define_tag', adminSigner, 'crooked', 'should be investigated')
        await assertThrows(async () => {
            // rejects invalid tag names
            await rpc.signedCall('conveyor.define_tag', adminSigner, 'bad tag man', 'no spaces please')
        })
        await assertThrows(async () => {
            // rejects invalid description
            await rpc.signedCall('conveyor.define_tag', adminSigner, 'brown_noise')
        })
        await assertThrows(async () => {
            // rejects non admin signature
            await rpc.signedCall('conveyor.define_tag', userSigner, 'banana', 'had one for breakfast')
        })
    })

    it('should list tags', async function() {
        const tags = await rpc.signedCall('conveyor.list_tags', adminSigner)
        assert.deepEqual(tags, [
            { name: 'bar', description: 'he had a drink at the bar' },
            { name: 'crooked', description: 'should be investigated' },
            { name: 'foo', description: 'this is my tag' } ]
        )
        // rejects non-admin signature
        await assertThrows(async () => await rpc.signedCall('conveyor.list_tags', userSigner))
    })

    it('should assign tag', async function() {
        await rpc.signedCall('conveyor.assign_tag', adminSigner, 'user1', 'bar')
        await rpc.signedCall('conveyor.assign_tag', adminSigner, 'user2', 'bar')
        // rejects non-admin signature
        await assertThrows(async () => await rpc.signedCall('conveyor.assign_tag', userSigner, 'user1', 'foo'))
        // rejects undefined tag
        const error = await assertThrows(async () => await rpc.signedCall('conveyor.assign_tag', adminSigner, 'user1', 'not_a_tag'))
        assert.equal(error.code, 420)
    })

    it('should unassign tag', async function() {
        await rpc.signedCall('conveyor.unassign_tag', adminSigner, 'user1', 'bar')
        // rejects non-admin signature
        await assertThrows(async () => await rpc.signedCall('conveyor.unassign_tag', userSigner, 'user2', 'foo'))
    })

    it('should lookup users by tag', async function() {
        await Promise.all([
            rpc.signedCall('conveyor.assign_tag', adminSigner, 'user3', 'bar'),
            rpc.signedCall('conveyor.assign_tag', adminSigner, 'user3', 'foo'),
            rpc.signedCall('conveyor.assign_tag', adminSigner, 'user4', 'foo'),
        ])
        let rv
        rv = await rpc.signedCall('conveyor.get_users_by_tags', adminSigner, 'bar')
        assert.deepEqual(rv, [ 'user2', 'user3' ])
        rv = await rpc.signedCall('conveyor.get_users_by_tags', adminSigner, ['foo', 'bar'])
        assert.deepEqual(rv, [ 'user3' ])
        // rejects non-admin signature
        await assertThrows(async () => await rpc.signedCall('conveyor.get_users_by_tags', userSigner, 'foo'))
    })

    it('should lookup tags by user', async function() {
        let rv
        rv = await rpc.signedCall('conveyor.get_tags_for_user', adminSigner, 'user2')
        assert.deepEqual(rv, [ 'bar' ])
        rv = await rpc.signedCall('conveyor.get_tags_for_user', adminSigner, 'user3')
        assert.deepEqual(rv, [ 'bar', 'foo' ])
        rv = await rpc.signedCall('conveyor.get_tags_for_user', adminSigner, 'user1')
        assert.deepEqual(rv, [  ])
        await assertThrows(async () => await rpc.signedCall('conveyor.get_tags_for_user', userSigner, 'user1'))
    })

    it('should show tag audit log', async function() {
        await rpc.signedCall('conveyor.unassign_tag', adminSigner, 'user3', 'bar')
        const rv = await rpc.signedCall('conveyor.get_tags_for_user', adminSigner, 'user3', true)
        assert.equal(rv.length, 2)
        assert(rv[0].deletedAt !== null)
        assert(rv[1].deletedAt === null)
        assert.deepEqual(Object.keys(rv[0]).sort(), [ 'createdAt', 'deletedAt', 'id', 'memo', 'tag', 'uid', 'updatedAt' ])
    })

})
