import * as assert from 'assert'
import {utils} from '@steemit/koa-jsonrpc'

export async function assertThrows(block: () => Promise<any>) {
    try {
        await block()
    } catch (error) {
        return error
    }
    assert.fail('Missing expected exception')
}

export class RPCError extends Error {
    public code: number
    public data: any
    constructor (error: any) {
        super(error.message)
        this.code = error.code
        this.data = error.data
    }
}

export function makeClient(port, startId = 0) {
    let id = startId
    return async (method: string, ...params) => {
        const rv = await utils.jsonRequest(
            {port, protocol: 'http:', method: 'post'},
            {id: ++id, jsonrpc: '2.0', method, params}
        )
        if (rv.error) {
            throw new RPCError(rv.error)
        }
        return rv.result
    }
}
