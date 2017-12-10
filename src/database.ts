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

export const User = db.define('user', {
    account: Sequelize.STRING,
    email: Sequelize.STRING,
    phone: Sequelize.STRING,
})
