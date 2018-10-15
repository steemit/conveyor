import { Client, ExtendedAccount } from 'dsteem'
import { Trie } from 'trie-prefix-tree2'

export async function loadAccountNames(client: any): Promise<Set<string>> {
    let lowerBoundName = ''
    const limit = 1000
    const names = new Set()
    while (true) {
        const results = await client.call('condenser_api', 'lookup_accounts', [
            lowerBoundName,
            limit
        ])
        for (const name of results) {
            names.add(name)
        }
        if (lowerBoundName === results[results.length - 1]) {
            break
        }
        lowerBoundName = results[results.length - 1]
    }
    return names
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