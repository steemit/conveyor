/**
 * @file Relational database models using sequelize.
 * @author Johan Nordberg <johan@steemit.com>
 */

import * as config from 'config'
import * as Sequelize from 'sequelize'

import {logger as baseLogger} from './logger'

const logger = baseLogger.child({module: 'database'})
const dbConfig = config.get('database') as Sequelize.Options
dbConfig.logging = (msg) => logger.debug(msg)

logger.info('setting up database with dialect %s', dbConfig.dialect)

export const db = new Sequelize(config.get('database'))

export interface UserAttributes {
    account: string
    email: string
    phone: string
}

export interface UserInstance extends Sequelize.Instance<UserAttributes>, UserAttributes {}

export const User = db.define<UserInstance, UserAttributes>('user', {
    account: {
        type: Sequelize.STRING,
        primaryKey: true,
    },
    email: {
        type: Sequelize.STRING,
        allowNull: true,
        unique: true,
        validate: {
            isEmail: true
        }
    },
    phone: {
        type: Sequelize.STRING,
        allowNull: true,
        unique: true,
        validate: {
            is: /^\+[0-9]+$/
        }
    },
})

export interface TagAttributes {
    /** Tag name, may only contain alphanumeric and underscore. */
    name: string
    /** Description of tag. */
    description: string
}

export interface TagInstance extends Sequelize.Instance<TagAttributes>, TagAttributes {}

export const Tag = db.define<TagInstance, TagAttributes>('tag', {
    name: {
        type: Sequelize.STRING,
        primaryKey: true,
        validate: {
            is: /^[a-z0-9_]+$/
        }
    },
    description: {
        allowNull: false,
        type: Sequelize.STRING,
    }
})

export interface UserTagAttributes {
    /** Account name or other unique identifier. */
    uid: string
    /** Assigned tag. */
    tag: string
}

export interface UserTagInstance extends Sequelize.Instance<UserTagAttributes>, UserTagAttributes {}

export const UserTag = db.define<UserTagInstance, UserTagAttributes>('usertag', {
    uid: {
        type: Sequelize.STRING,
        allowNull: false,
    },
    tag: {
        type: Sequelize.STRING,
        allowNull: false,
        references: {
            model: Tag,
            key: 'name',
        }
   },
}, {
    indexes: [{fields: ['uid']}]
})
