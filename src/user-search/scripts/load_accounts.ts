import {logger} from '../../logger'
import {CachingClient} from '../client'
import {loadAllAccountNames} from '../indexes'

export function loadThenWriteAccountNames() {
    const client = new CachingClient()
    const namesPromise = loadAllAccountNames(client)
    logger.info(`loading account names`)
    namesPromise.catch((err) => {
        logger.error(err)
        throw err
    })
    namesPromise.then((res) => {
        const namesArray = JSON.stringify(Array.from(res))
        const data = `export const userAccountNames: Set<string> = new Set(${namesArray})\n`
        process.stdout.write(data)
        logger.info(`completed loading account names`)
    })
}

loadThenWriteAccountNames()
