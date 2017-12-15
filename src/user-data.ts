/**
 * @file User data storage.
 * @author Johan Nordberg <johan@steemit.com>
 */

import {JsonRpcAuthMethodContext as JCtx, JsonRpcError} from '@steemit/koa-jsonrpc'
import * as config from 'config'
import {ValidationError} from 'sequelize'

import {User} from './database'

const ADMIN_ACCOUNT = config.get('admin_role')

interface UserData {
    email: string
    phone: string
}

export async function getUserData(this: JCtx, account: string) {
    this.assert(this.account === account || this.account === ADMIN_ACCOUNT, 'Unauthorized')
    const user: any = await User.findOne({where: {account}})
    if (!user) {
        throw new JsonRpcError(404, 'No such user')
    }
    return {email: user.email, phone: user.phone}
}

export async function setUserData(this: JCtx, account: string, data: UserData) {
    this.assert(this.account === account || this.account === ADMIN_ACCOUNT, 'Unauthorized')
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
    this.assert(this.account === ADMIN_ACCOUNT, 'Unauthorized')
    const user = await User.findOne({where: {email}})
    return user != undefined
}

export async function isPhoneRegistered(this: JCtx, phone: string) {
    this.assert(this.account === ADMIN_ACCOUNT, 'Unauthorized')
    const user = await User.findOne({where: {phone}})
    return user != undefined
}
