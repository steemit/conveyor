import * as config from 'config'
import * as _ from 'lodash'
const interval = require('interval-promise')
const trieLib = require('trie-prefix-tree')
import {logger} from '../logger'
import {CachingClient} from './client'

const REFRESH_INTERVAL: number = config.get('accounts_refresh_interval')

export async function loadAccountNames(client: any, start: string = '', end?: string): Promise<Set<string>> {
    const limit = 1000
    const names = new Set()
    while (true) {
        logger.debug(`loading ${limit} account names start with "${start}"`)
        const results = await client.call('condenser_api', 'lookup_accounts', [
            start,
            limit
        ])
        for (const name of results) {
            names.add(name)
        }
        logger.debug(`added ${results.length} account names`)
        const lastResult: string  = results[results.length - 1]
        if (start === lastResult) {
            logger.debug(`${start} === ${lastResult} so loading complete`)
            break
        }
        if (results.length < limit) {
            logger.debug(`results.length of ${results.length} <= ${limit} so loading complete`)
            break
        }
        if (end !== undefined && lastResult.startsWith(end)) {
            logger.debug(`${lastResult}.startsWith(${end}) is true  <= so loading complete`)
            break
        }
        start = results[results.length - 1]
    }
    return names
}


/**
 * This method creates many async generators to parallelize the loading of
 * account names from steemd. The `starts` and `ends` variables store the
 * limits params supplied to each async generator, eg, one async generator
 * loads all account names from "b" to "c".
 * Without this otherwise, we would need to make count(accounts)/1000
 * (currently about 1100) api calls in series.
 */
export async function loadAllAccountNames(client: any) {
    const starts = [''].concat(Array.from('bcdefghijklmnopqrstuvwxyz'))
    const ends = starts.slice(1)
    const pairs = _.zip(starts, ends)
    const promises = _.map(pairs, (pair) => {
        const [start, end] = pair
        logger.debug(`adding promise loadAccountNames(client,${start},${end}`)
        return loadAccountNames(client, start, end)
    })
    const resultSets = await Promise.all(promises)
    const combined = new Set()
    concatSets(combined, ...resultSets)
    return combined
}

export function loadAccountsTrie(accounts: Set<string>) {
    return trieLib(Array.from(accounts))
}

export class AccountNameTrie {
    public readonly trie: any
    public stopRefreshLoop: boolean
    public readonly refreshInterval: number
    public timerId: any
    private client: CachingClient



    constructor(initialAccounts: Set<string>,
                client: CachingClient,
                refreshInterval: number = REFRESH_INTERVAL) {
        this.client = client
        this.refreshInterval = refreshInterval
        this.trie = loadAccountsTrie(initialAccounts)
        this.stopRefreshLoop = true
    }

    public stopRefreshing() {
        this.stopRefreshLoop = true
        clearTimeout(this.timerId)
        logger.debug('stopped refresh loop')
    }

    public startRefreshing() {
        this.stopRefreshLoop = false
        this.refresh()
    }

    public matchPrefix(prefix: string) {
        return this.trie.getPrefix(prefix, false)
    }

    public refresh() {
        if (this.stopRefreshLoop) {
            logger.info('refresh called while stopRefreshLoop === true, returning')
            return
        }
        logger.info(`scheduled refresh cycle in ${this.refreshInterval / 1000}s`)
        this.timerId = setInterval(() => {
            logger.info('starting refresh')
            this.updateTrie().then(() => {
                // schedule the next one unless it shouldn't
                if ( !this.stopRefreshLoop) {
                    logger.info('scheduling next refresh cycle')
                }
            }, (err) => {
                logger.error(`Failed to update trie: ${err}`)
            })
        }, this.refreshInterval)
    }

    public async updateTrie() {
        const limit = 1000
        let start = ''
        while (!this.stopRefreshLoop) {
            const results = await this.client.call('condenser_api', 'lookup_accounts', [
                start,
                limit
            ], null)
            for (const account of results) {
                this.trie.addWord(account)
            }
            logger.debug(`added ${results.length} account names`)
            const lastResult: string  = results[results.length - 1]
            if (start === lastResult) {
                logger.debug(`${start} === ${lastResult} so loading complete`)
                break
            }
            if (results.length < limit) {
                logger.debug(`results.length of ${results.length} <= ${limit} so loading complete`)
                break
            }
            start = results[results.length - 1]
        }
    }

}

function sleep(ms) {
    return new Promise((resolve) => {setTimeout(resolve, ms)})
}

function concatSets(set, ...iterables) {
    for (const iterable of iterables) {
        for (const item of iterable) {
            set.add(item)
        }
    }
}
