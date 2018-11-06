/**
 * @file Link summarizer.
 * @author Benjamin Chodoroff <bchodoroff@steemit.com>
 */

import * as got from 'got'
import * as lruCache from 'lru-cache'
import * as unfluff from 'unfluff'
import * as url from 'url'

import { JsonRpcAuthMethodContext as JCtx, JsonRpcError } from '@steemit/koa-jsonrpc'

import { badDomains, badUrls } from './blacklist'

interface SummarizedUrl {
    description?: string
    image?: string
    title?: string
}

// Set up in-memory cache for our responses
const cache = lruCache({
    max: 10000,
    maxAge: 1000 * 60 * 60,
})

export async function summarizeUrl(this: JCtx, urlStr: string): Promise<SummarizedUrl> {
    let parsedUrl: any

    try {
        parsedUrl = url.parse(urlStr)
        this.assert(typeof parsedUrl.host === 'string')
        this.assert(typeof parsedUrl.pathname === 'string')
    } catch (e) {
        throw new JsonRpcError(400, 'Cannot parse URL')
    }

    // Refuse to summarize blacklisted domains
    badDomains.forEach((badDomain) => {
        if (parsedUrl.host.endsWith(badDomain)) {
            throw new JsonRpcError(403, 'Domain is blacklisted')
        }
    })

    // Refuse to summarize blacklisted URLs
    const urlLessProtocol = `${parsedUrl.host}${parsedUrl.pathname}`
    badUrls.forEach((badUrl) => {
        if (urlLessProtocol.startsWith(badUrl)) {
            throw new JsonRpcError(403, 'URL is blacklisted')
        }
    })

    // Serve from cache if possible
    if (cache.has(urlStr)) {
        this.log.debug({ url: urlStr }, 'serving from cache')
        return cache.get(urlStr)
    } else {
        let urlData
        try {
            urlData = await got(urlStr)
        } catch (e) {
            throw new JsonRpcError(400, 'Cannot fetch URL')
        }

        const unfluffed = unfluff(urlData.body)

        const res = {
            description: unfluffed.description,
            image: unfluffed.image,
            title: unfluffed.title,
        }

        this.log.debug({ url: urlStr }, 'storing in cache')
        cache.set(urlStr, res)

        return res
    }
}
