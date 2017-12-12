import 'mocha'
import * as assert from 'assert'
import * as http from 'http'
import {utils} from '@steemit/jsonrpc'

import {makeClient, assertThrows} from './common'

import {app} from './../src/server'
import {db} from './../src/database'

describe('user data', function() {
    const port = process.env['TEST_HTTP_PORT'] ? parseInt(process.env['TEST_HTTP_PORT'] as string) : 63205
    const server = http.createServer(app.callback())

    before(async () => { await db.sync() })
    before((done) => { server.listen(port, 'localhost', done) })
    after((done) => { server.close(done) })

    const call = makeClient(port)

    it('should set user data', async function() {
        await call('conveyor.set_user_data', 'foo', {email: 'foo@bar.com', phone: '+99123123123'})
    })

    it('should get user data', async function() {
        const rv = await call('conveyor.get_user_data', 'foo')
        assert.deepEqual(rv, {email: 'foo@bar.com', phone: '+99123123123'})
    })

    it('should throw on missing data', async function() {
        const error = await assertThrows(async () => {
            await call('conveyor.set_user_data', 'onlyphone', {phone: '+12345567'})
        })
        assert.deepEqual(error.data, {errors: [{
            path: 'email', message: 'user.email cannot be null',
        }]})
    })

    it('should throw on invalid email', async function() {
        const error = await assertThrows(async () => {
            await call('conveyor.set_user_data', 'bad', {phone: '+12345567', email: 'foo'})
        })
        assert.deepEqual(error.data, {errors: [{
            path: 'email', message: 'Validation isEmail on email failed',
        }]})
    })

    it('should throw on invalid phone number', async function() {
        const error = await assertThrows(async () => {
            await call('conveyor.set_user_data', 'bad', {phone: 'hello', email: 'foo@bar.com'})
        })
        assert.deepEqual(error.data, {errors: [{
            path: 'phone', message: 'Validation is on phone failed',
        }]})
    })

    it('should not create if validators fail', async function() {
        await assertThrows(async () => {
            await call('conveyor.set_user_data', 'nooo', {})
        })
        await assertThrows(async () => {
            await call('conveyor.get_user_data', 'nooo')
        })
    })

    it('should check if email exists', async function() {
        let rv
        rv = await call('conveyor.is_email_registered', 'foo@bar.com')
        assert.equal(rv, true)
        rv = await call('conveyor.is_email_registered', 'gaben@valvesoftware.com')
        assert.equal(rv, false)
    })

    it('should check if phone exists', async function() {
        let rv
        rv = await call('conveyor.is_phone_registered', '+99123123123')
        assert.equal(rv, true)
        rv = await call('conveyor.is_phone_registered', '+123456767899')
        assert.equal(rv, false)
    })

})
