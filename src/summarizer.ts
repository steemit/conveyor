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
    blacklisted?: boolean
    description?: string
    favicon?: string
    image?: string
    videos?: SummarizedUrlVideo[]
    title?: string
}

interface SummarizedUrlVideo {
  src: string
  height: number
  width: number
}

// Set up in-memory cache for our responses
const cache = lruCache({
    max: 10000,
    maxAge: 1000 * 60 * 60,
})

export async function summarizeUrl(this: JCtx, urlStr: string): Promise<SummarizedUrl> {
    let parsedUrl: any
    let blacklisted: boolean = false

    try {
        parsedUrl = url.parse(urlStr)
        this.assert(typeof parsedUrl.host === 'string')
        this.assert(typeof parsedUrl.pathname === 'string')
    } catch (e) {
        throw new JsonRpcError(400, 'Cannot parse URL')
    }

    // Is domain blacklisted?
    for (const badDomain of badDomains) {
        if (parsedUrl.host.endsWith(badDomain)) {
            blacklisted = true
            break
        }
    }

    // Is url blacklisted?
    if (!blacklisted) {
      const urlLessProtocol = `${parsedUrl.host}${parsedUrl.pathname}`
      for (const badUrl of badUrls) {
          if (urlLessProtocol.startsWith(badUrl)) {
              blacklisted = true
              break
          }
      }
    }

    // Serve from cache if possible
    if (cache.has(urlStr)) {
        this.log.debug({ url: urlStr }, 'serving from cache')
        return cache.get(urlStr)
    } else {
        let urlData
        try {
            urlData = await got(urlStr, { timeout: 2000 })
        } catch (e) {
            throw new JsonRpcError(400, 'Cannot fetch URL')
        }

        const unfluffed = unfluff(urlData.body)

        const res = {
            blacklisted,
            description: unfluffed.description,
            favicon: unfluffed.favicon,
            image: unfluffed.image,
            videos: unfluffed.videos.map((video) => ({
              src: video.src,
              height: video.height ? parseInt(video.height, 10) : null,
              width: video.width ? parseInt(video.width, 10) : null,
            })),
            title: unfluffed.title,
        }

        this.log.debug({ url: urlStr }, 'storing in cache')
        cache.set(urlStr, res)

        return res
    }
}
