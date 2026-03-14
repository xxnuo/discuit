package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	idb "github.com/discuitnet/discuit/internal/db"
	"github.com/discuitnet/discuit/internal/httperr"
	msql "github.com/discuitnet/discuit/internal/sql"
	"github.com/discuitnet/discuit/internal/uid"
	"github.com/discuitnet/discuit/internal/utils"
	"gorm.io/gorm"
)

// ContentType distinguishes between different types of user generated content
// (like posts and comments). It's integer values is used when interacting with
// the DB. With the use of the MarshalText and UnmarshalText functions, this
// type, when used as a field in a struct, is JSON marshaled and unmarshaled as
// a string.
type ContentType int

const (
	ContentTypePost = ContentType(iota)
	ContentTypeComment
)

func (t ContentType) String() string {
	b, err := t.MarshalText()
	if err != nil {
		return "[error]"
	}
	return string(b)
}

func (t ContentType) MarshalText() ([]byte, error) {
	switch t {
	case ContentTypePost:
		return []byte("post"), nil
	case ContentTypeComment:
		return []byte("comment"), nil
	}
	return nil, errors.New("unsupported content type")
}

func (t *ContentType) UnmarshalText(data []byte) error {
	switch string(data) {
	case "post":
		*t = ContentTypePost
	case "comment":
		*t = ContentTypeComment
	default:
		return errors.New("unsupported content type")
	}
	return nil
}

// Comment is a comment of a post.
type Comment struct {
	ID               uid.ID        `json:"id"`
	PostID           uid.ID        `json:"postId"`
	PostPublicID     string        `json:"postPublicId"`
	CommunityID      uid.ID        `json:"communityId"`
	CommunityName    string        `json:"communityName"`
	AuthorID         uid.ID        `json:"userId,omitempty"`
	AuthorUsername   string        `json:"username"`
	AuthorGhostID    string        `json:"userGhostId,omitempty"`
	PostedAs         UserGroup     `json:"userGroup"`
	AuthorDeleted    bool          `json:"userDeleted"`
	ParentID         uid.NullID    `json:"parentId"`
	Depth            int           `json:"depth"`
	NumReplies       int           `json:"noReplies"`
	NumRepliesDirect int           `json:"noRepliesDirect"`
	Ancestors        []uid.ID      `json:"ancestors"` // From root to parent.
	Body             string        `json:"body"`
	Upvotes          int           `json:"upvotes"`
	Downvotes        int           `json:"downvotes"`
	Points           int           `json:"-"`
	CreatedAt        time.Time     `json:"createdAt"`
	EditedAt         msql.NullTime `json:"editedAt"`

	// If the comment is deleted and the content of the comment (body, author,
	// etc) exists in the DB, and if ContentStripped is true, then those values
	// are stripped to default values in this struct.
	//
	// The JSON value is found only if the comment is deleted.
	ContentStripped *bool `json:"contentStripped,omitempty"`

	Deleted   bool          `json:"deleted"`
	DeletedAt msql.NullTime `json:"deletedAt"`
	DeletedBy uid.NullID    `json:"-"`
	DeletedAs UserGroup     `json:"deletedAs,omitempty"`

	Author *User `json:"author,omitempty"`

	// Reports whether the author of this comment is muted by the viewer.
	IsAuthorMuted bool `json:"isAuthorMuted,omitempty"`

	ViewerVoted   msql.NullBool `json:"userVoted"`
	ViewerVotedUp msql.NullBool `json:"userVotedUp"`

	PostTitle     string    `json:"postTitle,omitempty"`
	PostDeleted   bool      `json:"postDeleted"`
	PostDeletedAs UserGroup `json:"postDeletedAs,omitempty"`
}

var commentSelectColumns = []string{
	"comments.id",
	"comments.post_id",
	"comments.post_public_id",
	"comments.community_id",
	"comments.community_name",
	"comments.user_id",
	"comments.username",
	"comments.user_group",
	"comments.user_deleted",
	"comments.parent_id",
	"comments.depth",
	"comments.no_replies",
	"comments.no_replies_direct",
	"comments.ancestors",
	"comments.body",
	"comments.upvotes",
	"comments.downvotes",
	"comments.points",
	"comments.created_at",
	"comments.edited_at",
	"comments.deleted_at",
	"comments.deleted_as",
}

func buildSelectCommentsQuery(loggedIn bool, where string) string {
	columns := append([]string(nil), commentSelectColumns...)
	var joins []string
	if loggedIn {
		columns = append(columns, "comment_votes.id IS NOT NULL", "comment_votes.up")
		joins = []string{"LEFT OUTER JOIN comment_votes ON comments.id = comment_votes.comment_id AND comment_votes.user_id = ?"}
	}
	return msql.BuildSelectQuery("comments", columns, joins, where)
}

func selectCommentsQuery(ctx context.Context, db *gorm.DB, viewer *uid.ID) *gorm.DB {
	columns := append([]string(nil), commentSelectColumns...)
	joins := make([]idb.Join, 0, 1)
	if viewer != nil {
		columns = append(columns, "comment_votes.id IS NOT NULL", "comment_votes.up")
		joins = append(joins, idb.NewJoin("LEFT OUTER JOIN comment_votes ON comments.id = comment_votes.comment_id AND comment_votes.user_id = ?", *viewer))
	}
	return idb.Select(ctx, db, "comments", columns, joins...)
}

func toDBUIDList(ids []uid.ID) idb.UIDList {
	if len(ids) == 0 {
		return nil
	}
	list := make(idb.UIDList, len(ids))
	for i, item := range ids {
		list[i] = idb.UIDFrom(item)
	}
	return list
}

// Get comment returns a comment. If viewer is nil, viewer related fields of the
// comment (like Comment.ViewerVoted) will be nil.
func GetComment(ctx context.Context, db *gorm.DB, id uid.ID, viewer *uid.ID) (*Comment, error) {
	rows, err := selectCommentsQuery(ctx, db, viewer).Where("comments.id = ?", id).Rows()
	if err != nil {
		return nil, err
	}

	comments, err := scanComments(ctx, db, rows, viewer)
	if err != nil {
		return nil, fmt.Errorf("scanComments (id: %v): %w", id, err)
	}

	if len(comments) == 0 {
		return nil, errCommentNotFound
	}
	return comments[0], err
}

func GetCommentsByIDs(ctx context.Context, db *gorm.DB, viewer *uid.ID, ids ...uid.ID) ([]*Comment, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := selectCommentsQuery(ctx, db, viewer).Where("comments.id IN ?", ids).Rows()
	if err != nil {
		return nil, err
	}
	return scanComments(ctx, db, rows, viewer)
}

func scanComments(ctx context.Context, db *gorm.DB, rows *sql.Rows, viewer *uid.ID) ([]*Comment, error) {
	defer rows.Close()

	loggedIn := viewer != nil
	viewerAdmin, err := IsAdmin(db, viewer)
	if err != nil {
		return nil, err
	}

	var comments []*Comment
	for rows.Next() {
		comment := &Comment{}
		var ancestors []byte
		dest := []interface{}{
			&comment.ID,
			&comment.PostID,
			&comment.PostPublicID,
			&comment.CommunityID,
			&comment.CommunityName,
			&comment.AuthorID,
			&comment.AuthorUsername,
			&comment.PostedAs,
			&comment.AuthorDeleted,
			&comment.ParentID,
			&comment.Depth,
			&comment.NumReplies,
			&comment.NumRepliesDirect,
			&ancestors,
			&comment.Body,
			&comment.Upvotes,
			&comment.Downvotes,
			&comment.Points,
			&comment.CreatedAt,
			&comment.EditedAt,
			&comment.DeletedAt,
			&comment.DeletedAs,
		}
		if loggedIn {
			dest = append(dest, &comment.ViewerVoted, &comment.ViewerVotedUp)
		}

		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}

		comment.Deleted = comment.DeletedAt.Valid
		if comment.Deleted {
			comment.setStrippedContent(false)
		}

		if ancestors != nil {
			if err := json.Unmarshal(ancestors, &comment.Ancestors); err != nil {
				return nil, err
			}
		}

		comments = append(comments, comment)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(comments) == 0 {
		return nil, errCommentNotFound
	}

	if loggedIn {
		mutes, err := GetMutedUsers(ctx, db, *viewer, false)
		if err != nil {
			return nil, err
		}
		for _, comment := range comments {
			for _, mute := range mutes {
				if *mute.MutedUserID == comment.AuthorID && (!comment.Deleted || viewerAdmin) {
					comment.IsAuthorMuted = true
					break
				}
			}
		}
	}

	if err := populateCommentAuthors(ctx, db, comments, viewerAdmin); err != nil {
		return nil, fmt.Errorf("failed to populate comments authors: %w", err)
	}

	// If a comment is deleted and the viewer doesn't have the privilege to see
	// it, strip the comment's values that relate to its author in any way.
	if viewer != nil {
		if !viewerAdmin {
			viewerModOf := make(map[uid.ID]bool) // keys are community ids
			for _, comment := range comments {
				if comment.Deleted && comment.DeletedAs == UserGroupMods {
					viewerMod, ok := viewerModOf[comment.CommunityID]
					if !ok {
						var err error
						viewerMod, err = UserMod(ctx, db, comment.CommunityID, *viewer)
						if err != nil {
							return nil, err
						}
						viewerModOf[comment.CommunityID] = viewerMod
					}
					if !viewerMod {
						comment.StripContent()
					}
				} else {
					comment.StripContent()
				}
			}
		}
	} else {
		for _, comment := range comments {
			comment.StripContent()
		}
	}

	// Strip deleted author information, unless the viewer is an admin.
	for _, comment := range comments {
		if comment.AuthorDeleted {
			comment.setGhostAuthorID()
			if !viewerAdmin {
				comment.StripAuthorInfo()
			}
		}
	}

	return comments, nil
}

// addComment adds a record to the comments table. It does not check if the post
// is deleted or locked.
func addComment(ctx context.Context, db *gorm.DB, post *Post, author *User, parentID *uid.ID, commentBody string) (*Comment, error) {
	commentBody = utils.TruncateUnicodeString(commentBody, maxCommentBodyLength)
	var (
		parent    *Comment
		err       error
		ancestors []uid.ID
	)

	if parentID != nil {
		parent, err = GetComment(ctx, db, *parentID, nil)
		if err != nil {
			return nil, err
		}
		if parent.Deleted {
			return nil, httperr.NewBadRequest("comment-reply-to-deleted", "Cannot reply to a deleted comment.")
		}
		if parent.Depth == maxCommentDepth {
			return nil, httperr.NewBadRequest("comment-max-depth-reached", "Cannot reply because match depth is reached.")
		}
		ancestors = parent.Ancestors
		ancestors = append(ancestors, parent.ID)
	}

	id := uid.New()
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		depth, newParentID := 0, uid.NullID{}
		if parent != nil {
			newParentID.Valid, newParentID.ID = true, parent.ID
			depth = parent.Depth + 1
		}
		now := time.Now()
		commentModel := &idb.Comment{
			ID:            idb.UIDFrom(id),
			PostID:        idb.UIDFrom(post.ID),
			PostPublicID:  post.PublicID,
			CommunityID:   idb.UIDFrom(post.CommunityID),
			CommunityName: post.CommunityName,
			UserID:        idb.UIDFrom(author.ID),
			Username:      author.Username,
			UserGroup:     int8(UserGroupNormal),
			Depth:         uint8(depth),
			NoReplies:     0,
			Body:          &commentBody,
			CreatedAt:     now,
		}
		if newParentID.Valid {
			parentDBID := idb.UIDFrom(newParentID.ID)
			commentModel.ParentID = &parentDBID
		}
		if len(ancestors) > 0 {
			commentModel.Ancestors = toDBUIDList(ancestors)
		}
		if err := tx.Create(commentModel).Error; err != nil {
			return err
		}
		if err := tx.Model(&idb.Post{}).
			Where("id = ?", post.ID).
			Updates(map[string]any{
				"no_comments":      gorm.Expr("no_comments + 1"),
				"last_activity_at": now,
			}).
			Error; err != nil {
			return err
		}

		if parent != nil {
			if err := tx.Model(&idb.Comment{}).
				Where("id = ?", parent.ID).
				Update("no_replies_direct", gorm.Expr("no_replies_direct + 1")).
				Error; err != nil {
				return err
			}
			if err := tx.Model(&idb.Comment{}).
				Where("id IN ?", ancestors).
				Update("no_replies", gorm.Expr("no_replies + 1")).
				Error; err != nil {
				return err
			}
		}
		if err := tx.Create(&idb.PostsComment{
			TargetID:   idb.UIDFrom(id),
			TargetType: int8(ContentTypeComment),
			UserID:     idb.UIDFrom(author.ID),
		}).Error; err != nil {
			return err
		}
		if len(ancestors) > 0 {
			replies := make([]idb.CommentReply, len(ancestors))
			for i, ancestor := range ancestors {
				replies[i] = idb.CommentReply{
					ParentID: idb.UIDFrom(ancestor),
					ReplyID:  idb.UIDFrom(id),
				}
			}
			if err := tx.Create(&replies).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&idb.User{}).
			Where("id = ?", author.ID).
			Update("no_comments", gorm.Expr("no_comments + 1")).
			Error; err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Send notifications.
	if parent != nil && !parent.AuthorID.EqualsTo(author.ID) {
		go func() {
			if err := CreateCommentReplyNotification(context.Background(), db, parent.AuthorID, parent.ID, id, author, post); err != nil {
				log.Printf("Create reply notification failed: %v\n", err)
			}
		}()

	}
	if !post.AuthorID.EqualsTo(author.ID) && (parent == nil || !(parent.AuthorID.EqualsTo(post.AuthorID))) {
		go func() {
			if err := CreateNewCommentNotification(context.Background(), db, post, id, author); err != nil {
				log.Printf("Create new_comment notification failed: %v\n", err)
			}
		}()
	}

	return GetComment(ctx, db, id, nil)
}

// Save updates comment's body.
func (c *Comment) Save(ctx context.Context, db *gorm.DB, user uid.ID) error {
	if c.Deleted {
		return errCommentDeleted
	}
	if !c.AuthorID.EqualsTo(user) {
		return errNotAuthor
	}

	c.Body = utils.TruncateUnicodeString(c.Body, maxCommentBodyLength)

	now := time.Now()
	err := db.WithContext(ctx).
		Model(&idb.Comment{}).
		Where("id = ? AND deleted_at IS NULL", c.ID).
		Updates(map[string]any{
			"body":      c.Body,
			"edited_at": now,
		}).
		Error
	if err == nil {
		c.EditedAt.Valid = true
		c.EditedAt.Time = now
	}
	return err
}

// Delete returns an error if user, who's deleting the comment, has no
// permissions in his capacity as g to delete this comment.
func (c *Comment) Delete(ctx context.Context, db *gorm.DB, user uid.ID, g UserGroup) error {
	if c.Deleted {
		return errCommentDeleted
	}

	switch g {
	case UserGroupNormal:
		if !c.AuthorID.EqualsTo(user) {
			return errNotAuthor
		}
	case UserGroupMods:
		is, err := UserMod(ctx, db, c.CommunityID, user)
		if err != nil {
			return err
		}
		if !is {
			return errNotMod
		}
	case UserGroupAdmins:
		u, err := GetUser(ctx, db, user, nil)
		if err != nil {
			return err
		}
		if !u.Admin {
			return errNotAdmin
		}
	default:
		return errInvalidUserGroup
	}

	now := time.Now()
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var newBody string
		if g == UserGroupNormal {
			newBody = ""
		} else {
			newBody = c.Body
		}
		if err := tx.Model(&idb.Comment{}).
			Where("id = ?", c.ID).
			Updates(map[string]any{
				"body":       newBody,
				"deleted_at": now,
				"deleted_by": user,
				"deleted_as": g,
			}).
			Error; err != nil {
			return err
		}
		if g == UserGroupNormal {
			if err := tx.Where("target_id = ? AND user_id = ?", c.ID, c.AuthorID).Delete(&idb.PostsComment{}).Error; err != nil {
				return err
			}
		} else {
			if err := tx.Model(&idb.PostsComment{}).
				Where("target_id = ? AND user_id = ?", c.ID, c.AuthorID).
				Update("deleted", true).
				Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&idb.User{}).
			Where("id = ?", c.AuthorID).
			Update("no_comments", gorm.Expr("no_comments - 1")).
			Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	c.DeletedAt = msql.NewNullTime(now)
	c.DeletedBy = uid.NullID{Valid: true, ID: user}
	c.DeletedAs = g
	c.StripContent()
	RemoveAllReportsOfComment(ctx, db, c.ID)
	return err
}

func (c *Comment) setStrippedContent(v bool) {
	if c.ContentStripped == nil {
		c.ContentStripped = new(bool)
	}
	*c.ContentStripped = v
}

// StripAuthorInfo should be called if the author account of the comment is
// deleted and the viewer is not an admin.
func (c *Comment) StripAuthorInfo() {
	c.setGhostAuthorID()
	c.AuthorID.Clear()
	c.AuthorUsername = "ghost"
	// c.Author, if it's non-nil, should already be set to the ghost user.
}

func (c *Comment) setGhostAuthorID() {
	if c.AuthorGhostID == "" {
		c.AuthorGhostID = CalcGhostUserID(c.AuthorID, c.PostID.String())
	}
}

// StripContent strips all content of c that is either user generated or relates
// to a user.
func (c *Comment) StripContent() {
	if !c.Deleted {
		return
	}
	c.setStrippedContent(true)
	c.AuthorID.Clear()
	c.AuthorUsername = "[Hidden]"
	c.PostedAs = UserGroupNaN
	c.Body = "[Deleted comment]"
	c.ViewerVoted.Valid = false
	c.ViewerVotedUp.Valid = false
	c.Author = nil
}

// Vote votes on comment (if the comment is not deleted or the post locked).
func (c *Comment) Vote(ctx context.Context, db *gorm.DB, user uid.ID, up bool, newUserPointsThreshold int, newUserAgeThreshold time.Duration) error {
	if c.Deleted {
		return errCommentDeleted
	}

	if is, err := IsPostLocked(ctx, db, c.PostID); err != nil {
		return err
	} else if is {
		return errPostLocked
	}

	point := 1
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		canUserIncrementPoints, err := userAllowedToIncrementPoints(ctx, tx, user, newUserPointsThreshold, newUserAgeThreshold)
		if err != nil {
			return err
		}
		if err := tx.Create(&idb.CommentVote{
			CommentID: idb.UIDFrom(c.ID),
			UserID:    idb.UIDFrom(user),
			Up:        up,
			IsUserNew: !canUserIncrementPoints,
		}).Error; err != nil {
			if msql.IsErrDuplicateErr(err) {
				return httperr.NewBadRequest("already-voted", "You've already voted on the comment.")
			}
			return err
		}
		updates := map[string]any{
			"points": gorm.Expr("points + ?", point),
		}
		if up {
			updates["upvotes"] = gorm.Expr("upvotes + 1")
		} else {
			point = -1
			updates["points"] = gorm.Expr("points + ?", point)
			updates["downvotes"] = gorm.Expr("downvotes + 1")
		}
		if err := tx.Model(&idb.Comment{}).Where("id = ?", c.ID).Updates(updates).Error; err != nil {
			return err
		}
		if up && !c.AuthorID.EqualsTo(user) && canUserIncrementPoints {
			if err := incrementUserPoints(ctx, tx, c.AuthorID, 1); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	if up {
		c.Upvotes++
	} else {
		c.Downvotes++
	}
	c.Points += point
	c.ViewerVoted = msql.NewNullBool(true)
	c.ViewerVotedUp.Valid = true
	c.ViewerVotedUp.Bool = up

	// Attempt to create a notification (only for upvotes).
	if !c.AuthorID.EqualsTo(user) && up {
		go func() {
			if err := CreateNewVotesNotification(context.Background(), db, c.AuthorID, c.CommunityName, false, c.ID); err != nil {
				log.Printf("Failed creating new_votes notification: %v\n", err)
			}
		}()
	}

	return nil
}

// DeleteVote returns an error is the comment is deleted or the post locked.
func (c *Comment) DeleteVote(ctx context.Context, db *gorm.DB, user uid.ID) error {
	if c.Deleted {
		return errCommentDeleted
	}

	// Cannot vote if the post is locked.
	if is, err := IsPostLocked(ctx, db, c.PostID); err != nil {
		return err
	} else if is {
		return errPostLocked
	}

	var (
		voteID  uint64
		up      = false
		userNew = false
		point   = 1
	)
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		vote := idb.CommentVote{}
		if err := tx.Select("id", "up", "is_user_new").Where("comment_id = ? AND user_id = ?", c.ID, user).Take(&vote).Error; err != nil {
			return err
		}
		voteID = vote.ID
		up = vote.Up
		userNew = vote.IsUserNew
		if err := tx.Delete(&idb.CommentVote{}, voteID).Error; err != nil {
			return err
		}
		updates := map[string]any{}
		if up {
			point = -1
			updates["upvotes"] = gorm.Expr("upvotes - 1")
		} else {
			updates["downvotes"] = gorm.Expr("downvotes - 1")
		}
		updates["points"] = gorm.Expr("points + ?", point)
		if err := tx.Model(&idb.Comment{}).Where("id = ?", c.ID).Updates(updates).Error; err != nil {
			return err
		}
		if up && !c.AuthorID.EqualsTo(user) && !userNew {
			if err := incrementUserPoints(ctx, tx, c.AuthorID, -1); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	if up {
		c.Upvotes--
	} else {
		c.Downvotes--
	}
	c.Points += point
	c.ViewerVoted.Valid = false
	c.ViewerVotedUp.Valid = false

	return nil
}

// ChangeVote returns an error is the comment is deleted or the post locked.
func (c *Comment) ChangeVote(ctx context.Context, db *gorm.DB, user uid.ID, up bool) error {
	if c.Deleted {
		return errCommentDeleted
	}

	// Cannot vote if the post is locked.
	if is, err := IsPostLocked(ctx, db, c.PostID); err != nil {
		return err
	} else if is {
		return errPostLocked
	}

	var (
		voteID  uint64
		dbUp    = false
		userNew = false
		points  = 2
		exit    = false // if true, exit clean after the transaction
	)
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		vote := idb.CommentVote{}
		if err := tx.Select("id", "up", "is_user_new").Where("comment_id = ? AND user_id = ?", c.ID, user).Take(&vote).Error; err != nil {
			return err
		}
		voteID = vote.ID
		dbUp = vote.Up
		userNew = vote.IsUserNew
		if dbUp == up {
			exit = true
			return nil
		}
		if err := tx.Model(&idb.CommentVote{}).Where("id = ?", voteID).Update("up", up).Error; err != nil {
			return err
		}
		updates := map[string]any{}
		if dbUp {
			points = -2
			updates["upvotes"] = gorm.Expr("upvotes - 1")
			updates["downvotes"] = gorm.Expr("downvotes + 1")
		} else {
			updates["upvotes"] = gorm.Expr("upvotes + 1")
			updates["downvotes"] = gorm.Expr("downvotes - 1")
		}
		updates["points"] = gorm.Expr("points + ?", points)
		if err := tx.Model(&idb.Comment{}).Where("id = ?", c.ID).Updates(updates).Error; err != nil {
			return err
		}
		if !c.AuthorID.EqualsTo(user) && !userNew {
			points := 1
			if dbUp {
				points = -1
			}
			if err := incrementUserPoints(ctx, tx, c.AuthorID, points); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if exit {
		return nil
	}

	if dbUp {
		c.Upvotes--
		c.Downvotes++
	} else {
		c.Upvotes++
		c.Downvotes--
	}
	c.Points += points
	c.ViewerVotedUp = msql.NewNullBool(up)

	return nil
}

// ChangeUserGroup changes the capacity in which the comment's author added the
// post.
func (c *Comment) ChangeUserGroup(ctx context.Context, db *gorm.DB, author uid.ID, g UserGroup) error {
	if !c.AuthorID.EqualsTo(author) {
		return errNotAuthor
	}

	if c.PostedAs == g {
		return nil
	}

	switch g {
	case UserGroupNormal:
	case UserGroupMods:
		is, err := UserMod(ctx, db, c.CommunityID, author)
		if err != nil {
			return err
		}
		if !is {
			return errNotMod
		}
	case UserGroupAdmins:
		u, err := GetUser(ctx, db, author, nil)
		if err != nil {
			return err
		}
		if !u.Admin {
			return errNotAdmin
		}
	default:
		return errInvalidUserGroup
	}

	err := db.WithContext(ctx).
		Model(&idb.Comment{}).
		Where("id = ? AND deleted_at IS NULL", c.ID).
		Update("user_group", g).
		Error
	if err == nil {
		c.PostedAs = g
	}
	return err
}

// loadPostDeleted populates c.PostDeleted.
func (c *Comment) loadPostDeleted(ctx context.Context, db *gorm.DB) error {
	var post struct {
		DeletedAt msql.NullTime
		DeletedAs UserGroup
	}
	err := db.WithContext(ctx).
		Table("posts").
		Select("deleted_at", "deleted_as").
		Where("id = ?", c.PostID).
		Take(&post).
		Error
	if err == nil {
		c.PostDeletedAs = post.DeletedAs
		if post.DeletedAt.Valid {
			c.PostDeleted = true
		}
	}
	return err
}

// populateCommentAuthors populates the Author field of each comment of comments
// (except for deleted comments).
func populateCommentAuthors(ctx context.Context, db *gorm.DB, comments []*Comment, viewerAdmin bool) error {
	var authorIDs []uid.ID
	found := make(map[uid.ID]bool)
	for _, comment := range comments {
		if !found[comment.AuthorID] {
			authorIDs = append(authorIDs, comment.AuthorID)
			found[comment.AuthorID] = true
		}
	}

	if len(authorIDs) == 0 {
		return nil
	}

	authors, err := GetUsersByIDs(ctx, db, authorIDs, nil)
	if err != nil {
		return err
	}

	if !viewerAdmin {
		// If the author account is deleted, some of it's values are set to
		// ghost values, including the ID of the author. Undo this so that the
		// authors can be matched with the comments.
		for _, author := range authors {
			author.UnsetToGhost()
		}
	}

	for _, comment := range comments {
		found := true
		for _, author := range authors {
			if comment.AuthorID == author.ID {
				comment.Author = author
				break
			}
		}
		if !found {
			panic("author not found")
		}

	}

	if !viewerAdmin {
		// Reset deleted authors to ghosts.
		for _, author := range authors {
			author.SetToGhost()
		}
	}

	return nil
}

// GetSiteComments returns a cursor-paginated response of all comments of the site.
func GetSiteComments(ctx context.Context, db *gorm.DB, limit int, next *string, viewer *uid.ID) ([]*Comment, *string, error) {
	where, args := "", []any{}
	if next != nil {
		nextID, err := uid.FromString(*next)
		if err != nil {
			return nil, nil, errors.New("invalid next for site comments")
		}
		where = "WHERE comments.id <= ? "
		args = append(args, nextID)
	}

	where += "ORDER BY comments.id DESC LIMIT ? "
	args = append(args, limit+1)

	comments, err := getComments(ctx, db, viewer, where, args...)
	if err != nil {
		return nil, nil, err
	}

	var nextNext *string
	if len(comments) >= limit+1 {
		nextNext = new(string)
		*nextNext = comments[limit].ID.String()
		comments = comments[:limit]
	}

	if err := getCommentsPostTitles(ctx, db, comments, nil); err != nil {
		return nil, nil, err
	}

	return comments, nextNext, nil
}
