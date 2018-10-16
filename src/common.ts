/**
 * @file Shared things.
 * @author Johan Nordberg <johan@steemit.com>
 */

import * as config from 'config'
import * as dsteem from 'dsteem'

/**
 * Shared rpc client.
 */
export const client = new dsteem.Client(config.get('rpc_node'))
