import * as _ from 'lodash'
import { Trie } from 'trie-prefix-tree2'
import {logger} from '../logger'

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

function concatSets(set, ...iterables) {
    for (const iterable of iterables) {
        for (const item of iterable) {
            set.add(item)
        }
    }
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

export function loadAccountsTrie(accounts: Set<string>): Trie {
    return new Trie(Array.from(accounts))
}

export async function buildAccountsTrie(client: any): Promise<Trie> {
    const names = await loadAccountNames(client)
    return new Trie(Array.from(names))
}

export function matchPrefix(trie: Trie, prefix: string): string[] {
    return trie.getPrefix(prefix, false)
}

export function intersectMatches(trie: Trie, prefix: string, otherSet: Set<string>) {
    const matches = trie.getPrefix(prefix, false)
    return new Set(matches.filter((x) => otherSet.has(x)))
}
