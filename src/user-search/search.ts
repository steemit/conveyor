import { JsonRpcMethodContext as JCtx } from '@steemit/koa-jsonrpc'

import { Context } from '../server'
import { AccountNameTrie } from './indexes'
import { gdprList } from './lists'
import { UserAccountJSON } from './user'

interface ExtendedJCtx extends JCtx {
    ctx: Context
}

interface AutoCompleteResponse {
    global: UserAccountJSON[]
    friends: UserAccountJSON[]
    recent: UserAccountJSON[]
}

export async function getAccount(
    this: any,
    account: string,
    contextAccount?: string
) {
    const client: any = this.ctx.cacheClient
    if (gdprList.has(account)) {
        return []
    }
    return await client.loadAccountJSON(account, contextAccount)
}

export async function autocompleteAccount(
    this: any,
    accountSubstring: string,
    account: string
) {
    const client: any = this.ctx.cacheClient
    const trie: AccountNameTrie = this.ctx.userAccountTrie
    const [userAccount, userContext] = await client.loadAccount(account)
    let globalAccountNames: string[] = []
    if (accountSubstring.length > 3) {
        globalAccountNames = this.ctx.userAccountTrie.matchPrefix(
            accountSubstring
        )
    }
    const friendAccountNames = new Set(
        Array.from(userAccount.following).filter((x: string) =>
            x.startsWith(accountSubstring)
        )
    )
    const recentAccountNames = new Set(
        Array.from(userAccount.recentSendAccounts()).filter((x: string) =>
            x.startsWith(accountSubstring)
        )
    )

    const response: AutoCompleteResponse = {
        global: [],
        friends: [],
        recent: []
    }

    const accountsToLoad: Set<string> = new Set()
    if (globalAccountNames.length < 11) {
        globalAccountNames.forEach(function(this, item: string) {
            accountsToLoad.add(item)
        })
    }
    if (friendAccountNames.size < 11) {
        friendAccountNames.forEach(function(this, item: string) {
            accountsToLoad.add(item)
        })
    }
    if (recentAccountNames.size < 11) {
        recentAccountNames.forEach(function(this, item: string) {
            accountsToLoad.add(item)
        })
    }

    const loadedAccounts = await client.loadAccountsJSON(
        accountsToLoad,
        account
    )
    for (const userAcct of loadedAccounts) {
        if (recentAccountNames.has(userAcct.account)) {
            response.recent.push(userAcct)
        } else if (friendAccountNames.has(userAcct.account)) {
            response.friends.push(userAcct)
        } else {
            response.global.push(userAcct)
        }
    }
    return response
}
