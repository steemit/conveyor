/**
 * @file Steemitapi server.
 * @author Johan Nordberg <johan@steemit.com>
 */

import * as bunyan from 'bunyan'
import * as cluster from 'cluster'
import * as config from 'config'
import * as Koa from 'koa'
import * as Router from 'koa-router'
import * as os from 'os'

import * as drafts from './drafts'
import * as featureFlags from './feature-flags'

import {JsonRpc, requestLogger, rpcLogger} from '@steemit/jsonrpc'
import {logger} from './logger'

export const version = require('./version')
export const app = new Koa()

const router = new Router()
const rpc = new JsonRpc(config.get('name'))

app.proxy = true
app.on('error', (error) => {
    logger.error(error, 'Application error')
})

app.use(requestLogger(logger))
app.use(rpcLogger(logger))

async function healthcheck(ctx: Koa.Context) {
    const ok = true
    const date = new Date
    ctx.body = {ok, version, date}
}

router.post('/', rpc.middleware)
router.get('/.well-known/healthcheck.json', healthcheck)
router.get('/', healthcheck)

app.use(router.routes())

rpc.register('hello', async function(name: string) {
    this.log.info('Hello %s', name)
    return `I'm sorry, ${ name }, I can't do that.`
})

rpc.register('list_drafts', drafts.list)
rpc.register('save_draft', drafts.save)
rpc.register('remove_draft', drafts.remove)

rpc.register('get_feature_flag', featureFlags.getFlag)
rpc.register('set_feature_flag', featureFlags.setFlag)
rpc.register('get_feature_flags', featureFlags.getFlags)
rpc.register('set_feature_flag_probability', featureFlags.setProbability)
rpc.register('get_feature_flag_probabilities', featureFlags.getProbabilities)

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
