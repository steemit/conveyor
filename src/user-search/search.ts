import {JsonRpcAuthMethodContext as JCtx} from '@steemit/koa-jsonrpc'
import * as config from 'config'
import * as Koa from 'koa'
import {users as gdprList } from '../../user-data/lists/gdpr/users'
import {Context} from '../server'
import {matchPrefix} from './indexes'
import {logger} from '../logger'

const ADMIN_ACCOUNT = config.get('admin_role')

interface JCtx2 extends JCtx {
    ctx: Context
}

export async function getAccount(this: JCtx2,
                                 account: string,
                                 contextAccount: string) {
    this.assert(this.account === ADMIN_ACCOUNT, 'Unauthorized')
    const client: any = this.ctx.cacheClient
    if (gdprList.has(account)) {
        return []
    }
    logger.info(`getAccount account:${account} contextAccount:${contextAccount}`)
    return await client.loadAccountJSON(account, contextAccount)
}

export async function autocompleteAccount(this: JCtx2,
                                          account: string,
                                          accountSubstring: string
                                         ) {
    this.assert(this.account === ADMIN_ACCOUNT, 'Unauthorized')
    const client: any = this.ctx.cacheClient
    const userAccount = client.loadAccount(account)
    const globalAccountNames = matchPrefix(this.ctx.userAccountTrie, accountSubstring)
    const friendAccountNames = new Set(globalAccountNames.filter((x) => userAccount.following.has(x)))
    const recentAccountNames = new Set(globalAccountNames.filter((x) => userAccount.recentSendAccounts().has(x)))
    return {
        global: globalAccountNames.length < 10 ? await client.loadAccountsJSON(globalAccountNames) : [],
        friends: await client.loadAccountsJSON(friendAccountNames),
        recent: await client.loadAccountsJSON(recentAccountNames)
    }
}
