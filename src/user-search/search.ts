import {JsonRpcAuthMethodContext as JCtx} from '@steemit/koa-jsonrpc'
import * as config from 'config'
import {Trie} from 'trie-prefix-tree2'
import {users as gdprList } from '../../lists/gdpr/users'
import {logger} from '../logger'
import {CachingClient} from './client'
import {matchPrefix} from './indexes'

const ADMIN_ACCOUNT = config.get('admin_role')

export async function getAccount(this: JCtx,
                                 client: any,
                                 account: string,
                                 context_account: string) {
    // this.assert(this.account === ADMIN_ACCOUNT, 'Unauthorized') FIXME
    if (gdprList.has(account)) {
        return []
    }
    return await client.loadAccount(account, context_account)
}

export async function autocompleteAccount(this: JCtx,
                                          client: any,
                                          account: string,
                                          account_substring: string
                                         ) {
    this.assert(this.account === ADMIN_ACCOUNT, 'Unauthorized')
    const userAccount = client.loadAccount(account)
    const trie = new Trie([])
    const globalAccountNames = matchPrefix(trie, account_substring)
    const friendAccountNames = new Set(globalAccountNames.filter((x) => userAccount.following.has(x)))
    const recentAccountNames = new Set(globalAccountNames.filter((x) => userAccount.recentSendAccounts().has(x)))
    return {
        global: globalAccountNames.length < 10 ? globalAccountNames : [],
        friends: await client.loadAccounts(friendAccountNames),
        recent: await client.loadAccounts(recentAccountNames)
    }
}
