package core

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	idb "github.com/discuitnet/discuit/internal/db"
	"github.com/discuitnet/discuit/internal/httperr"
	"github.com/discuitnet/discuit/internal/images"
	msql "github.com/discuitnet/discuit/internal/sql"
	"github.com/discuitnet/discuit/internal/uid"
	"github.com/discuitnet/discuit/internal/utils"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	// For usernames and community names.
	maxUsernameLength = 21
	minUsernameLength = 3

	maxPasswordLength = 72 // in bytes (limit set by bcrypt)

	maxUserProfileAboutLength = 10000
	maxHiddenPosts            = 1000 // per user
)

// UserGroup represents who a user is.
type UserGroup int

const (
	UserGroupNaN = UserGroup(iota) // Psuedo user group.
	UserGroupNormal
	UserGroupAdmins
	UserGroupMods
)

func (u UserGroup) Valid() bool {
	_, err := u.MarshalText()
	return err == nil
}

// String implements fmt.Stringer interface. It returns the value returned by
// u.MarshalText (if u.MarshalText returns an error, it returns "[error]").
func (u UserGroup) String() string {
	b, err := u.MarshalText()
	if err != nil {
		return "[error]"
	}
	return string(b)
}

// MarshalText implements encoding.TextMarshaler interface.
func (u UserGroup) MarshalText() ([]byte, error) {
	s := ""
	switch u {
	case UserGroupNaN:
		s = "null"
	case UserGroupNormal:
		s = "normal"
	case UserGroupAdmins:
		s = "admins"
	case UserGroupMods:
		s = "mods"
	default:
		return nil, errInvalidUserGroup
	}
	return []byte(s), nil
}

// UnmarshalText implements encoding.TextUnmarshaler interface.
func (u *UserGroup) UnmarshalText(text []byte) error {
	switch string(text) {
	case "null":
		*u = UserGroupNaN
	case "normal":
		*u = UserGroupNormal
	case "admins":
		*u = UserGroupAdmins
	case "mods":
		*u = UserGroupMods
	default:
		return errInvalidUserGroup
	}
	return nil
}

type User struct {
	ID                uid.ID `json:"id"`
	UserIndex         int    `json:"-"`
	Username          string `json:"username"`
	UsernameLowerCase string `json:"-"`

	EmailPublic *string `json:"email"`

	// Additional admin-only viewable fields
	LastSeenAdminView    *time.Time   `json:"lastSeen,omitempty"`
	LastSeeenIPAdminView *msql.NullIP `json:"lastSeenIP,omitempty"`
	CreatedIPAdminView   *msql.NullIP `json:"createdIP,omitempty"`

	Email            msql.NullString `json:"-"`
	EmailConfirmedAt msql.NullTime   `json:"emailConfirmedAt"`
	Password         string          `json:"-"`
	About            msql.NullString `json:"aboutMe"`
	Points           int             `json:"points"`
	Admin            bool            `json:"isAdmin"`
	ProPic           *images.Image   `json:"proPic"`
	Badges           Badges          `json:"badges"`
	NumPosts         int             `json:"noPosts"`
	NumComments      int             `json:"noComments"`
	LastSeen         time.Time       `json:"-"`             // accurate to within 5 minutes
	LastSeenMonth    string          `json:"lastSeenMonth"` // of the form: November 2024
	LastSeenIP       msql.NullIP     `json:"-"`
	CreatedAt        time.Time       `json:"createdAt"`
	CreatedIP        msql.NullIP     `json:"-"`
	Deleted          bool            `json:"deleted"`
	DeletedAt        msql.NullTime   `json:"deletedAt,omitempty"`

	// User preferences.
	UpvoteNotificationsOff  bool     `json:"upvoteNotificationsOff"`
	ReplyNotificationsOff   bool     `json:"replyNotificationsOff"`
	HomeFeed                FeedType `json:"homeFeed"`
	RememberFeedSort        bool     `json:"rememberFeedSort"`
	EmbedsOff               bool     `json:"embedsOff"`
	HideUserProfilePictures bool     `json:"hideUserProfilePictures"`
	RequireAltText          bool     `json:"requireAltText"`

	WelcomeNotificationSent bool `json:"-"`

	// No banned users are supposed to be logged in. Make sure to log them out
	// before banning.
	BannedAt msql.NullTime `json:"bannedAt"`
	Banned   bool          `json:"isBanned"`

	MutedByViewer bool `json:"-"`

	NumNewNotifications int `json:"notificationsNewCount"`

	// The list of communities the user moderates.
	ModdingList []*Community `json:"moddingList"`

	// The following values are used only by SetToGhost and UnsetToGhost
	// methods.
	preGhostUsername  string
	preGhostID        uid.ID
	preGhostCreatedAt time.Time
	preGhostDeletedAt msql.NullTime
	preGhostBadges    Badges
}

// IsUsernameValid returns nil if name only consists
// of valid (0-9, A-Z, a-z, and _) characters and
// if it's of acceptable length.
func IsUsernameValid(name string) error {
	runes := [...]rune{
		'0', '1', '2', '3', '4', '5', '6', '7', '8', '9',
		'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j',
		'k', 'l', 'm', 'n', 'o', 'p', 'q', 'r', 's', 't',
		'u', 'v', 'w', 'x', 'y', 'z', 'A', 'B', 'C', 'D',
		'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N',
		'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X',
		'Y', 'Z', '_',
	}

	n := len(name)
	if n == 0 {
		return errors.New("is empty")
	}
	if n < minUsernameLength {
		return errors.New("is too short")
	}
	if n > maxUsernameLength {
		return errors.New("is too long")
	}

	for _, r := range name {
		match := false
		for _, r2 := range runes {
			if r == r2 {
				match = true
				break
			}
		}
		if !match {
			return errors.New("contains disallowed characters")
		}
	}
	return nil
}

func trimPassword(password []byte) []byte {
	if len(password) > maxPasswordLength {
		password = password[:maxPasswordLength]
	}
	return password
}

// HashPassword returns the hashed password if the password is acceptable,
// otherwise it returns an httperr.Error.
func HashPassword(password []byte) ([]byte, error) {
	if len(password) == 0 {
		return nil, httperr.NewBadRequest("invalid-password", "Password empty.")
	}

	password = trimPassword(password)
	hash, err := bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("error hashing password: %w", err)
	}
	return hash, nil
}

var selectUserCols = append([]string{
	"users.id",
	"users.user_index",
	"users.username",
	"users.username_lc",
	"users.email",
	"users.email_confirmed_at",
	"users.password",
	"users.about_me",
	"users.points",
	"users.is_admin",
	"users.no_posts",
	"users.no_comments",
	"users.notifications_new_count",
	"users.last_seen",
	"users.last_seen_ip",
	"users.created_at",
	"users.created_ip",
	"users.deleted_at",
	"users.banned_at",
	"users.upvote_notifications_off",
	"users.reply_notifications_off",
	"users.home_feed",
	"users.remember_feed_sort",
	"users.embeds_off",
	"users.hide_user_profile_pictures",
	"users.welcome_notification_sent",
	"users.require_alt_text",
}, images.ImageColumns("pro_pic")...)

var selectUserJoins = []idb.Join{
	idb.NewJoin("LEFT JOIN images AS pro_pic ON pro_pic.id = users.pro_pic"),
}

func buildSelectUserQuery(ctx context.Context, db *gorm.DB) *gorm.DB {
	return idb.Select(ctx, db, "users", selectUserCols, selectUserJoins...)
}

func GetUser(ctx context.Context, db *gorm.DB, user uid.ID, viewer *uid.ID) (*User, error) {
	rows, err := buildSelectUserQuery(ctx, db).Where("users.id = ?", user).Rows()
	if err != nil {
		return nil, err
	}

	users, err := scanUsers(ctx, db, rows, viewer)
	if err != nil {
		return nil, err
	}
	return users[0], err
}

func GetUsersByIDs(ctx context.Context, db *gorm.DB, IDs []uid.ID, viewer *uid.ID) ([]*User, error) {
	if len(IDs) == 0 {
		return nil, nil
	}
	rows, err := buildSelectUserQuery(ctx, db).Where("users.id IN ?", IDs).Rows()
	if err != nil {
		return nil, err
	}
	users, err := scanUsers(ctx, db, rows, viewer)
	if err != nil {
		return nil, err
	}
	return users, nil
}

func GetUsersByUsernames(ctx context.Context, db *gorm.DB, usernames []string, viewer *uid.ID) ([]*User, error) {
	if len(usernames) == 0 {
		return nil, nil
	}
	rows, err := buildSelectUserQuery(ctx, db).Where("users.username IN ?", usernames).Rows()
	if err != nil {
		return nil, err
	}
	users, err := scanUsers(ctx, db, rows, viewer)
	if err != nil {
		return nil, err
	}
	return users, nil
}

func GetUserByUsername(ctx context.Context, db *gorm.DB, username string, viewer *uid.ID) (*User, error) {
	rows, err := buildSelectUserQuery(ctx, db).Where("users.username_lc = ?", strings.ToLower(username)).Rows()
	if err != nil {
		return nil, err
	}
	users, err := scanUsers(ctx, db, rows, viewer)
	if err != nil {
		return nil, err
	}
	return users[0], err
}

func GetUserByEmail(ctx context.Context, db *gorm.DB, email string, viewer *uid.ID) (*User, error) {
	rows, err := buildSelectUserQuery(ctx, db).Where("users.email = ?", email).Rows()
	if err != nil {
		return nil, err
	}
	users, err := scanUsers(ctx, db, rows, viewer)
	if err != nil {
		return nil, err
	}
	return users[0], err
}

// scanUsers returns errUserNotFound if no rows can be found.
func scanUsers(ctx context.Context, db *gorm.DB, rows *sql.Rows, viewer *uid.ID) ([]*User, error) {
	defer rows.Close()

	var users []*User
	for rows.Next() {
		u := &User{
			Badges: make(Badges, 0),
		}
		dests := []any{
			&u.ID,
			&u.UserIndex,
			&u.Username,
			&u.UsernameLowerCase,
			&u.Email,
			&u.EmailConfirmedAt,
			&u.Password,
			&u.About,
			&u.Points,
			&u.Admin,
			&u.NumPosts,
			&u.NumComments,
			&u.NumNewNotifications,
			&u.LastSeen,
			&u.LastSeenIP,
			&u.CreatedAt,
			&u.CreatedIP,
			&u.DeletedAt,
			&u.BannedAt,
			&u.UpvoteNotificationsOff,
			&u.ReplyNotificationsOff,
			&u.HomeFeed,
			&u.RememberFeedSort,
			&u.EmbedsOff,
			&u.HideUserProfilePictures,
			&u.WelcomeNotificationSent,
			&u.RequireAltText,
		}

		proPic := &images.Image{}
		dests = append(dests, proPic.ScanDestinations()...)

		if err := rows.Scan(dests...); err != nil {
			return nil, err
		}

		u.Deleted = u.DeletedAt.Valid
		u.preGhostUsername = u.Username
		u.preGhostID = u.ID
		u.preGhostCreatedAt = u.CreatedAt
		u.preGhostDeletedAt = u.DeletedAt
		u.preGhostBadges = u.Badges

		if u.BannedAt.Valid {
			u.Banned = true
		}

		if proPic.ID != nil {
			proPic.PostScan()
			setCommunityProPicCopies(proPic)
			u.ProPic = proPic
		}

		users = append(users, u)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(users) == 0 {
		return nil, errUserNotFound
	}

	if viewer != nil {
		mutes, err := GetMutedUsers(ctx, db, *viewer, false)
		if err != nil {
			return nil, err
		}
		for _, user := range users {
			for _, mute := range mutes {
				if *mute.MutedUserID == user.ID {
					user.MutedByViewer = true
					break
				}
			}
		}
	}

	if err := fetchBadges(db, users...); err != nil {
		return nil, fmt.Errorf("fetching badges: %w", err)
	}

	viewerAdmin, err := IsAdmin(db, viewer)
	if err != nil {
		return nil, err
	}

	for _, user := range users {
		// Hide everything that only the user themself or an admin should see
		// from public view.
		if viewerAdmin || (viewer != nil && *viewer == user.ID) {
			if user.Email.Valid {
				user.EmailPublic = new(string)
				*user.EmailPublic = user.Email.String
			}
		}
		if viewerAdmin {
			user.LastSeenAdminView = &user.LastSeen
			user.LastSeeenIPAdminView = &user.LastSeenIP
			user.CreatedIPAdminView = &user.CreatedIP
		}
		// Set the user info of deleted users to the ghost user for everyone
		// except the admins.
		if user.Deleted && !viewerAdmin {
			user.SetToGhost()
		}

		user.LastSeenMonth = user.LastSeen.Month().String() + " " + strconv.Itoa(user.LastSeen.Year())
	}

	return users, nil
}

// RegisterUser creates a new user.
func RegisterUser(ctx context.Context, db *gorm.DB, username, email, password, ip string) (*User, error) {
	// Check for duplicates.
	if exists, _, err := usernameExists(ctx, db, username); err != nil {
		return nil, err
	} else if exists {
		return nil, &httperr.Error{
			HTTPStatus: http.StatusConflict,
			Code:       "user_exists",
			Message:    fmt.Sprintf("A user with username %s already exists.", username),
		}
	}

	// Check if username is valid.
	if err := IsUsernameValid(username); err != nil {
		return nil, httperr.NewBadRequest("invalid-username", fmt.Sprintf("Username %v.", err))
	}

	hash, err := HashPassword([]byte(password))
	if err != nil {
		return nil, err
	}

	// Note: Thet email address is not checked to be a valid email address. Any
	// string can be stored as an email address currently.
	nullEmail := msql.NullString{}
	if email != "" {
		nullEmail.Valid = true
		nullEmail.String = email
	}

	var ipany any
	if ip != "" {
		ipany = ip
	}
	id := uid.New()

	record := &idb.User{
		ID:         idb.UIDFrom(id),
		Username:   username,
		UsernameLC: strings.ToLower(username),
		Password:   string(hash),
	}
	if nullEmail.Valid {
		record.Email = &nullEmail.String
	}
	if ipStr, ok := ipany.(string); ok {
		ipValue := idb.IPFrom(net.ParseIP(ipStr))
		record.CreatedIP = &ipValue
	}
	err = db.WithContext(ctx).Create(record).Error
	if err != nil {
		return nil, err
	}

	if err := addUserToDefaultCommunities(ctx, db, id); err != nil {
		log.Println("Failed to add user to default communities: ", err)
		// Continue on failure.
	}

	if err := CreateList(ctx, db, id, "bookmarks", "Bookmarks", msql.NullString{}, false); err != nil {
		log.Println("Failed to create the default community of user: ", username)
		// Continue on failure.
	}

	return GetUser(ctx, db, id, nil)
}

func addUserToDefaultCommunities(ctx context.Context, db *gorm.DB, user uid.ID) error {
	rows, err := idb.Select(ctx, db, "communities", []string{"communities.id"},
		idb.NewJoin("INNER JOIN default_communities ON communities.name_lc = default_communities.name_lc"),
	).Rows()
	if err != nil {
		return err
	}
	defer rows.Close()

	var communities []uid.ID
	for rows.Next() {
		var id uid.ID
		if err = rows.Scan(&id); err != nil {
			return err
		}
		communities = append(communities, id)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	if len(communities) == 0 {
		return nil
	}

	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		rows := make([]idb.CommunityMember, 0, len(communities))
		for _, communityID := range communities {
			rows = append(rows, idb.CommunityMember{
				CommunityID: idb.UIDFrom(communityID),
				UserID:      idb.UIDFrom(user),
			})
		}
		if err := tx.Create(&rows).Error; err != nil {
			return err
		}
		return tx.Model(&idb.Community{}).
			Where("id IN ?", communities).
			Update("no_members", gorm.Expr("no_members + ?", 1)).
			Error
	})
}

func usernameExists(ctx context.Context, db *gorm.DB, username string) (exists bool, user uid.ID, err error) {
	username = strings.ToLower(username)
	var row struct {
		ID uid.ID
	}
	if err := db.WithContext(ctx).Table("users").Select("id").Where("username_lc = ?", username).Take(&row).Error; err == nil {
		exists = true
		user = row.ID
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		err = nil
	}
	return
}

func userWithEmailExists(ctx context.Context, db *gorm.DB, email string) (exists bool, user uid.ID, err error) {
	var row struct {
		ID uid.ID
	}
	if err := db.WithContext(ctx).Table("users").Select("id").Where("email = ?", email).Take(&row).Error; err == nil {
		exists = true
		user = row.ID
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		err = nil
	}
	return
}

// MatchLoginCredentials returns a nil error if a user was found and the
// password matches.
func MatchLoginCredentials(ctx context.Context, db *gorm.DB, username, password string) (*User, error) {
	user, err := GetUserByUsername(ctx, db, username, nil)
	if err != nil {
		if err == errUserNotFound {
			return nil, ErrWrongPassword
		}
		return nil, err
	}

	if user.Deleted {
		return nil, ErrWrongPassword
	}

	if err = bcrypt.CompareHashAndPassword([]byte(user.Password), trimPassword([]byte(password))); err != nil {
		return nil, ErrWrongPassword
	}
	return user, nil
}

// incrementUserPoints adds amount to user's points.
func incrementUserPoints(ctx context.Context, tx *gorm.DB, user uid.ID, amount int) error {
	return tx.WithContext(ctx).
		Model(&idb.User{}).
		Where("id = ?", user).
		Update("points", gorm.Expr("points + ?", amount)).
		Error
}

// If db is nil, tx is used for the query execution.
func getUserPointsAndCreatedAt(ctx context.Context, db *gorm.DB, tx *gorm.DB, user uid.ID) (int, time.Time, error) {
	var (
		row struct {
			Points    int
			CreatedAt time.Time
		}
		query *gorm.DB
	)

	if db != nil {
		query = db.WithContext(ctx)
	} else {
		query = tx.WithContext(ctx)
	}

	if err := query.Table("users").Select("points", "created_at").Where("users.id = ?", user).Take(&row).Error; err != nil {
		return 0, time.Time{}, err
	}
	return row.Points, row.CreatedAt, nil
}

func userAllowedToIncrementPoints(ctx context.Context, tx *gorm.DB, user uid.ID, requiredPoints int, requiredAge time.Duration) (bool, error) {
	points, createdAt, err := getUserPointsAndCreatedAt(ctx, nil, tx, user)
	if err != nil {
		return false, err
	}
	return points >= requiredPoints && time.Since(createdAt) > requiredAge, nil
}

func UserAllowedToPostImages(ctx context.Context, db *gorm.DB, user uid.ID, requiredPoints int) (bool, error) {
	points, _, err := getUserPointsAndCreatedAt(ctx, db, nil, user)
	if err != nil {
		return false, err
	}
	return points >= requiredPoints, nil
}

// Update updates the user's updatable fields.
func (u *User) Update(ctx context.Context, db *gorm.DB) error {
	if u.Deleted {
		return ErrUserDeleted
	}

	u.About.String = utils.TruncateUnicodeString(u.About.String, maxUserProfileAboutLength)
	values := map[string]any{
		"email":                      u.EmailPublic,
		"about_me":                   u.About,
		"upvote_notifications_off":   u.UpvoteNotificationsOff,
		"reply_notifications_off":    u.ReplyNotificationsOff,
		"home_feed":                  u.HomeFeed,
		"remember_feed_sort":         u.RememberFeedSort,
		"embeds_off":                 u.EmbedsOff,
		"hide_user_profile_pictures": u.HideUserProfilePictures,
		"require_alt_text":           u.RequireAltText,
	}
	return db.WithContext(ctx).Model(&idb.User{}).Where("id = ?", u.ID).Updates(values).Error
}

func (u *User) IsGhost() bool {
	return u.Username == "ghost"
}

// SetToGhost sets u.username to "ghost" and u.ID to zero, if the user is
// deleted.
func (u *User) SetToGhost() {
	if u.Deleted {
		u.Username = "ghost"
		u.UsernameLowerCase = "ghost"
		u.ID = uid.ID{}
		u.CreatedAt = time.Time{}
		u.DeletedAt = msql.NewNullTime(time.Time{})
		u.Badges = make(Badges, 0)
	}
}

// UnsetToGhost is the inverse of u.SetToGhost.
func (u *User) UnsetToGhost() {
	if u.Deleted {
		u.Username = u.preGhostUsername
		u.UsernameLowerCase = strings.ToLower(u.Username)
		u.ID = u.preGhostID
		u.CreatedAt = u.preGhostCreatedAt
		u.DeletedAt = u.preGhostDeletedAt
		u.Badges = u.preGhostBadges
	}
}

// MarshalJSONForAdminViewer marshals the user with additional fields for admin
// eyes only.
func (u *User) MarshalJSONForAdminViewer(ctx context.Context, db *gorm.DB) ([]byte, error) {
	user := &struct {
		*User
		CreatedIP                msql.NullIP `json:"createdIP"`
		UserIndex                int         `json:"userIndex"`
		LastSeen                 time.Time   `json:"lastSeen"`
		LastSeenIP               msql.NullIP `json:"lastSeenIP"`
		WebPushSubsriptionsCount int         `json:"webPushSubscriptionsCount"`
	}{
		User:       u,
		CreatedIP:  u.CreatedIP,
		UserIndex:  u.UserIndex,
		LastSeen:   u.LastSeen,
		LastSeenIP: u.LastSeenIP,
	}

	var count int64
	if err := db.Model(&idb.WebPushSubscription{}).Where("user_id = ?", u.ID).Count(&count).Error; err != nil {
		return nil, err
	}
	user.WebPushSubsriptionsCount = int(count)

	return json.Marshal(user)
}

// Delete deletes a user. Make sure that the user is logged out on all sessions
// before calling this function.
func (u *User) Delete(ctx context.Context, db *gorm.DB) error {
	if u.Deleted {
		return ErrUserDeleted
	}
	if u.Banned {
		return errors.New("cannot delete banned account (unban user first and then continue)")
	}

	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		subQuery := tx.Table("community_members").Select("community_id").Where("user_id = ?", u.ID)
		if err := tx.Model(&idb.Community{}).
			Where("id IN (?)", subQuery).
			Update("no_members", gorm.Expr("no_members - ?", 1)).
			Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", u.ID).Delete(&idb.CommunityMember{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", u.ID).Delete(&idb.CommunityMod{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", u.ID).Delete(&idb.CommunityBanned{}).Error; err != nil {
			return err
		}
		if err := tx.Model(&idb.Comment{}).Where("user_id = ?", u.ID).Update("user_deleted", true).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", u.ID).Delete(&idb.Notification{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ? OR muted_user_id = ?", u.ID, u.ID).Delete(&idb.MutedUser{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", u.ID).Delete(&idb.MutedCommunity{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", u.ID).Delete(&idb.WebPushSubscription{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", u.ID).Delete(&idb.List{}).Error; err != nil {
			return err
		}
		if err := u.DeleteProPicTx(ctx, db, tx); err != nil {
			return err
		}
		now := time.Now()
		if err := tx.Model(&idb.User{}).Where("id = ?", u.ID).Updates(map[string]any{
			"email":                   nil,
			"email_confirmed_at":      nil,
			"password":                utils.GenerateStringID(48),
			"about_me":                nil,
			"is_admin":                false,
			"notifications_new_count": 0,
			"deleted_at":              now,
		}).Error; err != nil {
			return err
		}
		u.DeletedAt = msql.NewNullTime(now)
		u.NumNewNotifications = 0
		return nil
	})
}

// DeleteContent deletes all posts and comments of user that were created in the
// last n days (n=0 means all time). It does not delete the user.
func (u *User) DeleteContent(ctx context.Context, db *gorm.DB, n int, admin uid.ID) error {
	t := time.Now()
	defer func() {
		log.Printf("Took %v to delete content of user %s\n", time.Since(t), u.Username)
	}()

	postIDsQuery := db.WithContext(ctx).Table("posts").Select("id").Where("user_id = ?", u.ID)
	if n > 0 {
		since := time.Now().Add(-1 * time.Hour * 24 * time.Duration(n))
		postIDsQuery = postIDsQuery.Where("created_at > ?", since)
	}

	rows, err := postIDsQuery.Rows()
	if err != nil {
		return err
	}
	defer rows.Close()

	var postIDs []uid.ID
	for rows.Next() {
		var id uid.ID
		if err := rows.Scan(&id); err != nil {
			return err
		}
		postIDs = append(postIDs, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	posts, err := GetPostsByIDs(ctx, db, nil, true, postIDs...)
	if err != nil {
		return err
	}

	for _, post := range posts {
		if !(post.Deleted && post.DeletedContent) {
			if err := post.Delete(ctx, db, admin, UserGroupAdmins, true, false); err != nil {
				return err
			}
		}
	}

	where, args := "comments.user_id = ?", []any{u.ID}
	if n > 0 {
		since := time.Now().Add(-1 * time.Hour * 24 * time.Duration(n))
		where += " AND comments.created_at > ?"
		args = append(args, since)
	}

	rows, err = selectCommentsQuery(ctx, db, nil).Where(where, args...).Rows()
	if err != nil {
		return err
	}
	comments, err := scanComments(ctx, db, rows, nil)
	if err != nil && err != errCommentNotFound {
		return err
	}

	for _, comment := range comments {
		if !comment.Deleted {
			if err := comment.Delete(ctx, db, admin, UserGroupAdmins); err != nil {
				return err
			}
		}
	}

	return nil
}

// Ban bans the user from site. Important: Make sure to log out all sessions of
// this user before calling this function, and never allow this user to login.
//
// Note: An admin can be banned.
func (u *User) Ban(ctx context.Context, db *gorm.DB) error {
	t := time.Now()
	err := db.WithContext(ctx).Model(&idb.User{}).Where("id = ?", u.ID).Update("banned_at", t).Error
	if err == nil {
		u.BannedAt = msql.NewNullTime(t)
		u.Banned = true
	}
	return err
}

func (u *User) Unban(ctx context.Context, db *gorm.DB) error {
	return db.WithContext(ctx).Model(&idb.User{}).Where("id = ?", u.ID).Update("banned_at", nil).Error
}

// MakeAdmin makes the user an admin of the site. If isAdmin is false
// admin is removed as an admin.
func (u *User) MakeAdmin(ctx context.Context, db *gorm.DB, isAdmin bool) error {
	_, err := MakeAdmin(ctx, db, u.Username, isAdmin)
	if err != nil {
		u.Admin = isAdmin
	}
	return err
}

func (u *User) ChangePassword(ctx context.Context, db *gorm.DB, previousPass, newPass string) error {
	// MatchLoginCredentials checks for deleted account status.
	if _, err := MatchLoginCredentials(ctx, db, u.Username, previousPass); err != nil {
		return err
	}
	hash, err := HashPassword([]byte(newPass))
	if err != nil {
		return err
	}
	err = db.WithContext(ctx).Model(&idb.User{}).Where("id = ?", u.ID).Update("password", string(hash)).Error
	u.Password = string(hash)
	return err
}

func (u *User) ResetNewNotificationsCount(ctx context.Context, db *gorm.DB) error {
	err := resetNewNotificationsCount(ctx, db, u.ID)
	if err == nil {
		u.NumNewNotifications = 0
	}
	return err
}

// MarkAllNotificationsAsSeen marks all notifications as seen, if t is the zero
// value, and if not, it marks all notifications of type t as seen.
func (u *User) MarkAllNotificationsAsSeen(ctx context.Context, db *gorm.DB, t NotificationType) error {
	return markAllNotificationsAsSeen(ctx, db, u.ID, t)
}

func (u *User) DeleteAllNotifications(ctx context.Context, db *gorm.DB) error {
	return deleteAllNotifications(ctx, db, u.ID)
}

// GetBannedFromCommunities returns the list of communities that user
// is banned from.
func (u *User) GetBannedFromCommunities(ctx context.Context, db *gorm.DB) ([]uid.ID, error) {
	rows, err := db.WithContext(ctx).Table("community_banned").Select("community_id", "expires").Where("user_id = ?", u.ID).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []uid.ID
	var expires []msql.NullTime
	for rows.Next() {
		var id uid.ID
		var e msql.NullTime
		if err = rows.Scan(&id, &e); err != nil {
			return nil, err
		}
		ids = append(ids, id)
		expires = append(expires, e)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	var nonExpired []uid.ID
	for i := range ids {
		if !expires[i].Valid || time.Since(expires[i].Time) > 0 {
			nonExpired = append(nonExpired, ids[i])
		}
	}
	return nonExpired, nil
}

// Saw updates u.LastSeen to current time. It also updates the IP address of the
// user.
func (u *User) Saw(ctx context.Context, db *gorm.DB, userIP string) error {
	if u.Deleted {
		return ErrUserDeleted
	}
	err := UserSeen(ctx, db, u.ID, userIP)
	if err != nil {
		u.LastSeen = time.Now()
	}
	return err
}

// UserSeen updates user's LastSeen to current time. It also updates the IP
// address of the user.
func UserSeen(ctx context.Context, db *gorm.DB, user uid.ID, userIP string) error {
	return db.WithContext(ctx).
		Model(&idb.User{}).
		Where("id = ? AND deleted_at IS NULL", user).
		Updates(map[string]any{
			"last_seen":    time.Now(),
			"last_seen_ip": userIP,
		}).
		Error
}

// CountAllUsers return the no of users of the site, including deleted users.
func CountAllUsers(ctx context.Context, db *gorm.DB) (n int, err error) {
	var count int64
	err = db.WithContext(ctx).Model(&idb.User{}).Count(&count).Error
	n = int(count)
	return
}

func getUserModeratingCommunities(ctx context.Context, db *gorm.DB, user uid.ID) ([]*Community, error) {
	return getCommunities(ctx, db, nil, "WHERE communities.id IN (SELECT community_mods.community_id FROM community_mods WHERE user_id = ?)", user)
}

func (u *User) LoadModeratingCommunitiesList(ctx context.Context, db *gorm.DB) error {
	comms, err := getUserModeratingCommunities(ctx, db, u.ID)
	if err != nil {
		return err
	}
	u.ModdingList = comms
	return nil
}

func (u *User) DeleteProPicTx(ctx context.Context, db *gorm.DB, tx *gorm.DB) error {
	if u.ProPic == nil {
		return nil
	}
	if err := db.WithContext(ctx).Model(&idb.User{}).Where("id = ?", u.ID).Update("pro_pic", nil).Error; err != nil {
		return fmt.Errorf("failed to set users.pro_pic to null for user %s: %w", u.Username, err)
	}
	if err := images.DeleteImagesTx(ctx, tx, db, *u.ProPic.ID); err != nil {
		return fmt.Errorf("failed to delete pro pic of user %s: %w", u.Username, err)
	}
	u.ProPic = nil
	return nil
}

func (u *User) DeleteProPic(ctx context.Context, db *gorm.DB) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return u.DeleteProPicTx(ctx, db, tx)
	})
}

func (u *User) UpdateProPic(ctx context.Context, db *gorm.DB, image []byte) error {
	if u.Deleted {
		return ErrUserDeleted
	}

	var newImageID uid.ID
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := u.DeleteProPicTx(ctx, db, tx); err != nil {
			return err
		}
		imageID, err := images.SaveImageTx(ctx, tx, "disk", image, &images.ImageOptions{
			Width:  2000,
			Height: 2000,
			Format: images.ImageFormatJPEG,
			Fit:    images.ImageFitContain,
		})
		if err != nil {
			return fmt.Errorf("fail to save user pro pic: %w", err)
		}
		if err := tx.WithContext(ctx).Model(&idb.User{}).Where("id = ?", u.ID).Update("pro_pic", imageID).Error; err != nil {
			// Attempt to delete the image
			if err := images.DeleteImagesTx(ctx, tx, db, imageID); err != nil {
				log.Printf("failed to delete image (core.User.UpdateProPic): %v\n", err)
			}
			return fmt.Errorf("failed to set users.pro_pic to value: %w", err)
		}
		newImageID = imageID
		return nil
	})
	if err != nil {
		return err
	}

	record, err := images.GetImageRecord(ctx, db, newImageID)
	if err != nil {
		return err
	}
	u.ProPic = record.Image()
	setCommunityProPicCopies(u.ProPic)
	return nil
}

func (u *User) Muted(ctx context.Context, db *gorm.DB, user uid.ID) (bool, error) {
	return UserMuted(ctx, db, u.ID, user)
}

func (u *User) MutedBy(ctx context.Context, db *gorm.DB, user uid.ID) (bool, error) {
	return UserMuted(ctx, db, user, u.ID)
}

// badgeTypeInt returns the int badge type of badgeType.
func badgeTypeInt(db *gorm.DB, badgeType string) (int, error) {
	var row struct {
		ID int
	}
	if err := db.Table("badge_types").Select("id").Where("name = ?", badgeType).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, httperr.NewNotFound("badge_type_not_found", "Badge type not found.")
		}
		return 0, err
	}
	return row.ID, nil
}

// AddBadge addes new badge to u. Also, u.Badges is refetched upon a successful
// add.
func (u *User) AddBadge(ctx context.Context, db *gorm.DB, badgeType string) error {
	if u.Deleted {
		return ErrUserDeleted
	}

	badgeTypeInt, err := badgeTypeInt(db, badgeType)
	if err != nil {
		return err
	}
	err = db.Create(&idb.UserBadge{Type: uint(badgeTypeInt), UserID: idb.UIDFrom(u.ID)}).Error
	if err != nil && !msql.IsErrDuplicateErr(err) {
		return err
	}

	if err := CreateNewBadgeNotification(ctx, db, u.ID, badgeType); err != nil {
		log.Printf("Error creating new badge notification: %v\n", err)
	}

	return fetchBadges(db, u)
}

func (u *User) RemoveBadgesByType(db *gorm.DB, badgeType string) error {
	if u.Deleted {
		return ErrUserDeleted
	}

	badgeTypeInt, err := badgeTypeInt(db, badgeType)
	if err != nil {
		return err
	}
	return db.Where("type = ? AND user_id = ?", badgeTypeInt, u.ID).Delete(&idb.UserBadge{}).Error
}

func (u *User) RemoveBadge(db *gorm.DB, id int) error {
	if u.Deleted {
		return ErrUserDeleted
	}

	return db.Where("id = ? and user_id = ?", id, u.ID).Delete(&idb.UserBadge{}).Error
}

func (u *User) HidePost(ctx context.Context, db *gorm.DB, postID uid.ID) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&idb.HiddenPost{UserID: idb.UIDFrom(u.ID), PostID: idb.UIDFrom(postID)}).Error; err != nil {
			if msql.IsErrDuplicateErr(err) {
				return nil
			}
			return err
		}

		var count int64
		if err := tx.Model(&idb.HiddenPost{}).Where("user_id = ?", u.ID).Count(&count).Error; err != nil {
			return err
		}
		if count > int64(maxHiddenPosts) {
			var threshold struct {
				CreatedAt time.Time
			}
			if err := tx.Table("hidden_posts").
				Select("created_at").
				Where("user_id = ?", u.ID).
				Order("created_at DESC").
				Offset(maxHiddenPosts).
				Limit(1).
				Take(&threshold).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return nil
				}
				return err
			}
			if err := tx.Where("user_id = ? AND created_at <= ?", u.ID, threshold.CreatedAt).Delete(&idb.HiddenPost{}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (u *User) UnhidePost(ctx context.Context, db *gorm.DB, postID uid.ID) error {
	return db.WithContext(ctx).Where("user_id = ? AND post_id = ?", u.ID, postID).Delete(&idb.HiddenPost{}).Error
}

// NewBadgeType creates a new type of user badge. Calling this function more
// than once with the same name will not result in an error.
func NewBadgeType(db *gorm.DB, name string) error {
	if err := db.Create(&idb.BadgeType{Name: name}).Error; err != nil && !msql.IsErrDuplicateErr(err) {
		return err
	}
	return nil
}

// A Badge corresponds to a row in the user_badges table.
type Badge struct {
	ID        int       `json:"id"`
	Type      int       `json:"-"`
	TypeName  string    `json:"type"`
	UserID    uid.ID    `json:"-"`
	CreatedAt time.Time `json:"-"`
}

type Badges []*Badge

// fetchBadges retrives all the badges of users from DB and populates each users
// Badges field.
func fetchBadges(db *gorm.DB, users ...*User) error {
	if len(users) == 0 {
		return nil
	}
	if len(users) > 1000 {
		log.Printf("Warning: fetching more than %d users badges at once\n", len(users))
	}

	userIDs := make([]any, len(users))
	m := make(map[uid.ID]*User)
	for i, user := range users {
		userIDs[i] = user.ID
		m[user.ID] = user
	}

	rows, err := db.Table("user_badges AS b").
		Select("b.id", "b.type", "t.name", "b.user_id", "b.created_at").
		Joins("INNER JOIN badge_types AS t ON b.type = t.id").
		Where("b.user_id IN ?", userIDs).
		Rows()
	if err != nil {
		return err
	}
	defer rows.Close()

	badges := Badges{}
	for rows.Next() {
		b := &Badge{}
		rows.Scan(
			&b.ID,
			&b.Type,
			&b.TypeName,
			&b.UserID,
			&b.CreatedAt)
		badges = append(badges, b)
	}

	if err = rows.Err(); err != nil {
		return err
	}

	for _, b := range badges {
		user, ok := m[b.UserID]
		if !ok {
			return fmt.Errorf("fetching badges user (%v) not found for badge %s", b.UserID, b.TypeName)
		}
		user.Badges = append(user.Badges, b)
	}

	return nil
}

type adminsCacheStore struct {
	mu          sync.RWMutex // guards following
	admins      []uid.ID
	usernames   []string // of the admins
	lastFetched time.Time
}

func (ac *adminsCacheStore) isAdmin(db *gorm.DB, user uid.ID) (bool, error) {
	ac.mu.RLock()
	shouldRefetch := false
	if time.Since(ac.lastFetched) > time.Minute*5 {
		shouldRefetch = true
	}
	ac.mu.RUnlock()

	if shouldRefetch {
		if err := ac.refresh(db); err != nil {
			return false, err
		}
	}

	ac.mu.RLock()
	defer ac.mu.RUnlock()
	return slices.Index(ac.admins, user) != -1, nil
}

func (ac *adminsCacheStore) refresh(db *gorm.DB) error {
	rows, err := db.Table("users").Select("id", "username").Where("users.is_admin = ?", true).Rows()
	if err != nil {
		return err
	}
	defer rows.Close()

	var admins []uid.ID
	var usernames []string
	for rows.Next() {
		var id uid.ID
		var username string
		if err = rows.Scan(&id, &username); err != nil {
			return err
		}
		admins = append(admins, id)
		usernames = append(usernames, username)
	}
	if err = rows.Err(); err != nil {
		return err
	}

	ac.mu.Lock()
	defer ac.mu.Unlock()

	ac.admins = admins
	ac.usernames = usernames
	ac.lastFetched = time.Now()
	return nil
}

var adminsCache = &adminsCacheStore{}

// IsAdmin reports whether user is an admin. User can be nil, in which case this
// function returns false.
func IsAdmin(db *gorm.DB, user *uid.ID) (bool, error) {
	if user == nil {
		return false, nil
	}
	return adminsCache.isAdmin(db, *user)
}

// MakeAdmin makes the user an admin of the site. If isAdmin is false admin user
// is removed as an admin.
func MakeAdmin(ctx context.Context, db *gorm.DB, user string, isAdmin bool) (*User, error) {
	u, err := GetUserByUsername(ctx, db, user, nil)
	if err != nil {
		return nil, err
	}
	if u.Deleted {
		return nil, ErrUserDeleted
	}

	if isAdmin {
		if u.Admin {
			return nil, httperr.NewBadRequest("already-admin", "User is already an admin.")
		}
	} else {
		if !u.Admin {
			return nil, httperr.NewBadRequest("already-not-admin", "User is already not an admin.")
		}
	}

	// Note: Duplicate the changes to the User.Delete function when making changes to this SQL query.
	if err = db.WithContext(ctx).Model(&idb.User{}).Where("id = ?", u.ID).Update("is_admin", isAdmin).Error; err != nil {
		return nil, err
	}

	if err := adminsCache.refresh(db); err != nil {
		fmt.Printf("Error refreshing admins list cache: %v", err)
	}

	u.Admin = isAdmin
	return u, nil
}

const (
	GhostUserUsername  = "ghost"
	NobodyUserUsername = "nobody"
)

// CreateGhostUser creates the ghost user, if the ghost user isn't already
// created. The ghost user is the user with the username ghost that takes, so to
// speak, the place of all deleted users.
//
// The returned bool indicates whether the call to this function created the
// ghost user (if the ghost user was already created, it will be false).
func CreateGhostUser(db *gorm.DB) (bool, error) {
	var row struct {
		UsernameLC string
	}
	if err := db.Table("users").Select("username_lc").Where("username_lc = ?", GhostUserUsername).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Ghost user not found; create one.
			_, createErr := RegisterUser(context.Background(), db, GhostUserUsername, "", utils.GenerateStringID(48), "")
			return createErr == nil, createErr
		}
		return false, err
	}
	return false, nil
}

// CreateNobodyUser creates a user named nobody and makes that user an admin.
// This is an admin account reserved for programmatic admin actions.
func CreateNobodyUser(db *gorm.DB) (bool, error) {
	var row struct {
		UsernameLC string
	}
	if dbErr := db.Table("users").Select("username_lc").Where("username_lc = ?", NobodyUserUsername).Take(&row).Error; dbErr != nil {
		if errors.Is(dbErr, gorm.ErrRecordNotFound) {
			// Ghost user not found; create one.
			user, err := RegisterUser(context.Background(), db, NobodyUserUsername, "", utils.GenerateStringID(48), "")
			if err != nil {
				return false, err
			}

			user.About = msql.NewNullString("Not a human")
			user.Update(context.Background(), db)

			if _, err := MakeAdmin(context.Background(), db, user.Username, true); err != nil {
				return false, err
			}
			return true, nil
		}
		return false, dbErr
	}
	return false, nil
}

func CalcGhostUserID(user uid.ID, unique string) string {
	b := make([]byte, len(user)+len(unique))
	copy(b, user[:])
	copy(b[len(user):], []byte(unique))
	sum := sha1.Sum(b)
	return hex.EncodeToString(sum[:])[:8]
}

func UserDeleted(db *gorm.DB, user uid.ID) (bool, error) {
	var row struct {
		DeletedAt msql.NullTime
	}
	if err := db.Table("users").Select("deleted_at").Where("id = ?", user).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, errUserNotFound
		}
		return false, err
	}
	return row.DeletedAt.Valid, nil
}

// UserMuted reports whether the user muted is muted by the user muter.
func UserMuted(ctx context.Context, db *gorm.DB, muter, muted uid.ID) (bool, error) {
	var row struct {
		ID int
	}
	if err := db.WithContext(ctx).Table("muted_users").Select("id").Where("user_id = ? AND muted_user_id = ?", muter, muted).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("UserMuted db error: %w", err)
	}
	return true, nil
}

func GetUsers(ctx context.Context, db *gorm.DB, limit int, next *string, viewer *uid.ID) ([]*User, *string, error) {
	query := buildSelectUserQuery(ctx, db)
	if next != nil {
		nextID, err := uid.FromString(*next)
		if err != nil {
			return nil, nil, errors.New("invalid next for site users")
		}
		query = query.Where("users.id <= ?", nextID)
	}
	rows, err := query.Order("users.id DESC").Limit(limit + 1).Rows()
	if err != nil {
		return nil, nil, err
	}

	users, err := scanUsers(ctx, db, rows, viewer)
	if err != nil {
		return nil, nil, err
	}

	var nextNext *string
	if len(users) >= limit+1 {
		nextNext = new(string)
		*nextNext = users[limit].ID.String()
		users = users[:limit]
	}

	return users, nextNext, nil
}

func GetAllUserIDs(ctx context.Context, db *gorm.DB, fetchDeleted, fetchBanned bool) ([]uid.ID, error) {
	query := db.WithContext(ctx).Table("users").Select("id")
	if !fetchDeleted {
		query = query.Where("deleted_at IS NULL")
	}
	if !fetchBanned {
		query = query.Where("banned_at IS NULL")
	}

	rows, err := query.Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := []uid.ID{}
	for rows.Next() {
		var id uid.ID
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
