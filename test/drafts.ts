import 'mocha'
import * as assert from 'assert'
import * as http from 'http'
import * as UUID from 'uuid/v4'
import {utils} from '@steemit/jsonrpc'

import {makeClient} from './common'

import {app} from './../src/server'

describe('drafts', function() {
    const port = process.env['TEST_HTTP_PORT'] ? parseInt(process.env['TEST_HTTP_PORT'] as string) : 63205
    const server = http.createServer(app.callback())

    before((done) => { server.listen(port, 'localhost', done) })
    after((done) => { server.close(done) })

    const call = makeClient(port)
    const uuid = UUID()

    it('should create draft', async function() {
        const rv = await call('conveyor.save_draft', 'foo', {title: 'foo', body: 'bar'})
        assert.equal(typeof rv.uuid, 'string')
    })

    it('should create draft with uuid', async function() {
        const rv = await call('conveyor.save_draft', 'foo', {uuid, title: 'foo', body: 'bar'})
        assert.equal(rv.uuid, uuid)
    })

    it('should update draft', async function() {
        const rv = await call('conveyor.save_draft', 'foo', {uuid, title: 'foo2', body: 'bar2'})
        assert.equal(rv.uuid, uuid)
    })

    it('should delete draft', async function() {
        const rv1 = await call('conveyor.save_draft', 'foo', {title: 'foo', body: 'bar'})
        const rv2 = await call('conveyor.remove_draft', 'foo', rv1.uuid)
        assert.equal(rv2.uuid, rv1.uuid)
    })

    it('should list drafts', async function() {
        const rv = await call('conveyor.list_drafts', 'foo')
        assert.equal(rv.length, 2)
        assert.deepEqual(rv[1], {uuid, title: 'foo2', body: 'bar2'})
    })

})
