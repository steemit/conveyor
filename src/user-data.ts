/**
 * @file User data storage.
 * @author Johan Nordberg <johan@steemit.com>
 */

import {JsonRpcError, JsonRpcMethodContext as JCtx} from '@steemit/koa-jsonrpc'
import {ValidationError} from 'sequelize'

import {User} from './database'

interface UserData {
    email: string
    phone: string
}

export async function getUserData(this: JCtx, account: string) {
    // TODO: Add auth, account or admin role
    const user: any = await User.findOne({where: {account}})
    if (!user) {
        throw new JsonRpcError(404, 'No such user')
    }
    return {email: user.email, phone: user.phone}
}

export async function setUserData(this: JCtx, account: string, data: UserData) {
    // TODO: Add auth, account or admin role
    this.assert(typeof data === 'object', 'data must be object')
    this.assert(Object.keys(data).length > 0, 'no keys in data')
    const {phone, email} = data
    const update = {account, phone, email}
    try {
        await User.upsert(update)
        this.log.info('updated user data for %s', account)
    } catch (error) {
        if (error instanceof ValidationError) {
            const errors = (error as ValidationError).errors.map((error) => {
                const {message, path} = error
                return {message, path}
            })
            throw new JsonRpcError(400, {info: {errors}}, error.message)
        } else {
            throw error
        }
    }
}

export async function isEmailRegistered(this: JCtx, email: string) {
    // TODO: Add auth, admin role
    const user = await User.findOne({where: {email}})
    return user != undefined
}

export async function isPhoneRegistered(this: JCtx, phone: string) {
    // TODO: Add auth, admin role
    const user = await User.findOne({where: {phone}})
    return user != undefined
}
