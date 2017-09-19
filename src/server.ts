/**
 * @file Overseer server.
 * @author Johan Nordberg <johan@steemit.com>
 */

import * as bunyan from 'bunyan'
import * as cluster from 'cluster'
import * as config from 'config'
import * as Koa from 'koa'
import * as os from 'os'

import {JsonRpc, requestLogger, rpcLogger} from '@steemit/jsonrpc'

const logger = bunyan.createLogger({
    name: config.get('name'),
    streams: (config.get('log') as any[]).map(({level, out}) => {
        if (out === 'stdout') {
            return {level, stream: process.stdout}
        } else if (out === 'stderr') {
            return {level, stream: process.stderr}
        } else {
            return {level, path: out}
        }
    })
})

export const app = new Koa()
const rpc = new JsonRpc(config.get('name'))

app.proxy = true
app.on('error', (error) => {
    logger.error(error, 'Application error')
})

app.use(requestLogger(logger))
app.use(rpcLogger(logger))
app.use(rpc.middleware)

rpc.register('hello', async function(name: string) {
    this.log.info('Hello %s', name)
    return `I'm sorry, ${ name }, I can't do that.`
})

function run() {
    const port = config.get('port')
    app.listen(port, () => {
        logger.info('running on port %d', port)
    })
}

if (module === require.main) {
    let numWorkers = config.get('num_workers')
    if (numWorkers === 0) {
        numWorkers = os.cpus().length
    }
    if (numWorkers > 1) {
        if (cluster.isMaster) {
            logger.info('spawning %d workers', numWorkers)
            for (let i = 0; i < numWorkers; i++) {
                cluster.fork()
            }
        } else {
            run()
        }
    } else {
        run()
    }
}
