import 'mocha'
import * as assert from 'assert'
import * as http from 'http'
import {utils} from '@steemit/jsonrpc'

import {app} from './../src/server'

describe('server', function() {
    const port = process.env['TEST_HTTP_PORT'] ? parseInt(process.env['TEST_HTTP_PORT'] as string) : 63205
    const server = http.createServer(app.callback())

    before((done) => { server.listen(port, 'localhost', done) })
    after((done) => { server.close(done) })

    it('should work', async function() {
        const rv = await utils.jsonRequest(
            {port, protocol: 'http:', method: 'post'},
            {id: 1, jsonrpc: '2.0', method: 'hello', params: {name: 'Dave'}}
        )
        assert.deepEqual(rv, {jsonrpc: '2.0', id: 1, result: "I'm sorry, Dave, I can't do that."})
    })

})
