/**
 * @file Drafts API.
 * @author Johan Nordberg <johan@steemit.com>
 */

import * as config from 'config'
import * as UUID from 'uuid/v4'

import {JsonRpcAuthMethodContext as JCtx, JsonRpcError} from '@steemit/koa-jsonrpc'
import {store} from './store'

const KEY_PREFIX = config.get('name')
function draftsKey(account: string) {
    return `${ KEY_PREFIX }_${ account }_drafts.json`
}

async function readDrafts(account: string): Promise<any[]> {
    return await store.readJSON(draftsKey(account)) || []
}

async function writeDrafts(account: string, drafts: any) {
    await store.writeJSON(draftsKey(account), drafts)
}

export async function list(this: JCtx, account: string) {
    this.assert(this.account === account, 'Unauthorized')
    this.log.info({account}, 'List drafts')
    return readDrafts(account)
}

export async function save(this: JCtx, account: string, draft: any) {
    this.assert(this.account === account, 'Unauthorized')
    if (!draft.uuid) {
        draft.uuid = UUID()
    }
    const uuid = draft.uuid
    this.log.info({account, uuid}, 'Save draft')
    // TODO: Validate draft object
    const drafts = await readDrafts(account)
    const existing = drafts.find((item) => item.uuid === uuid)
    if (existing) {
        drafts.splice(drafts.indexOf(existing), 1, draft)
    } else {
        drafts.push(draft)
    }
    await writeDrafts(account, drafts)
    return {uuid}
}

export async function remove(this: JCtx, account: string, uuid: string) {
    this.assert(this.account === account, 'Unauthorized')
    this.log.info({account, uuid}, 'Remove draft')
    const drafts = await readDrafts(account)
    const existing = drafts.find((item) => item.uuid === uuid)
    if (!existing) {
        throw new JsonRpcError(100, {info: {uuid}}, 'Draft not found')
    }
    drafts.splice(drafts.indexOf(existing), 1)
    await writeDrafts(account, drafts)
    return {uuid}
}
