package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	idb "github.com/discuitnet/discuit/internal/db"
	"github.com/discuitnet/discuit/internal/httperr"
	msql "github.com/discuitnet/discuit/internal/sql"
	"github.com/discuitnet/discuit/internal/uid"
	"github.com/discuitnet/discuit/internal/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ListItemsSort int

const (
	ListItemsSortByAddedDsc = ListItemsSort(iota)
	ListItemsSortByAddedAsc
	ListItemsSortByCreatedDesc
	ListItemsSortByCreatedAsc

	ListOrderingDefault = ListItemsSortByAddedDsc
)

func (o ListItemsSort) String() string {
	switch o {
	case ListItemsSortByAddedDsc:
		return "addedDsc"
	case ListItemsSortByAddedAsc:
		return "addedAsc"
	case ListItemsSortByCreatedDesc:
		return "createdDsc"
	case ListItemsSortByCreatedAsc:
		return "createdAsc"
	}
	return "" // Unsupported list sort.
}

// MarshalText implements the text.Marshaler interface. It returns an
// httperr.Error (bad request) on error.
func (o ListItemsSort) MarshalText() ([]byte, error) {
	text := o.String()
	if text == "" {
		return nil, httperr.NewBadRequest("invalid-list-sort", "Invalid list sort.")
	}
	return []byte(text), nil
}

// UnmarshalText implements the text.Unmarshaler interface. It returns an
// httperr.Error (bad request) on error.
func (o *ListItemsSort) UnmarshalText(data []byte) error {
	switch string(data) {
	case "addedDsc":
		*o = ListItemsSortByAddedDsc
	case "addedAsc":
		*o = ListItemsSortByAddedAsc
	case "createdDsc":
		*o = ListItemsSortByCreatedDesc
	case "createdAsc":
		*o = ListItemsSortByCreatedAsc
	}
	return httperr.NewBadRequest("invalid-list-sort", "Invalid list sort.")
}

type List struct {
	ID            int             `json:"id"`
	UserID        uid.ID          `json:"userId"`
	Username      string          `json:"username"`
	Name          string          `json:"name"`
	DisplayName   string          `json:"displayName"`
	Description   msql.NullString `json:"description"`
	Public        bool            `json:"public"`
	NumItmes      int             `json:"numItems"`
	Sort          ListItemsSort   `json:"sort"` // current sort
	CreatedAt     time.Time       `json:"createdAt"`
	LastUpdatedAt time.Time       `json:"lastUpdatedAt"`
}

func getLists(ctx context.Context, query *gorm.DB) ([]*List, error) {
	rows, err := query.Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	lists := []*List{}
	for rows.Next() {
		list := &List{}
		err = rows.Scan(
			&list.ID,
			&list.UserID,
			&list.Username,
			&list.Name,
			&list.DisplayName,
			&list.Description,
			&list.Public,
			&list.NumItmes,
			&list.Sort,
			&list.CreatedAt,
			&list.LastUpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		lists = append(lists, list)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return lists, nil
}

func selectListsQuery(ctx context.Context, db *gorm.DB) *gorm.DB {
	return idb.Select(ctx, db, "lists", []string{
		"lists.id",
		"lists.user_id",
		"users.username",
		"lists.name",
		"lists.display_name",
		"lists.description",
		"lists.public",
		"lists.num_items",
		"lists.ordering",
		"lists.created_at",
		"lists.last_updated_at",
	}, idb.NewJoin("INNER JOIN users on lists.user_id = users.id"))
}

func GetList(ctx context.Context, db *gorm.DB, id int) (*List, error) {
	lists, err := getLists(ctx, selectListsQuery(ctx, db).Where("lists.id = ?", id))
	if err != nil {
		return nil, err
	}
	if len(lists) == 0 {
		return nil, httperr.NewNotFound("list-not-found", "List not found.")
	}
	return lists[0], nil
}

func GetListByName(ctx context.Context, db *gorm.DB, user uid.ID, name string) (*List, error) {
	lists, err := getLists(ctx, selectListsQuery(ctx, db).Where("user_id = ? AND lists.name = ?", user, name))
	if err != nil {
		return nil, err
	}
	if len(lists) == 0 {
		return nil, httperr.NewNotFound("list-not-found", "List not found.")
	}
	return lists[0], nil
}

// GetUsersLists returns all the lists of user. The argument sort has to be
// either empty or one of be one of: lexical, last_updated. And filter has to be
// either empty or one of: all, public, private.
func GetUsersLists(ctx context.Context, db *gorm.DB, user uid.ID, sort, filter string) ([]*List, error) {
	if sort == "" {
		sort = "lastAdded"
	}
	if filter == "" {
		filter = "all"
	}
	if !(sort == "name" || sort == "lastAdded") {
		return nil, httperr.NewBadRequest("invalid-lists-sort", "Invalid lists sort.")
	}
	if !(filter == "all" || filter == "public" || filter == "private") {
		return nil, httperr.NewBadRequest("invalid-lists-filter", "Invalid lists filter.")
	}

	query := selectListsQuery(ctx, db).Where("user_id = ?", user)
	if filter == "public" {
		query = query.Where("public = ?", true)
	} else if filter == "private" {
		query = query.Where("public = ?", false)
	}
	if sort == "name" {
		query = query.Order("name ASC")
	} else if sort == "lastAdded" {
		query = query.Order("last_updated_at DESC")
	}

	return getLists(ctx, query)
}

// listnameValid always returns an httperr.Error.
func listnameValid(name string) error {
	if err := IsUsernameValid(name); err != nil {
		return httperr.NewBadRequest("invalid-list-name", fmt.Sprintf("list name %v", err))
	}
	return nil
}

func truncateListDisplayName(s string) string {
	return utils.TruncateUnicodeString(s, 50)
}

func CreateList(ctx context.Context, db *gorm.DB, user uid.ID, name, displayName string, description msql.NullString, public bool) error {
	if description.String == "" {
		description.Valid = false
	}

	if err := listnameValid(name); err != nil {
		return err
	}

	displayName = truncateListDisplayName(displayName)

	description.String = utils.TruncateUnicodeString(description.String, maxUserProfileAboutLength)
	var descriptionValue *string
	if description.Valid {
		descriptionValue = &description.String
	}
	err := db.WithContext(ctx).Create(&idb.List{
		UserID:      idb.UIDFrom(user),
		Name:        name,
		DisplayName: displayName,
		Description: descriptionValue,
		Public:      public,
		Ordering:    int8(ListOrderingDefault),
	}).Error
	if err != nil && msql.IsErrDuplicateErr(err) {
		return &httperr.Error{
			HTTPStatus: http.StatusConflict,
			Code:       "duplicate-list",
			Message:    "A list with that name already exists.",
		}
	}
	return err
}

// Update updates the list's updatable fields.
func (l *List) Update(ctx context.Context, db *gorm.DB) error {
	// Check errors:
	if err := listnameValid(l.Name); err != nil {
		return err
	}

	// Truncate:
	l.Description.String = utils.TruncateUnicodeString(l.Description.String, maxUserProfileAboutLength)
	l.DisplayName = truncateListDisplayName(l.DisplayName)
	var descriptionValue *string
	if l.Description.Valid {
		descriptionValue = &l.Description.String
	}
	return db.WithContext(ctx).
		Model(&idb.List{}).
		Where("id = ?", l.ID).
		Updates(map[string]any{
			"name":         l.Name,
			"display_name": l.DisplayName,
			"description":  descriptionValue,
			"public":       l.Public,
			"ordering":     l.Sort,
		}).
		Error
}

// UnmarshalUpdatableFieldsJSON extracts the updatable values of the list from
// the encoded JSON string.
func (l *List) UnmarshalUpdatableFieldsJSON(data []byte) error {
	temp := *l // shallow copy
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}
	l.Name = temp.Name
	l.DisplayName = temp.DisplayName
	l.Description = temp.Description
	if l.Description.String == "" {
		l.Description.Valid = false
	}
	l.Public = temp.Public
	l.Sort = temp.Sort
	return nil
}

func (l *List) Delete(ctx context.Context, db *gorm.DB) error {
	return db.WithContext(ctx).Where("id = ?", l.ID).Delete(&idb.List{}).Error
}

func (l *List) AddItem(ctx context.Context, db *gorm.DB, targetType ContentType, targetID uid.ID) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		item := idb.ListItem{
			ListID:     uint64(l.ID),
			TargetType: int8(targetType),
			TargetID:   idb.UIDFrom(targetID),
		}
		create := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&item)
		if create.Error != nil {
			return create.Error
		}
		if create.RowsAffected == 0 {
			return nil
		}
		if err := tx.Model(&idb.List{}).
			Where("id = ?", l.ID).
			Updates(map[string]any{
				"num_items":       gorm.Expr("num_items + 1"),
				"last_updated_at": time.Now(),
			}).
			Error; err != nil {
			return err
		}
		return nil
	})
}

func (l *List) DeleteItem(ctx context.Context, db *gorm.DB, targetType ContentType, targetID uid.ID) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item idb.ListItem
		err := tx.Select("id").
			Where("list_id = ? AND target_id = ? AND target_type = ?", l.ID, targetID, targetType).
			Take(&item).
			Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		if err := tx.Where("id = ?", item.ID).Delete(&idb.ListItem{}).Error; err != nil {
			return err
		}
		if err := tx.Model(&idb.List{}).
			Where("id = ?", l.ID).
			Update("num_items", gorm.Expr("num_items - 1")).
			Error; err != nil {
			return err
		}
		return nil
	})
}

func (l *List) DeleteAllItems(ctx context.Context, db *gorm.DB) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("list_id = ?", l.ID).Delete(&idb.ListItem{}).Error; err != nil {
			return err
		}
		if err := tx.Model(&idb.List{}).Where("id = ?", l.ID).Update("num_items", 0).Error; err != nil {
			return err
		}
		return nil
	})
}

type ListItem struct {
	ID         int         `json:"id"`
	ListID     int         `json:"listId"`
	TargetType ContentType `json:"targetType"`
	TargetID   uid.ID      `json:"targetId"`
	CreatedAt  time.Time   `json:"createdAt"` // When the list item was created, not the target item.

	TargetItem any `json:"targetItem"` // Either a Post or a Comment.
}

func GetListItem(ctx context.Context, db *gorm.DB, listID, itemID int) (*ListItem, error) {
	rows, err := selectListItemsQuery(ctx, db).
		Where("id = ? AND list_id = ?", itemID, listID).
		Rows()
	if err != nil {
		return nil, err
	}

	items, err := scanListItems(rows, listID)
	if err != nil {
		return nil, err
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("list item not found for listID %d and itemID %d", listID, itemID)
	}

	return items[0], nil
}

// The next string should contain either a timestamp or a marshaled uid.ID; if
// not, the function will return an error. It's safe for next to be nil.
func GetListItems(ctx context.Context, db *gorm.DB, listID, limit int, sort ListItemsSort, next *string, viewer *uid.ID) (*ListItemsResultSet, error) {
	query := selectListItemsQuery(ctx, db).Where("list_id = ?", listID)

	// Parse the pagination cursor, if present.
	if next != nil {
		if sort == ListItemsSortByAddedAsc || sort == ListItemsSortByAddedDsc {
			// next should be a time.Time value.
			i, err := strconv.ParseInt(*next, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("failed to parse next (GetListItems, time): %w", err)
			}
			nextTime := time.Unix(i, 0)
			if sort == ListItemsSortByAddedAsc {
				query = query.Where("created_at >= ?", nextTime)
			} else {
				query = query.Where("created_at <= ?", nextTime)
			}
		} else {
			nextID, err := uid.FromString(*next)
			if err != nil {
				return nil, fmt.Errorf("failed to parse next (GetListItems, uid.ID): %w", err)
			}
			if sort == ListItemsSortByCreatedAsc {
				query = query.Where("target_id >= ?", nextID)
			} else {
				query = query.Where("target_id <= ?", nextID)
			}
		}
	}

	orderBy := ""
	switch sort {
	case ListItemsSortByAddedAsc:
		orderBy = "created_at ASC"
	case ListItemsSortByAddedDsc:
		orderBy = "created_at DESC"
	case ListItemsSortByCreatedAsc:
		orderBy = "target_id ASC"
	case ListItemsSortByCreatedDesc:
		orderBy = "target_id ASC"
	}
	query = query.Order(orderBy)

	if limit > 0 {
		query = query.Limit(limit + 1)
	}

	rows, err := query.Rows()
	if err != nil {
		return nil, err
	}

	items, err := scanListItems(rows, listID)
	if err != nil {
		return nil, err
	}

	// Return the result set.
	set := &ListItemsResultSet{}
	if len(items) > limit {
		lastItem := items[len(items)-1]
		set.Next = new(string)
		if sort == ListItemsSortByAddedAsc || sort == ListItemsSortByAddedDsc {
			// Sort by added at.
			*set.Next = strconv.FormatInt(lastItem.CreatedAt.Unix(), 10)
		} else {
			// Sort by target created at.
			*set.Next = lastItem.TargetID.String()
		}
		set.Items = items[:len(items)-1]
	} else {
		set.Items = items
	}

	// Fetch the posts and comments.
	var (
		postIDs         = make([]uid.ID, 0, len(set.Items))
		postItemsMap    = make(map[uid.ID]*ListItem, len(set.Items))
		commentIDs      = make([]uid.ID, 0, len(set.Items))
		commentItemsMap = make(map[uid.ID]*ListItem, len(set.Items))
	)
	for _, item := range set.Items {
		if item.TargetType == ContentTypePost {
			postIDs = append(postIDs, item.TargetID)
			postItemsMap[item.TargetID] = item
		} else if item.TargetType == ContentTypeComment {
			commentIDs = append(commentIDs, item.TargetID)
			commentItemsMap[item.TargetID] = item
		}
	}

	posts, err := GetPostsByIDs(ctx, db, viewer, true, postIDs...)
	if err != nil {
		return nil, err
	}
	for _, post := range posts {
		postItemsMap[post.ID].TargetItem = post
	}

	comments, err := GetCommentsByIDs(ctx, db, viewer, commentIDs...)
	if err != nil {
		return nil, err
	}
	for _, comment := range comments {
		commentItemsMap[comment.ID].TargetItem = comment
	}
	if len(comments) > 0 {
		if err := getCommentsPostTitles(ctx, db, comments, viewer); err != nil {
			return nil, err
		}
	}

	return set, nil
}

func (li *ListItem) Delete(ctx context.Context, db *gorm.DB) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Where("id = ?", li.ID).Delete(&idb.ListItem{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		if err := tx.Model(&idb.List{}).
			Where("id = ?", li.ListID).
			Update("num_items", gorm.Expr("num_items - 1")).
			Error; err != nil {
			return err
		}
		return nil
	})
}

func selectListItemsQuery(ctx context.Context, db *gorm.DB) *gorm.DB {
	return idb.Select(ctx, db, "list_items", []string{"id", "target_type", "target_id", "created_at"})
}

func scanListItems(rows *sql.Rows, listID int) ([]*ListItem, error) {
	defer rows.Close()

	items := []*ListItem{}
	for rows.Next() {
		item := &ListItem{ListID: listID}
		err := rows.Scan(
			&item.ID,
			&item.TargetType,
			&item.TargetID,
			&item.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

type ListItemsResultSet struct {
	Items []*ListItem `json:"items"`
	Next  *string     `json:"next"` // either a timestamp or a uid.ID.
}

// ListsItemIsSavedTo returns the ids of the lists the post or comment target is
// saved in.
func ListsItemIsSavedTo(ctx context.Context, db *gorm.DB, user uid.ID, targetID uid.ID, targetType ContentType) ([]int, error) {
	_ = user
	rows, err := idb.Select(
		ctx,
		db,
		"list_items",
		[]string{"lists.id"},
		idb.NewJoin("INNER JOIN lists on lists.id = list_items.list_id"),
	).
		Where("list_items.target_id = ? AND list_items.target_type = ?", targetID, targetType).
		Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := []int{} // So, it's JSON marshaled as an array.
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return ids, nil
}
