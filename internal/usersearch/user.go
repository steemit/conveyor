package usersearch

import (
	"encoding/json"

	protocolapi "github.com/steemit/steemutil/protocol/api"
)

// UserAccountJSON is the wire-format representation of an account, matching
// the TS UserAccountJSON interface. Field names preserve the TS naming even
// where misleading (vote_sp is actually voting_power, value_sp is balance).
type UserAccountJSON struct {
	Account          string          `json:"account"`
	VoteSp           int16           `json:"vote_sp"`
	JoinedAt         string          `json:"joined_at"`
	Reputation       json.RawMessage `json:"reputation"`
	Tags             []string        `json:"tags"`
	ValueSp          string          `json:"value_sp"`
	FollowersCount   int             `json:"followers_count"`
	FollowingCount   int             `json:"following_count"`
	ContextAccount   string          `json:"context_account,omitempty"`
	ContextRecentSends int           `json:"context_recent_sends,omitempty"`
	ContextIsFollowing bool          `json:"context_is_following,omitempty"`
	ContextIsFollower  bool          `json:"context_is_follower,omitempty"`
	ContextIsMuted     bool          `json:"context_is_muted,omitempty"`
}

// UserAccount is the in-memory domain model for a Steem account, built from
// 6 parallel steemd API calls. Mirrors TS UserAccount class.
type UserAccount struct {
	Account          string
	VoteSp           int16
	JoinedAt         string
	Reputation       json.RawMessage
	Tags             []string
	ValueSp          string
	FollowersCount   int
	FollowingCount   int
	Followers        map[string]struct{}
	Following        map[string]struct{}
	Ignored          map[string]struct{}
	TransferTargets  map[string]int
}

// NewUserAccount constructs a UserAccount from the raw steemd data, mirroring
// the TS UserAccount constructor.
func NewUserAccount(
	ext *protocolapi.ExtendedAccount,
	fc *protocolapi.FollowCountReturn,
	transferTargets map[string]int,
	followers []*protocolapi.FollowReturn,
	following []*protocolapi.FollowReturn,
	ignored []*protocolapi.FollowReturn,
) *UserAccount {
	ua := &UserAccount{
		Account:         ext.Name,
		VoteSp:          ext.VotingPower,
		JoinedAt:        ext.Created,
		Reputation:      ext.Reputation,
		Tags:            getUserTags(ext.Name),
		ValueSp:         ext.Balance,
		FollowersCount:  fc.FollowerCount,
		FollowingCount:  fc.FollowingCount,
		Followers:       make(map[string]struct{}),
		Following:       make(map[string]struct{}),
		Ignored:         make(map[string]struct{}),
		TransferTargets: transferTargets,
	}
	for _, f := range followers {
		ua.Followers[f.Follower] = struct{}{}
	}
	for _, f := range following {
		ua.Following[f.Following] = struct{}{}
	}
	for _, f := range ignored {
		ua.Ignored[f.Follower] = struct{}{}
	}
	return ua
}

// IsFollower reports whether 'account' follows this user.
func (u *UserAccount) IsFollower(account string) bool {
	_, ok := u.Followers[account]
	return ok
}

// IsFollowing reports whether this user follows 'account'.
func (u *UserAccount) IsFollowing(account string) bool {
	_, ok := u.Following[account]
	return ok
}

// IsIgnored reports whether this user has muted/ignored 'account'.
func (u *UserAccount) IsIgnored(account string) bool {
	_, ok := u.Ignored[account]
	return ok
}

// RecentSendsCount returns the number of transfers to 'account' in the last 30 days.
func (u *UserAccount) RecentSendsCount(account string) int {
	return u.TransferTargets[account]
}

// RecentSendAccounts returns all accounts this user has recently transferred to.
func (u *UserAccount) RecentSendAccounts() []string {
	out := make([]string, 0, len(u.TransferTargets))
	for k := range u.TransferTargets {
		out = append(out, k)
	}
	return out
}

// UserContext describes the relationship between the viewed account and a
// context (viewer) account.
type UserContext struct {
	ContextAccount     string
	ContextRecentSends int
	ContextIsFollowing bool
	ContextIsFollower  bool
	ContextIsMuted     bool
}

// UserContextFor builds a UserContext from this account's view of the other account.
func (u *UserAccount) UserContextFor(otherAccount string) *UserContext {
	return &UserContext{
		ContextAccount:     otherAccount,
		ContextRecentSends: u.RecentSendsCount(otherAccount),
		ContextIsFollowing: u.IsFollowing(otherAccount),
		ContextIsFollower:  u.IsFollower(otherAccount),
		ContextIsMuted:     u.IsIgnored(otherAccount),
	}
}

// ToJSON returns the base account JSON (no context fields).
func (u *UserAccount) ToJSON() UserAccountJSON {
	return UserAccountJSON{
		Account:        u.Account,
		VoteSp:         u.VoteSp,
		JoinedAt:       u.JoinedAt,
		Reputation:     u.Reputation,
		Tags:           u.Tags,
		ValueSp:        u.ValueSp,
		FollowersCount: u.FollowersCount,
		FollowingCount: u.FollowingCount,
	}
}

// ToJSONWithContext returns the account JSON merged with context fields.
func (u *UserAccount) ToJSONWithContext(uc *UserContext) UserAccountJSON {
	j := u.ToJSON()
	if uc != nil {
		j.ContextAccount = uc.ContextAccount
		j.ContextRecentSends = uc.ContextRecentSends
		j.ContextIsFollowing = uc.ContextIsFollowing
		j.ContextIsFollower = uc.ContextIsFollower
		j.ContextIsMuted = uc.ContextIsMuted
	}
	return j
}
