
conveyor
========

Steemit API


Feature flags
-------------

Feature flags allows our apps (condenser mainly) to hide certain features behind flags.

Flags can be set individually for users or probabilistically to roll out a feature incrementally, for example `set_flag_probability orange_theme 0.5` will enable the flag for ~50% of our users. Changing the flag probability does not re-randomize the user selection so that a user that had their flag enabled at 0.5 will still have it enabled when going to 0.75. This is achieved by seeding a PRNG with the shasum of username+flag.

Flags set individually per user always takes precedence.

### API

#### `get_feature_flags <username>`

Get feature flags object for `<username>`.

*Authenticated: requires signature of `<username>` or an admin account.*

Returns: Key value mapping of feature flags, missing flags should be treated as `false` by the client.

```js
{
    horse_hockey_minigame_unlocked: true,
    donovan_theme: false
}
```


#### `get_feature_flag <username> <flag>`

Get specific `<flag>` for `<username>`.

*Authenticated: requires signature of <username> or an admin account.*

Returns: false/true


#### `set_feature_flag <username> <flag> <true/false>`

Set a `<flag>` override for `<username>`.

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

#### `get_user_data <username>`

Return user data for `<username>`, returns an error if the account is not found.

*Authenticated: requires signature of an admin account or the requested user.*

```js
{
    phone: '+1480080085',
    email: 'foo@bar.com'
}
```


#### `set_user_data <username> <data>`

Set user data for `<username>`.

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
