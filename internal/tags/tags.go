// Package tags implements the tag subsystem (define/list/assign/unassign/
// query), mirroring the original TS src/tags.ts. Tags are stored in the Tag
// and UserTag tables via GORM.
package tags

import (
	"fmt"
	"regexp"
	"sort"
	"time"

	"gorm.io/gorm"

	"github.com/steemit/conveyor/internal/jsonrpc"
	"github.com/steemit/conveyor/internal/models"
)

var tagPattern = regexp.MustCompile(`^[a-z0-9_]+$`)

// Tags holds the dependencies for tag operations.
type Tags struct {
	db        *gorm.DB
	adminRole string
}

// New creates a Tags instance.
func New(db *gorm.DB, adminRole string) *Tags {
	return &Tags{db: db, adminRole: adminRole}
}

// Register registers all tag RPC methods.
func (t *Tags) Register(rpc *jsonrpc.Server) {
	rpc.RegisterAuthenticated("conveyor.define_tag", t.defineTag)
	rpc.RegisterAuthenticated("conveyor.list_tags", t.listTags)
	rpc.RegisterAuthenticated("conveyor.assign_tag", t.assignTag)
	rpc.RegisterAuthenticated("conveyor.unassign_tag", t.unassignTag)
	rpc.RegisterAuthenticated("conveyor.get_users_by_tags", t.getUsersByTags)
	rpc.RegisterAuthenticated("conveyor.get_tags_for_user", t.getTagsForUser)
}

func (t *Tags) assertAdmin(ctx *jsonrpc.Context) *jsonrpc.Error {
	if ctx.Account != t.adminRole {
		return jsonrpc.NewError(400, nil, "Unauthorized")
	}
	return nil
}

func (t *Tags) defineTag(ctx *jsonrpc.Context, req *jsonrpc.Request) (any, error) {
	if e := t.assertAdmin(ctx); e != nil {
		return nil, e
	}
	var p struct {
		Name        string `json:"tag"`
		Description string `json:"description"`
	}
	if err := req.UnmarshalParams(&p); err != nil {
		return nil, err
	}
	if !tagPattern.MatchString(p.Name) {
		return nil, jsonrpc.NewError(400, nil, "Invalid tag name")
	}
	if p.Description == "" {
		return nil, jsonrpc.NewError(400, nil, "Invalid tag description")
	}
	if err := t.db.WithContext(req.Ctx).Create(&models.Tag{Name: p.Name, Description: p.Description}).Error; err != nil {
		return nil, jsonrpc.ErrInternalError(err)
	}
	return true, nil
}

func (t *Tags) listTags(ctx *jsonrpc.Context, req *jsonrpc.Request) (any, error) {
	if e := t.assertAdmin(ctx); e != nil {
		return nil, e
	}
	var tags []models.Tag
	if err := t.db.WithContext(req.Ctx).Order("name ASC").Find(&tags).Error; err != nil {
		return nil, jsonrpc.ErrInternalError(err)
	}
	result := make([]map[string]string, len(tags))
	for i, tag := range tags {
		result[i] = map[string]string{"name": tag.Name, "description": tag.Description}
	}
	return result, nil
}

func (t *Tags) assignTag(ctx *jsonrpc.Context, req *jsonrpc.Request) (any, error) {
	if e := t.assertAdmin(ctx); e != nil {
		return nil, e
	}
	var p struct {
		UID  string `json:"uid"`
		Tag  string `json:"tag"`
		Memo string `json:"memo"`
	}
	if err := req.UnmarshalParams(&p); err != nil {
		return nil, err
	}
	if p.UID == "" {
		return nil, jsonrpc.NewError(400, nil, "Invalid user uid")
	}
	if !tagPattern.MatchString(p.Tag) {
		return nil, jsonrpc.NewError(400, nil, "Invalid tag")
	}
	if p.Memo == "" {
		p.Memo = fmt.Sprintf("Created by %s from %s", ctx.Account, ctx.IP)
	}

	// Application-layer FK check: verify the tag exists (no DB-level FK in Go version).
	var tag models.Tag
	result := t.db.WithContext(req.Ctx).First(&tag, "name = ?", p.Tag)
	if result.Error != nil {
		return nil, jsonrpc.NewError(420, nil, "No such tag")
	}

	// Check for existing active assignment (idempotent).
	var count int64
	t.db.WithContext(req.Ctx).Model(&models.UserTag{}).
		Where("uid = ? AND tag = ? AND deleted_at IS NULL", p.UID, p.Tag).Count(&count)
	if count > 0 {
		return []string{}, nil // idempotent: already assigned
	}

	if err := t.db.WithContext(req.Ctx).Create(&models.UserTag{UID: p.UID, Tag: p.Tag, Memo: p.Memo}).Error; err != nil {
		return nil, jsonrpc.ErrInternalError(err)
	}
	return []string{}, nil
}

func (t *Tags) unassignTag(ctx *jsonrpc.Context, req *jsonrpc.Request) (any, error) {
	if e := t.assertAdmin(ctx); e != nil {
		return nil, e
	}
	var p struct {
		UID string `json:"uid"`
		Tag string `json:"tag"`
	}
	if err := req.UnmarshalParams(&p); err != nil {
		return nil, err
	}
	if p.UID == "" {
		return nil, jsonrpc.NewError(400, nil, "Invalid user uid")
	}
	if !tagPattern.MatchString(p.Tag) {
		return nil, jsonrpc.NewError(400, nil, "Invalid tag")
	}

	now := time.Now()
	t.db.WithContext(req.Ctx).Model(&models.UserTag{}).
		Where("uid = ? AND tag = ? AND deleted_at IS NULL", p.UID, p.Tag).
		Update("deleted_at", now)

	return map[string]any{}, nil
}

func (t *Tags) getUsersByTags(ctx *jsonrpc.Context, req *jsonrpc.Request) (any, error) {
	if e := t.assertAdmin(ctx); e != nil {
		return nil, e
	}

	// tags can be a string or []string. Parse from raw params.
	var p struct {
		Tags any `json:"tags"`
	}
	if err := req.UnmarshalParams(&p); err != nil {
		return nil, err
	}

	var tags []string
	switch v := p.Tags.(type) {
	case string:
		tags = []string{v}
	case []any:
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, jsonrpc.NewError(400, nil, "Invalid tags")
			}
			tags = append(tags, s)
		}
	default:
		return nil, jsonrpc.NewError(400, nil, "Invalid tags")
	}

	if len(tags) == 0 {
		return []string{}, nil
	}

	// Query active UserTags matching any of the tags.
	var userTags []models.UserTag
	t.db.WithContext(req.Ctx).
		Where("tag IN ? AND deleted_at IS NULL", tags).
		Find(&userTags)

	// Build uid → taglist map, then find uids with ALL tags (intersection).
	tagmap := make(map[string][]string)
	for _, ut := range userTags {
		tagmap[ut.UID] = append(tagmap[ut.UID], ut.Tag)
	}

	tagSet := make(map[string]bool)
	for _, t := range tags {
		tagSet[t] = true
	}

	var result []string
	for uid, taglist := range tagmap {
		// Check if taglist contains all requested tags.
		has := make(map[string]bool)
		for _, t := range taglist {
			if tagSet[t] {
				has[t] = true
			}
		}
		allPresent := true
		for _, t := range tags {
			if !has[t] {
				allPresent = false
				break
			}
		}
		if allPresent {
			result = append(result, uid)
		}
	}

	sort.Strings(result)
	return result, nil
}

func (t *Tags) getTagsForUser(ctx *jsonrpc.Context, req *jsonrpc.Request) (any, error) {
	if e := t.assertAdmin(ctx); e != nil {
		return nil, e
	}
	var p struct {
		UID   string `json:"uid"`
		Audit bool   `json:"audit"`
	}
	if err := req.UnmarshalParams(&p); err != nil {
		return nil, err
	}
	if p.UID == "" {
		return nil, jsonrpc.NewError(400, nil, "Invalid user uid")
	}

	if p.Audit {
		var userTags []models.UserTag
		t.db.WithContext(req.Ctx).Where("uid = ?", p.UID).Order("created_at ASC").Find(&userTags)
		// Return full records.
		result := make([]map[string]any, len(userTags))
		for i, ut := range userTags {
			m := map[string]any{
				"uid":   ut.UID,
				"tag":   ut.Tag,
				"memo":  ut.Memo,
			}
			if ut.DeletedAt != nil {
				m["deletedAt"] = ut.DeletedAt
			}
			m["createdAt"] = ut.CreatedAt
			result[i] = m
		}
		return result, nil
	}

	// Non-audit: return active tag names only, sorted.
	var userTags []models.UserTag
	t.db.WithContext(req.Ctx).Where("uid = ? AND deleted_at IS NULL", p.UID).Find(&userTags)
	tags := make([]string, len(userTags))
	for i, ut := range userTags {
		tags[i] = ut.Tag
	}
	sort.Strings(tags)
	return tags, nil
}
