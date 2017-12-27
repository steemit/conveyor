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
