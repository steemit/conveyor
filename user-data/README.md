# Conveyor User Data


The `conveyor.get_account` and `conveyor.autocomplete_account` methods, known 
collectively as the "User Search API" depend on external data. This data is 
currently stored in the https://github.com/steemit/condenser and https://github.com/steemit/redeemer-irredeemables
github repos.

## User Lists
The [Makefile](../Makfile) contains the sources of the files used to compose:
- [bad actor list](./lists/bad_actors/users.js)
- [exchange accounts list](./lists/exchanges/users.js)
- [GDPR list](./lists/gdpr/users.js)

To add additional sources for any of these lists, add the url **without** the https
scheme to the correct `Makefile` variable, such as `BAD_ACTORS_LIST`, eg, to include 
an additional list of bad actors found at the url `https://domain/bad_actors_list.txt`,
you would add `domain/bad_actors_list.txt` to the variable `BAD_ACTORS_LIST` in 
the [Makefile](../Makfile)

## User Accounts
The [Makefile](../Makfile) contains the a recipe to generate a list of all accounts
and store it in [user-data/accounts/accounts.js](./accounts/accounts.js)
used when the app is started and before it begins to update that list with new
accounts.

