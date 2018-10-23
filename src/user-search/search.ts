import {JsonRpcMethodContext as JCtx} from '@steemit/koa-jsonrpc'

import {logger} from '../logger'
import {Context} from '../server'
import {matchPrefix} from './indexes'
import {gdprList} from './lists'
import {UserAccountJSON} from './user'

interface ExtendedJCtx extends JCtx {
    ctx: Context
}

interface AutoCompleteResponse {
    global: UserAccountJSON[]
    friends: UserAccountJSON[]
    recent: UserAccountJSON[]
}

export async function getAccount(this: any,
                                 account: string,
                                 contextAccount?: string) {
    const client: any = this.ctx.cacheClient
    if (gdprList.has(account)) {
        return []
    }
    return await client.loadAccountJSON(account, contextAccount)
}

export async function autocompleteAccount(this: any,
                                          accountSubstring: string,
                                          account: string
                                         ) {
    const client: any = this.ctx.cacheClient
    const [userAccount, userContext] = await client.loadAccount(account)
    const recentSendAccounts = userAccount.recentSendAccounts()
    // logger.debug(`recentSendAccounts.size:${recentSendAccounts.size}`)
    const globalAccountNames = matchPrefix(this.ctx.userAccountTrie, accountSubstring)
    // logger.debug(`globalAccountNames.length:${globalAccountNames.length}`)
    const friendAccountNames = new Set(globalAccountNames.filter((x) => userAccount.isFollowing(x)))
    // logger.debug(`friendAccountNames.size:${friendAccountNames.size}`)
    const recentAccountNames = new Set(globalAccountNames.filter((x) => userAccount.recentSendAccounts().has(x)))
    // logger.debug(`recentAccountNames.size:${recentAccountNames.size}`)
    const response: AutoCompleteResponse = {
            global: [],
            friends: [],
            recent: []
    }
    if (globalAccountNames.length === 0) {
        return response
    }
    const accountsToLoad: Set<string> = new Set()
    if (globalAccountNames.length < 10) {
        globalAccountNames.forEach(function(this, item) { accountsToLoad.add(item)})
        // logger.debug(`globalAccountNames -> accountsToLoad.size: ${accountsToLoad.size}`)
    }
    if (friendAccountNames.size < 10) {
        friendAccountNames.forEach(function(this, item) { accountsToLoad.add(item)})
        // logger.debug(`friendAccountNames -> accountsToLoad.size: ${accountsToLoad.size}`)
    }
    if (recentAccountNames.size < 10) {
        recentAccountNames.forEach(function(this, item) { accountsToLoad.add(item)})
        // logger.debug(`recentAccountNames -> accountsToLoad.size: ${accountsToLoad.size}`)
    }
    // logger.debug(`accountsToLoad.size: ${accountsToLoad.size}`)
    const loadedAccounts = await client.loadAccountsJSON(accountsToLoad, account)
    for (const userAcct of loadedAccounts) {
        // logger.debug(`userAcct.account:${userAcct.account}`)
        if (globalAccountNames.includes(userAcct.account) ) {
            response.global.push(userAcct)
        }
        if (friendAccountNames.has(userAcct.account)) {
            response.friends.push(userAcct)
        }
        if (recentAccountNames.has(userAcct.account)) {
            response.recent.push(userAcct)
        }
    }
    return response
}
