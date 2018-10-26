/**
 * @file Conveyor server.
 * @author Johan Nordberg <johan@steemit.com>
 */

import * as cluster from 'cluster'
import * as config from 'config'
import * as Koa from 'koa'
import * as Router from 'koa-router'
import * as os from 'os'

const userAccountNames = require('../user-data/accounts/accounts')
import * as drafts from './drafts'
import * as featureFlags from './feature-flags'
import * as price from './price'
import * as tags from './tags'
import * as userData from './user-data'
import * as userSearch from './user-search/search'

import {JsonRpcAuth, requestLogger, rpcLogger} from '@steemit/koa-jsonrpc'
import {db} from './database'
import {logger} from './logger'
import {CachingClient} from './user-search/client'
import {AccountNameTrie} from './user-search/indexes'


export const version = require('./version')

export interface Context extends Koa.Context {
    cacheClient: any
    userAccountTrie: any
}

export interface KoaAppWithCustomContext extends Koa {
    context: Context
}

export const app = new Koa() as KoaAppWithCustomContext

const cacheClient = new CachingClient()
const userAccountTrie = new AccountNameTrie(userAccountNames, cacheClient, config.get('accounts_refresh_interval'))

app.context.cacheClient = cacheClient
app.context.userAccountTrie = userAccountTrie

const router = new Router()
export const rpc = new JsonRpcAuth(config.get('rpc_node'), config.get('name'))

app.proxy = true
app.on('error', (error) => {
    logger.error(error, 'Application error')
})

app.use(requestLogger(logger))
app.use(rpcLogger(logger))

async function healthcheck(ctx: Koa.Context) {
    const ok = true
    const date = new Date()
    ctx.body = {ok, version, date}
}

router.post('/', rpc.middleware)
router.get('/.well-known/healthcheck.json', healthcheck)
router.get('/', healthcheck)

app.use(router.routes())

rpc.register('hello', async function(name: string = 'Anonymous') {
    this.log.info('Hello %s', name)
    return `I'm sorry, ${ name }, I can't do that.`
})

rpc.register('get_account', userSearch.getAccount)
rpc.register('autocomplete_account', userSearch.autocompleteAccount)


rpc.register('get_prices', price.getPrices)

rpc.registerAuthenticated('whoami', async function() {
    this.log.info('Whoami %s', this.account)
    return this.account
})

rpc.registerAuthenticated('list_drafts', drafts.list)
rpc.registerAuthenticated('save_draft', drafts.save)
rpc.registerAuthenticated('remove_draft', drafts.remove)

rpc.registerAuthenticated('get_feature_flag', featureFlags.getFlag)
rpc.registerAuthenticated('set_feature_flag', featureFlags.setFlag)
rpc.registerAuthenticated('get_feature_flags', featureFlags.getFlags)
rpc.registerAuthenticated('set_feature_flag_probability', featureFlags.setProbability)
rpc.registerAuthenticated('get_feature_flag_probabilities', featureFlags.getProbabilities)

rpc.registerAuthenticated('get_user_data', userData.getUserData)
rpc.registerAuthenticated('set_user_data', userData.setUserData)
rpc.registerAuthenticated('is_email_registered', userData.isEmailRegistered)
rpc.registerAuthenticated('is_phone_registered', userData.isPhoneRegistered)

rpc.registerAuthenticated('define_tag', tags.defineTag)
rpc.registerAuthenticated('list_tags', tags.listTags)
rpc.registerAuthenticated('assign_tag', tags.assignTag)
rpc.registerAuthenticated('unassign_tag', tags.unassignTag)
rpc.registerAuthenticated('get_users_by_tags', tags.getUsersByTags)
rpc.registerAuthenticated('get_tags_for_user', tags.getTagsForUser)

function run() {
    const port = config.get('port')
    const server = app.listen(port, () => {
        logger.info('running on port %d', port)
    })
    app.context.userAccountTrie.startRefreshing()
    logger.info('registering server on close listener')
    server.on('close', () => {
        logger.fatal('on "app.close" stopping accounts refresh loop')
        app.context.userAccountTrie.stopRefreshing()
    })
}

if (module === require.main) {
    if (cluster.isMaster) {
        db.sync().catch((error) => {
            logger.fatal(error, 'unable to sync database')
        })
    }
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
