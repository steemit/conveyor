import {users as  badActorsList} from '../../user-data/lists/bad_actors/users'
import {users as exchangesList } from '../../user-data/lists/exchanges/users'
import {users as gdprList } from '../../user-data/lists/gdpr/users'
// import * as verifiedList from '../../user-data/lists/verified/users' FIXME

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