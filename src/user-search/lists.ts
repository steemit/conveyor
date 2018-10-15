import {users as  badActorsList} from '../../lists/bad_actors/users'
import {users as exchangesList } from '../../lists/exchanges/users'
import {users as gdprList } from '../../lists/gdpr/users'
// import * as verifiedList from '../../lists/verified/users' FIXME

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