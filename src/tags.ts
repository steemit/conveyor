/**
 * @file User tags.
 * @author Johan Nordberg <johan@steemit.com>
 */

import {JsonRpcAuthMethodContext as JCtx, JsonRpcError} from '@steemit/koa-jsonrpc'
import * as config from 'config'
import {Op} from 'sequelize'

import {Tag, UserTag} from './database'

const ADMIN_ACCOUNT = config.get('admin_role') as string

export async function defineTag(this: JCtx, name: string, description: string) {
    this.assert(this.account === ADMIN_ACCOUNT, 'Unauthorized')
    this.assert(typeof name === 'string', 'Invalid tag name')
    this.assert(typeof description === 'string' && description.length > 0, 'Invalid tag description')
    await Tag.create({name, description})
}

export async function listTags(this: JCtx) {
    this.assert(this.account === ADMIN_ACCOUNT, 'Unauthorized')
    return (await Tag.all({order: ['name']})).map(({name, description}) => ({name, description}))
}

export async function assignTag(this: JCtx, uid: string, tag: string) {
    this.assert(this.account === ADMIN_ACCOUNT, 'Unauthorized')
    this.assert(typeof uid === 'string', 'Invalid user uid')
    this.assert(typeof tag === 'string', 'Invalid tag')
    const existing = await UserTag.find({where: {uid, tag}})
    if (!existing) {
        try {
            await UserTag.create({uid, tag})
        } catch (error) {
            if (error.name === 'SequelizeForeignKeyConstraintError') {
                throw new JsonRpcError(420, 'No such tag')
            }
            throw error
        }
    }
}

export async function unassignTag(this: JCtx, uid: string, tag: string) {
    this.assert(this.account === ADMIN_ACCOUNT, 'Unauthorized')
    this.assert(typeof uid === 'string', 'Invalid user uid')
    this.assert(typeof tag === 'string', 'Invalid tag')
    await UserTag.destroy({where: {uid, tag}})
}

export async function getUsersByTags(this: JCtx, tags: string | string[]) {
    this.assert(this.account === ADMIN_ACCOUNT, 'Unauthorized')
    if (typeof tags === 'string') {
        tags = [tags]
    }
    this.assert(Array.isArray(tags) && tags.every((tag) => typeof tag === 'string'), 'Invalid tags')
    const tagmap = new Map<string, string[]>()
    const where = {[Op.or]: tags.map((tag) => ({tag}))}
    for (const {tag, uid} of await UserTag.all({where})) {
        if (!tagmap.has(uid)) { tagmap.set(uid, []) }
        tagmap.get(uid)!.push(tag)
    }
    const rv: string[] = []
    for (const [uid, taglist] of tagmap) {
        if (tags.every((tag) => taglist.includes(tag))) {
            rv.push(uid)
        }
    }
    return rv.sort()
}

export async function getTagsForUser(this: JCtx, uid: string) {
    this.assert(this.account === ADMIN_ACCOUNT, 'Unauthorized')
    this.assert(typeof uid === 'string', 'Invalid user uid')
    return (await UserTag.all({where: {uid}})).map((usertag) => usertag.tag).sort()
}
