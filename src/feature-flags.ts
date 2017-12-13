
import {JsonRpcError, JsonRpcMethodContext as JCtx} from '@steemit/koa-jsonrpc'
import * as config from 'config'
import {createHash} from 'crypto'
import * as rjs from 'random-js'

import {store} from './store'

function sha256(input: string) {
    return createHash('sha256').update(input).digest()
}

const engine = rjs.engines.mt19937()
const random = rjs.real(0, 1)

function flagProbability(username: string, flag: string) {
    const hash = sha256(username + flag)
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

async function readProbabilities(): Promise<{[name: string]: number}> {
    return await store.readJSON(FLAG_PROBABILITY_KEY) || {}
}

async function writeProbabilities(value: {[name: string]: number}) {
    return store.writeJSON(FLAG_PROBABILITY_KEY, value)
}

function flagKey(username: string) {
    return `${ KEY_PREFIX }_${ username }_feature-flags.json`
}

async function readFlags(username: string): Promise<{[name: string]: boolean}> {
    return await store.readJSON(flagKey(username)) || {}
}

async function writeFlags(username: string, value: {[name: string]: boolean}) {
    return store.writeJSON(flagKey(username), value)
}

function validateFlag(flag: string) {
    if (!FLAG_PATTERN.test(flag)) {
        throw new JsonRpcError(400, {info: {flag}}, 'Flags must be lowercase and contain only a-z and underscore')
    }
}

export async function setProbability(this: JCtx, flag: string, probability: number) {
    // TODO: this should require authentication
    validateFlag(flag)
    this.assert(probability >= 0 && probability <= 1, 'Probability must be a fraction between 0 and 1')
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
    // TODO: this should require authentication
    return readProbabilities()
}

export async function setFlag(this: JCtx, username: string, flag: string, value: boolean | null) {
    // TODO: this should require authentication
    validateFlag(flag)
    this.assert(typeof value === 'boolean' || value === null, 'Value must be boolean or null')
    const flags = await readFlags(username)
    if (value === null) {
        this.log.info('deleting flag %s from user %s', flag, username)
        delete flags[flag]
    } else {
        this.log.info('setting flag %s to %d on user %s', flag, value, username)
        flags[flag] = value
    }
    await writeFlags(username, flags)
}

export async function getFlag(this: JCtx, username: string, flag: string) {
    validateFlag(flag)
    const flags = await readFlags(username)
    if (flags[flag]) {
        return flags[flag]
    }
    const probabilities = await readProbabilities()
    if (probabilities[flag]) {
        return (flagProbability(username, flag) < probabilities[flag])
    }
    return false
}

export async function getFlags(this: JCtx, username: string) {
    const rv = await readFlags(username)
    const probabilities = await readProbabilities()
    for (const flag of Object.keys(probabilities)) {
        if (rv[flag] === undefined) {
            rv[flag] = (flagProbability(username, flag) < probabilities[flag])
        }
    }
    return rv
}
