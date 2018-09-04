import {JsonRpcAuthMethodContext as JCtx, JsonRpcError} from '@steemit/koa-jsonrpc'
import * as config from 'config'
import {ValidationError} from 'sequelize'

import {User} from './database'

const ADMIN_ACCOUNT = config.get('admin_role')

export async function getAccount(this: JCtx, account: string) {
    this.assert(this.account === ADMIN_ACCOUNT, 'Unauthorized')
    const user = await User.findOne({where: {account}})
    return user
}
