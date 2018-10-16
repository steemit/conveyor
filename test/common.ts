import * as assert from 'assert'
import {utils} from '@steemit/koa-jsonrpc'
import {sign as signRequest} from '@steemit/rpc-auth'

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

export interface RPCSigner {
    account: string
    key: string
}

const SEQNO_STEP_SIZE = 1

export class RPC {
    private requestOpts: any
    private seqNo = 0
    constructor(port: number) {
        this.requestOpts = {port, protocol: 'http:', method: 'post'}
    }
    public async call(method: string, ...params) {
        const response = await this.send(this.buildRequest(method, ...params))
        return this.resolveResponse(response)
    }
    public async signedCall(method: string, signer: RPCSigner, ...params) {
        const request = signRequest(this.buildRequest(method, ...params), signer.account, [signer.key])
        const response = await this.send(request)
        return this.resolveResponse(response)
    }
    private async send(data: any) {
        return await utils.jsonRequest(this.requestOpts, data)
    }
    private buildRequest(method: string, ...params): any {
        const currentSeqNo = this.seqNo
        const nextSeqNo = currentSeqNo + SEQNO_STEP_SIZE
        const request = {id: nextSeqNo, jsonrpc: '2.0', method, params}
        this.seqNo = nextSeqNo
        return request
    }
    private resolveResponse(response) {
        if (response.error) {
            throw new RPCError(response.error)
        }
        return response.result
    }
}
