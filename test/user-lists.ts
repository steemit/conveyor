import * as assert from 'assert'
import 'mocha'
import * as sinon from 'sinon'

import {getUserTags} from '../src/user-search/lists'

describe('user lists', function(this) {
    it('should return ["exchange"] ', async function(this) {
        const results = getUserTags('poloniex')
        assert.deepEqual(results, ['exchange'])
    })
    it('should return ["abuse"] ', async function(this) {
        const results = getUserTags('a-0-0')
        assert.deepEqual(results, ['abuse'])
    })
    it('should return ["none"] ', async function(this) {
        const results = getUserTags('steemit')
        assert.deepEqual(results, ['none'])
    })
})
