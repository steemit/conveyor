/**
 * @file Overseer server.
 * @author Johan Nordberg <johan@steemit.com>
 */

import * as bunyan from 'bunyan'
import * as cluster from 'cluster'
import * as config from 'config'
import * as Koa from 'koa'
import * as os from 'os'
import * as UUID from 'uuid/v4'

import JsonRpc, {JsonRpcRequest, JsonRpcResponse} from './jsonrpc'

const rakam = new Rakam(
    config.get('rakam.api_endpoint'),
    config.get('rakam.api_key')
)

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
    }),
    serializers: {
        rpc_req: (req: JsonRpcRequest) => {
            return `${ req.method }:${ req.id || 'null' }`
        }
    }
})

export const app = new Koa()
const rpc = new JsonRpc()

app.proxy = true
app.on('error', (error) => {
    logger.error(error, 'Application error')
})

/**
 * Request id middleware. Tags each request with a UUID,
 * uses the X-Request-Id header if present.
 */
app.use((ctx, next) => {
    ctx['start_time'] = process.hrtime()
    const uuid = ctx.request.get('X-Request-Id') || UUID()
    ctx['req_id'] = uuid
    ctx.response.set('X-Request-Id', uuid)
    return next()
})

/**
 * Log middleware. Attaches a bunyan child logger with
 * the uuid and ip to the request context.
 */
app.use((ctx, next) => {
    const log = logger.child({
        req_id: ctx['req_id'],
        req_ip: ctx.request.ip
    })
    ctx['log'] = log
    return next()
})

/** Request logger middleware. */
app.use((ctx, next) => {
    const log: bunyan = ctx['log']
    log.debug('<-- %s %s', ctx.method, ctx.path)
    const done = () => {
        const delta = process.hrtime(ctx['start_time'])
        const ms = delta[0] * 1e3 + delta[1] / 1e6
        const size = ctx.response.length
        log.debug({ms, size}, '--> %s %s %d', ctx.method, ctx.path, ctx.status)
    }
    ctx.res.once('close', done)
    ctx.res.once('finish', done)
    return next()
})

/** JSON RPC middleware. */
app.use(rpc.middleware)

/** RPC logging middleware. */
app.use(async (ctx, next) => {
    const logResponse = (response: JsonRpcResponse) => {
        if (!(response instanceof JsonRpcResponse)) {
            return
        }
        let log: bunyan = ctx['log']
        if (response.request) {
            log = log.child({rpc_req: response.request})
        }
        if (response.error) {
            log.error(response.error)
        } else {
            log.debug({ms: response.time}, 'rpc call')
        }
    }
    let responses = ctx.body
    if (!Array.isArray(ctx.body)) {
        responses = [responses]
    }
    responses.forEach(logResponse)
    return next()
})

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
