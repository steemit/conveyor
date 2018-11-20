export const badActorsList = require('../../user-data/lists/bad_actors/users')
export const exchangesList = require('../../user-data/lists/exchanges/users')
export const gdprList = require('../../user-data/lists/gdpr/users')

enum UserAccountTags {
    abuse,
    exchange,
    verified,
    none
}

const verified: Set<string> = new Set() // FIXME

export function getUserTags(account: string): string[] {
    if (exchangesList.has(account)) {
        return ['exchange']
    }
    if (badActorsList.has(account)) {
        return ['abuse']
    }
    if (verified.has(account)) {
        return ['verified']
    }
    return ['none']
}
