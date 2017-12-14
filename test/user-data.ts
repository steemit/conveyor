import 'mocha'
import * as assert from 'assert'
import * as http from 'http'
import {PrivateKey} from 'dsteem'
import {utils} from '@steemit/koa-jsonrpc'

import {RPC, assertThrows} from './common'

import {app} from './../src/server'
import {db} from './../src/database'

describe('user data', function() {
    this.slow(10 * 1000)
    this.timeout(20 * 1000)

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

    it('should set user data', async function() {
        await rpc.signedCall('conveyor.set_user_data', userSigner, 'bar', {email: 'foo@bar.com', phone: '+99123123123'})
    })

    it('should get user data', async function() {
        const rv = await rpc.signedCall('conveyor.get_user_data', userSigner, 'bar')
        assert.deepEqual(rv, {email: 'foo@bar.com', phone: '+99123123123'})
    })

    it('should throw on missing data', async function() {
        const error = await assertThrows(async () => {
            await rpc.signedCall('conveyor.set_user_data', adminSigner, 'foo', {phone: '+12345567'})
        })
        assert.deepEqual(error.data, {errors: [{
            path: 'email', message: 'user.email cannot be null',
        }]})
    })

    it('should throw on invalid email', async function() {
        const error = await assertThrows(async () => {
            await rpc.signedCall('conveyor.set_user_data', adminSigner, 'foo', {phone: '+12345567', email: 'foo'})
        })
        assert.deepEqual(error.data, {errors: [{
            path: 'email', message: 'Validation isEmail on email failed',
        }]})
    })

    it('should throw on invalid phone number', async function() {
        const error = await assertThrows(async () => {
            await rpc.signedCall('conveyor.set_user_data', adminSigner, 'foo', {phone: 'hello', email: 'foo@bar.com'})
        })
        assert.deepEqual(error.data, {errors: [{
            path: 'phone', message: 'Validation is on phone failed',
        }]})
    })

    it('should not create if validators fail', async function() {
        let error
        error = await assertThrows(async () => {
            await rpc.signedCall('conveyor.set_user_data', adminSigner, 'nooo', {})
        })
        assert.equal(error.message, 'no keys in data')
        error = await assertThrows(async () => {
            await rpc.signedCall('conveyor.get_user_data', adminSigner, 'nooo')
        })
        assert.equal(error.message, 'No such user')
    })

    it('should check if email exists', async function() {
        let rv
        rv = await rpc.signedCall('conveyor.is_email_registered', adminSigner, 'foo@bar.com')
        assert.equal(rv, true)
        rv = await rpc.signedCall('conveyor.is_email_registered', adminSigner, 'gaben@valvesoftware.com')
        assert.equal(rv, false)
    })

    it('should check if phone exists', async function() {
        let rv
        rv = await rpc.signedCall('conveyor.is_phone_registered', adminSigner, '+99123123123')
        assert.equal(rv, true)
        rv = await rpc.signedCall('conveyor.is_phone_registered', adminSigner, '+123456767899')
        assert.equal(rv, false)
    })

})
