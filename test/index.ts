import {rpc} from './../src/server'
import {client} from './../src/common'
before(() => {
    // swizzle the rpc verifier to verify anything except account
    // names containing the string 'fail'
    (<any>rpc).verifier = async (message, signatures, account) => {
        if (account.indexOf('fail') !== -1) {
            throw new Error('Fail')
        }
    }
    // mock rpc calls
    client.call = async (api: string, method: string, params = []) => {
        const apiMethod = `${ api }.${ method }`
        switch (apiMethod) {
            case 'database_api.get_dynamic_global_properties':
                return {
                    total_vesting_fund_steem: '100.000 STEEM',
                    total_vesting_shares: '4200.000000 VESTS',
                }
            case 'database_api.get_feed_history':
                return {
                    price_history: [
                        { base: '5.000 SBD', quote: '1.000 STEEM' }
                    ]
                }
            case 'database_api.get_order_book':
                return {
                    bids: [ { order_price: { base: '1.000 SBD', quote: '5.000 STEEM' } } ],
                    asks: [ { order_price: { base: '5.000 STEEM', quote: '1.000 SBD' } } ],
                }
            default:
                throw new Error(`No mock data for: ${ apiMethod }`)
        }
    }
})
after(() => {
    // just a precaution in case the testing code is accidentally exec'd
    (<any>rpc).verifier = async () => {
        throw new Error('You shall not pass')
    }
    client.call = async () => {
        throw new Error('Run you fools')
    }
})
