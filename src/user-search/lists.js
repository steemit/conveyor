"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const users_1 = require("../../user-data/lists/bad_actors/users");
const users_2 = require("../../user-data/lists/exchanges/users");
// import * as verifiedList from '../../user-data/lists/verified/users' FIXME
var UserAccountTags;
(function (UserAccountTags) {
    UserAccountTags[UserAccountTags["abuse"] = 0] = "abuse";
    UserAccountTags[UserAccountTags["exchange"] = 1] = "exchange";
    UserAccountTags[UserAccountTags["verified"] = 2] = "verified";
    UserAccountTags[UserAccountTags["none"] = 3] = "none";
})(UserAccountTags || (UserAccountTags = {}));
const verified = new Set(); // FIXME
function getUserTags(account) {
    if (users_2.users.has(account)) {
        return ['exchange'];
    }
    if (users_1.users.has(account)) {
        return ['abuse'];
    }
    if (verified.has(account)) {
        return ['verified'];
    }
    return ['none'];
}
exports.getUserTags = getUserTags;
