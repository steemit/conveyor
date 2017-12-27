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

    it('should allow missing data', async function() {
        await rpc.signedCall('conveyor.set_user_data', adminSigner, 'missing', {phone: null})
        await rpc.signedCall('conveyor.set_user_data', adminSigner, 'missing', {phone: null, email: null})
        await rpc.signedCall('conveyor.set_user_data', adminSigner, 'missing', {email: null})
        const rv = await rpc.signedCall('conveyor.get_user_data', adminSigner, 'missing')
        assert.deepEqual(rv, {email: null, phone: null})
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

    it('should enforce email uniqueness', async function() {
        await rpc.signedCall('conveyor.set_user_data', adminSigner, 'user1', {email: 'user1@mail.com', phone: '+99911112'})
        const error = await assertThrows(async () => {
            await rpc.signedCall('conveyor.set_user_data', adminSigner, 'user2', {email: 'user1@mail.com', phone: '+44223121'})
        })
        assert.deepEqual(error.data, {errors: [ { message: 'email must be unique', path: 'email' } ]})
    })

    it('should enforce phone uniqueness', async function() {
        await rpc.signedCall('conveyor.set_user_data', adminSigner, 'userphone1', {email: 'userphone1@mail.com', phone: '+42424242'})
        const error = await assertThrows(async () => {
            await rpc.signedCall('conveyor.set_user_data', adminSigner, 'userphone2', {email: 'userphone2@mail.com', phone: '+42424242'})
        })
        assert.deepEqual(error.data, {errors: [ { message: 'phone must be unique', path: 'phone' } ]})
    })

    it('should update user data', async function() {
        let rv
        await rpc.signedCall('conveyor.set_user_data', adminSigner, 'updateguy', {email: 'updateguy@hey.no', phone: '+1230002'})
        rv = await rpc.signedCall('conveyor.is_email_registered', adminSigner, 'updateguy@hey.no')
        assert.equal(rv, true, 'email registered')
        rv = await rpc.signedCall('conveyor.get_user_data', adminSigner, 'updateguy')
        assert.deepEqual(rv, {email: 'updateguy@hey.no', phone: '+1230002'}, 'data set')
        await rpc.signedCall('conveyor.set_user_data', adminSigner, 'updateguy', {email: 'updateguy@your.up', phone: '+1230002'})
        rv = await rpc.signedCall('conveyor.get_user_data', adminSigner, 'updateguy')
        assert.deepEqual(rv, {email: 'updateguy@your.up', phone: '+1230002'}, 'data updated')
        rv = await rpc.signedCall('conveyor.is_email_registered', adminSigner, 'updateguy@hey.no')
        assert.equal(rv, false, 'prev email not registered')
        rv = await rpc.signedCall('conveyor.is_email_registered', adminSigner, 'updateguy@your.up')
        assert.equal(rv, true, 'new email registered')
        await assertThrows(async () => {
            await rpc.signedCall('conveyor.set_user_data', adminSigner, 'updateguy', {email: 'foo@bar.com', phone: '+1230002'})
        })
    })

})
