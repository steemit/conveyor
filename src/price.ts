/**
 * @file Steem price APIs.
 * @author Johan Nordberg <johan@steemit.com>
 */

import {JsonRpcError, JsonRpcMethodContext as JCtx} from '@steemit/koa-jsonrpc'
import {Asset, Price} from 'dsteem'

import {client} from './common'

const ONE_STEEM = new Asset(1, 'STEEM')

export async function getPrices(this: JCtx) {

    // prefetch api calls
    const [orders, feed, props] = await Promise.all([
        client.call('database_api', 'get_order_book', [1]),
        client.call('database_api', 'get_feed_history'),
        client.database.getDynamicGlobalProperties(),
    ])

    // calculate the STEEM<>SBD price using the internal market
    // we average the lowest ask and highest bid to get market price
    const prices = orders.asks.concat(orders.bids).map((order) => Price.from(order.order_price))
    const sbdPerSteem = prices.reduce((t, p) => t + p.convert(ONE_STEEM).amount, 0) / prices.length

    // STEEM<>USD is reported by witnesses
    // (unit reported is SBD but under the assumption that 1 USD always equals 1 SBD)
    const steemUsdPrice = Price.from(feed.price_history[feed.price_history.length - 1])
    const usdPerSteem = steemUsdPrice.convert(ONE_STEEM).amount

    // VESTS<>STEEM price
    const vestsPrice = Price.from({
        base: props.total_vesting_fund_steem,
        quote: props.total_vesting_shares,
    })
    const vestsPerSteem = vestsPrice.convert(ONE_STEEM).amount

    return {
        steem_sbd: sbdPerSteem,
        steem_usd: usdPerSteem,
        steem_vest: vestsPerSteem,
    }
}
