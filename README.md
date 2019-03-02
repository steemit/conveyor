
conveyor
========

Knowledgr API

Additional docs may be found here:
- [docs/Conveyor.md](./docs/Conveyor.md)
- [docs/Conveyor.html](./docs/Conveyor.html)
- [user-data/README.md](./user-data/README.md)


Feature flags
-------------

Feature flags allows our apps (condenser mainly) to hide certain features behind flags.

Flags can be set individually for users or probabilistically to roll out a feature incrementally, for example `set_flag_probability orange_theme 0.5` will enable the flag for ~50% of our users. Changing the flag probability does not re-randomize the user selection so that a user that had their flag enabled at 0.5 will still have it enabled when going to 0.75. This is achieved by seeding a PRNG with the shasum of username+flag.

Flags set individually per user always takes precedence.

### API

#### `get_feature_flags <account>`

Get feature flags object for the username `<account>`.

*Authenticated: requires signature of `<account>` or an admin account.*

Returns: Key value mapping of feature flags, missing flags should be treated as `false` by the client.

```js
{
    horse_hockey_minigame_unlocked: true,
    donovan_theme: false
}
```


#### `get_feature_flag <account> <flag>`

Get specific `<flag>` for the username `<account>`.

*Authenticated: requires signature of <account> or an admin account.*

Returns: false/true


#### `set_feature_flag <account> <flag> <value>`

Set a `<flag>` override for the username `<account>`.

*Authenticated: requires signature of an admin account.*


#### `set_feature_flag_probability <flag> <probability>`

Set the `<probability>` expressed as a fraction `0.0` to `1.0` that a `<flag>` will resolve to true.

*Authenticated: requires signature of an admin account.*


#### `get_feature_flag_probabilities`

Get all the set flag probabilities.

*Authenticated: requires signature of an admin account.*

```js
{
    horse_hockey_minigame_unlocked: 1.0,
    donovan_theme: 0.5,
    lucky_af: 0.0001
}
```


User data
---------

Conveyor is the central point for storing sensitive user data (email, phone, etc). No other services should store this data and should instead query for it here every time.

### API

#### `get_user_data <account>`

Return user data for `<account>`, returns an error if the account is not found.

*Authenticated: requires signature of an admin account or the requested user.*

```js
{
    phone: '+1480080085',
    email: 'foo@bar.com'
}
```


#### `set_user_data <account> <data>`

Set user data for `<account>`.

*Authenticated: requires signature of an admin account.*

`<data>` is an object with the keys `email` and `phone`.


#### `is_email_registered <email>`

Check if the `<email>` address is in the database.

*Authenticated: requires signature of an admin account.*

Returns `true` or `false`


#### `is_phone_registered <phone>`

Check if the `<phone>` number is in the database.

*Authenticated: requires signature of an admin account.*

Returns `true` or `false`



User tags
---------

Tagging mechanism for other services, allows defining and assigning tags to accounts (or other identifiers) and querying for them.

### API

#### `define_tag <name> <description>`

Define a new tag, valid tag names include only alphanumeric characters and underscore (`_`), description is required.

*Authenticated: requires signature of an admin account.*


#### `list_tags`

List all defined tags.

*Authenticated: requires signature of an admin account.*

```js
[ { name: 'make_site_unbearable', description: 'Slows down the site for user by adding a sleep(5) to every request.' },
  { name: 'accepted_tos', description: 'User has accepted the terms of service' } ]
```

#### `assign_tag <uid> <tag> [memo]`

Assign `<tag>` to user, throws if tag is not defined.

Note that this is a no-op if user already has the tag.

*Authenticated: requires signature of an admin account.*


#### `unassign_tag <uid> <tag>`

Remove `<tag>` from user.

Note that this is a no-op if user already has the tag.

*Authenticated: requires signature of an admin account.*


#### `get_users_by_tags <tags>`

Get a list of users that has `<tags>` assigned, `<tags>` can be either
a string or an array of strings in which case the user must have all the tags given
to be included in the response.

*Authenticated: requires signature of an admin account.*

```js
[ 'some_user', 'another_user' ]
```


#### `get_tags_for_user <uid>`

Get a list of tags assigned to `<uid>`.

```js
[ 'some_tag', 'another_tag' ]
```

*Authenticated: requires signature of an admin account.*
