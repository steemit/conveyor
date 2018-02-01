
import {JsonRpcAuthMethodContext as JCtx, JsonRpcError} from '@steemit/koa-jsonrpc'
import * as config from 'config'
import {createHash} from 'crypto'
import * as rjs from 'random-js'

import {store} from './store'

function sha256(input: string) {
    return createHash('sha256').update(input).digest()
}

const engine = rjs.engines.mt19937()
const random = rjs.real(0, 1)

function flagProbability(account: string, flag: string) {
    const hash = sha256(account + flag)
    const seeds: number[] = []
    for (let i = 0; i < 8; i++) {
        seeds.push(hash.readInt32LE(i * 4))
    }
    engine.seedWithArray(seeds)
    return random(engine)
}

const KEY_PREFIX = config.get('name')
const FLAG_PROBABILITY_KEY = `${ KEY_PREFIX }_feature-flags.json`
const FLAG_PATTERN = /^[a-z_]+$/
const ADMIN_ACCOUNT = config.get('admin_role')

async function readProbabilities(): Promise<{[name: string]: number}> {
    return await store.readJSON(FLAG_PROBABILITY_KEY) || {}
}

async function writeProbabilities(value: {[name: string]: number}) {
    return store.writeJSON(FLAG_PROBABILITY_KEY, value)
}

function flagKey(account: string) {
    return `${ KEY_PREFIX }_${ account }_feature-flags.json`
}

async function readFlags(account: string): Promise<{[name: string]: boolean}> {
    return await store.readJSON(flagKey(account)) || {}
}

async function writeFlags(account: string, value: {[name: string]: boolean}) {
    return store.writeJSON(flagKey(account), value)
}

function validateFlag(flag: string) {
    if (!FLAG_PATTERN.test(flag)) {
        throw new JsonRpcError(400, {info: {flag}}, 'Flags must be lowercase and contain only a-z and underscore')
    }
}

export async function setProbability(this: JCtx, flag: string, probability: number) {
    this.assert(this.account === ADMIN_ACCOUNT, 'Unauthorized')
    validateFlag(flag)
    if (typeof probability !== 'number') {
        probability = Number.parseFloat(probability)
    }
    this.assert(
        Number.isFinite(probability) &&
        probability >= 0 && probability <= 1,
        'Probability must be a fraction between 0 and 1'
    )
    this.log.info('set probability for feature flag %s to %d%', flag, ~~(probability * 100))
    const probabilities = await readProbabilities()
    if (probability === 0) {
        delete probabilities[flag]
    } else {
        probabilities[flag] = probability
    }
    await writeProbabilities(probabilities)
}

export async function getProbabilities(this: JCtx) {
    this.assert(this.account === ADMIN_ACCOUNT, 'Unauthorized')
    return readProbabilities()
}

export async function setFlag(this: JCtx, account: string, flag: string, value: boolean | null) {
    this.assert(this.account === ADMIN_ACCOUNT, 'Unauthorized')
    validateFlag(flag)
    this.assert(typeof value === 'boolean' || value === null, 'Value must be boolean or null')
    const flags = await readFlags(account)
    if (value === null) {
        this.log.info('deleting flag %s from user %s', flag, account)
        delete flags[flag]
    } else {
        this.log.info('setting flag %s to %d on user %s', flag, value, account)
        flags[flag] = value
    }
    await writeFlags(account, flags)
}

export async function getFlag(this: JCtx, account: string, flag: string) {
    this.assert(this.account === account || this.account === ADMIN_ACCOUNT, 'Unauthorized')
    validateFlag(flag)
    const flags = await readFlags(account)
    if (flags[flag]) {
        return flags[flag]
    }
    const probabilities = await readProbabilities()
    if (probabilities[flag]) {
        return (flagProbability(account, flag) < probabilities[flag])
    }
    return false
}

export async function getFlags(this: JCtx, account: string) {
    this.assert(this.account === account || this.account === ADMIN_ACCOUNT, 'Unauthorized')
    const rv = await readFlags(account)
    const probabilities = await readProbabilities()
    for (const flag of Object.keys(probabilities)) {
        if (rv[flag] === undefined) {
            rv[flag] = (flagProbability(account, flag) < probabilities[flag])
        }
    }
    return rv
}
