
/**
 * Abstract blob store interface.
 * See https://github.com/maxogden/abstract-blob-store
 */
export declare class BlobStore {

    constructor(opts?: any)

    /**
     * This method should return a writable stream, and call cb with err or
     * metadata when it finishes writing the data to the underlying blob store.
     */
    public createWriteStream(
        opts: BlobKey, cb: (error: Error | undefined, metadata: any) => void
    ): NodeJS.WritableStream

    /**
     * This method should return a readable stream that emits blob data from
     * the underlying blob store or emits an error if the blob does not exist
     * or if there was some other error during the read.
     */
    public createReadStream(opts: BlobKey): NodeJS.ReadStream

    /**
     * This checks if a blob exists in the store.
     */
    public exists(opts: BlobKey, cb: (error: Error | undefined, exists: boolean) => void)

    /**
     * This checks if a blob exists in the store.
     */
    public remove(opts: BlobKey, cb: (error: Error | undefined, exists: boolean) => void)

}

/**
 * If opts is a string it should be interpreted as a key.
 * Otherwise opts should be an object with any blob metadata you
 * would like to store, e.g. name. It must have a key property
 * that the user can pass to other methods to get the blob back again.
 */
export type BlobKey = string | {[key: string]: any}

/**
 * Abstract blob store wrapper with helpers for making async calls.
 */
export class AsyncBlobStore {

    constructor(public store: BlobStore) {}

    public async read(opts: BlobKey) {
        return new Promise<Buffer>((resolve, reject) => {
            const stream = this.store.createReadStream(opts)
            const chunks: any[] = []
            stream.on('data', (chunk) => { chunks.push(chunk) })
            stream.on('error', reject)
            stream.on('end', () => { resolve(Buffer.concat(chunks)) })
        })
    }

    public async safeRead(opts: BlobKey) {
        try {
            return await this.read(opts)
        } catch (error) {
            // https://github.com/maxogden/abstract-blob-store/pull/31
            if (!error.notFound && error.message !== 'Blob not found') {
                throw error
            }
        }
    }

    public async write(opts: BlobKey, data: Buffer | string): Promise<{[key: string]: any}> {
        return new Promise(async (resolve, reject) => {
            const stream = this.store.createWriteStream(opts, (error, metadata) => {
                if (error) { reject(error) } else { resolve(metadata) }
            })
            stream.write(data)
            stream.end()
        })
    }

}
