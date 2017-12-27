import {rpc} from './../src/server'
before(() => {
    // swizzle the rpc verifier to verify anything except account
    // names containing the string 'fail'
    (<any>rpc).verifier = async (message, signatures, account) => {
        if (account.indexOf('fail') !== -1) {
            throw new Error('Fail')
        }
    }
})
after(() => {
    // just a precaution in case the testing code is accidentally exec'd
    (<any>rpc).verifier = async () => {
        throw new Error('You shall not pass')
    }
})
