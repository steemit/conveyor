/**
 * @file Drafts API.
 * @author Johan Nordberg <johan@steemit.com>
 */

import * as config from 'config'
import * as UUID from 'uuid/v4'

import {JsonRpcError, JsonRpcMethodContext} from '@steemit/jsonrpc'
import {logger} from './logger'
import {store} from './store'

const KEY_PREFIX = config.get('name')
function draftsKey(username: string) {
    return `${ KEY_PREFIX }_${ username }_drafts.json`
}

async function readDrafts(username: string): Promise<any[]> {
    const data = await store.safeRead(draftsKey(username))
    if (data) {
        return JSON.parse(data.toString('utf8'))
    }
    return []
}

async function writeDrafts(username: string, drafts: any) {
    await store.write(draftsKey(username), JSON.stringify(drafts))
}

export async function list(this: JsonRpcMethodContext, username: string) {
    this.log.info({username}, 'List drafts')
    return readDrafts(username)
}

export async function save(this: JsonRpcMethodContext, username: string, draft: any) {
    if (!draft.uuid) {
        draft.uuid = UUID()
    }
    const uuid = draft.uuid
    this.log.info({username, uuid}, 'Save draft')
    // TODO: Validate draft object
    const drafts = await readDrafts(username)
    const existing = drafts.find((item) => item.uuid === uuid)
    if (existing) {
        drafts.splice(drafts.indexOf(existing), 1, draft)
    } else {
        drafts.push(draft)
    }
    await writeDrafts(username, drafts)
    return {uuid}
}

export async function remove(this: JsonRpcMethodContext, username: string, uuid: string) {
    this.log.info({username, uuid}, 'Remove draft')
    const drafts = await readDrafts(username)
    const existing = drafts.find((item) => item.uuid === uuid)
    if (!existing) {
        throw new JsonRpcError(100, {info: {uuid}}, 'Draft not found')
    }
    drafts.splice(drafts.indexOf(existing), 1)
    await writeDrafts(username, drafts)
    return {uuid}
}
