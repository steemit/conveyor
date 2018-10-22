import {JsonRpcAuthMethodContext as JCtx} from '@steemit/koa-jsonrpc'

import {users as gdprList } from '../../user-data/lists/gdpr/users'
import {logger} from '../logger'
import {Context} from '../server'
import {matchPrefix} from './indexes'

interface JCtx2 extends JCtx {
    ctx: Context
}

export async function getAccount(this: JCtx2,
                                 account: string,
                                 contextAccount: string) {
    const client: any = this.ctx.cacheClient
    if (gdprList.has(account)) {
        return []
    }
    logger.info(`getAccount account:${account} contextAccount:${contextAccount}`)
    const userAccountJSON = await client.loadAccountJSON(account, contextAccount)
    logger.info(`userAccountJSON: ${JSON.stringify(userAccountJSON)}`)
    return userAccountJSON
}

export async function autocompleteAccount(this: JCtx2,
                                          account: string,
                                          accountSubstring: string
                                         ) {
    const client: any = this.ctx.cacheClient
    const userAccount = client.loadAccount(account)
    const globalAccountNames = matchPrefix(this.ctx.userAccountTrie, accountSubstring)
    const friendAccountNames = new Set(globalAccountNames.filter((x) => userAccount.following.has(x)))
    const recentAccountNames = new Set(globalAccountNames.filter((x) => userAccount.recentSendAccounts().has(x)))
    return JSON.stringify({
        global: globalAccountNames.length < 10 ? await client.loadAccountsJSON(globalAccountNames) : [],
        friends: await client.loadAccountsJSON(friendAccountNames),
        recent: await client.loadAccountsJSON(recentAccountNames)
    })
}
