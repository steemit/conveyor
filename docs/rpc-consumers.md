# RPC Method Consumers

Inventory of which services actually call conveyor's JSON-RPC methods, based
on a workspace-wide code audit plus live verification of the deployed
environments (EB + SSM container inspection) on **2026-09-23**.

Purpose: feed security decisions (nonce-replay exposure, feature-flag
dormancy — see the 2026-08-18 audit decisions in PR #64) and any future
deprecation work with facts about real callers instead of assumptions.

## Summary

Conveyor registers 24 methods (5 public, 19 requiring a `__signed` envelope).
Only **5 distinct methods have active callers** today, across **2 consumer
services**:

| Consumer | Deployment | Methods called |
|---|---|---|
| **faucet** | dev + production | `set_user_data`, `assign_tag`, `is_email_registered`, `is_phone_registered` |
| **condenser (legacy build) in production** | production only | `get_tags_for_user`, `assign_tag` (TOS endpoints) |

`assign_tag` is the only method with two consumers. All other signed methods
(15) and the feature-flag family have **no callers**.

## Active consumers

### faucet (dev + production)

Signs server-side with its own posting key (`CONVEYOR_USERNAME` /
`CONVEYOR_POSTING_WIF`) via `steem.api.signedCallAsync`:

- `helpers/services.js` — `conveyorCall()` wrapper
- `routes/apiHandlers.js` — signup flow: `set_user_data` (store email, with a
  single retry), `assign_tag` (`accepted_tos_20180614`), plus the
  `is_email_registered` / `is_phone_registered` availability checks

### condenser — legacy build, production

Production runs the legacy (master-branch) build of condenser, verified in the
`steemit-production-beta-condenser-001` container on 2026-09-23: image
`steemit/condenser:pr4032-test1-*`, package name `steemit.com`, legacy layout
(`lib/`, `webpack/`, `Vagrantfile`) and `src/server/api/general.js` present.

Its server-side TOS endpoints call conveyor with the conveyor account's
posting key:

- `GET /isTosAccepted` → `conveyor.get_tags_for_user`
- `POST /acceptTos` → `conveyor.assign_tag`

> Tag-prefix note for `steemit/condenser` images: `next-<sha>-<ts>` tags are
> the rewrite; bare `pr<N>-test1-<ts>` tags are legacy (master) builds. The
> dev environment (`steemit-dev-condenser-002`) runs the rewrite
> (`next-e0bcaa6-*`); production still runs legacy.

## Methods with no callers

- **Feature flags** (all 5 methods): the legacy condenser's
  `get_feature_flags` saga is disabled by an early return (`UserSaga.js`,
  "// not yet in use"); the rewrite has zero references; faucet and wallet
  never call them. Dormant by design — see the KNOWN LIMITATION on
  `internal/featureflags/flags.go` before putting them into use (salt
  prerequisite, audit T-011).
- **Drafts** (3), the remaining **user-data** (`get_user_data`) and **tags**
  (`define_tag`, `list_tags`, `get_users_by_tags`) methods, plus `whoami`:
  registered for API-contract parity with the legacy TS service; no internal
  consumer calls them today.
- **Public methods** (`hello`, `get_prices`, `get_account`,
  `autocomplete_account`, `summarize_url`) are unauthenticated by design and
  face the open internet rather than a specific consumer.

## Holders of conveyor credentials that are NOT RPC consumers

These services hold `CONVEYOR_USERNAME` / `CONVEYOR_POSTING_WIF` but call the
**kingdom** service, not conveyor — conveyor's account is the on-chain
identity for account creation and recovery:

- **wallet** (dev + production): `kingdom.recovery_account`
  (`src/lib/steem/server.ts`, `requestAccountRecovery`)
- **faucet**: `kingdom.create_account` (`helpers/services.js`)
- **condenser rewrite**: its EB env carries `CONVEYOR_USERNAME`, but no code
  reads it — vestigial from the legacy migration.

## Rewrite transition

The condenser rewrite (repo `next` branch, v16.3.x, currently dev-only) has
**zero** conveyor references — no RPC calls, no signed calls. When it
replaces the legacy build in production:

- `get_tags_for_user` loses its last consumer (TOS flow disappears with the
  legacy app);
- `assign_tag` shrinks to faucet-only (signup TOS tag);
- the feature-flag family remains dormant.

## Verification trail

- Code audit: workspace-wide grep for `conveyor.` method strings and
  `CONVEYOR_*` credentials across faucet, condenser (rewrite), condenser-legacy,
  wallet, kingdom, hivemind, jussi.
- Live check (2026-09-23): EB environment versions + SSM `docker ps` /
  in-container inspection for `steemit-dev-conveyor-002`,
  `steemit-production-beta-condenser-001` (image layout, `package.json`
  identity, presence of legacy conveyor call sites).
