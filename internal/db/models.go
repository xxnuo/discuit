package db

import "time"

type MigrationRecord struct {
	Version   int       `gorm:"column:version;primaryKey;autoIncrement:false"`
	Name      string    `gorm:"column:name;type:varchar(255);not null"`
	Dirty     bool      `gorm:"column:dirty;not null;default:false"`
	AppliedAt time.Time `gorm:"column:applied_at;not null"`
}

func (MigrationRecord) TableName() string {
	return "schema_migrations"
}

type User struct {
	ID                      UID     `gorm:"column:id;primaryKey"`
	UserIndex               *int    `gorm:"column:user_index;uniqueIndex"`
	Username                string  `gorm:"column:username;type:varchar(20);not null"`
	UsernameLC              string  `gorm:"column:username_lc;type:varchar(20);not null;uniqueIndex"`
	Email                   *string `gorm:"column:email;type:varchar(255);index"`
	EmailConfirmedAt        *time.Time
	Password                string     `gorm:"column:password;type:varchar(128);not null"`
	AboutMe                 *string    `gorm:"column:about_me;type:text"`
	Points                  int        `gorm:"column:points;not null;default:1"`
	IsAdmin                 bool       `gorm:"column:is_admin;not null;default:false"`
	NotificationsNewCount   int        `gorm:"column:notifications_new_count;not null;default:0"`
	CreatedAt               time.Time  `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP"`
	DeletedAt               *time.Time `gorm:"column:deleted_at"`
	NoPosts                 int        `gorm:"column:no_posts;not null;default:0"`
	NoComments              int        `gorm:"column:no_comments;not null;default:0"`
	LastSeen                time.Time  `gorm:"column:last_seen;not null;default:CURRENT_TIMESTAMP;index"`
	LastSeenIP              *IP        `gorm:"column:last_seen_ip"`
	BannedAt                *time.Time `gorm:"column:banned_at"`
	UpvoteNotificationsOff  bool       `gorm:"column:upvote_notifications_off;not null;default:false"`
	ReplyNotificationsOff   bool       `gorm:"column:reply_notifications_off;not null;default:false"`
	HomeFeed                int        `gorm:"column:home_feed;not null;default:0"`
	RememberFeedSort        bool       `gorm:"column:remember_feed_sort;not null;default:false"`
	EmbedsOff               bool       `gorm:"column:embeds_off;not null;default:false"`
	ProPic                  *UID       `gorm:"column:pro_pic"`
	HideUserProfilePictures bool       `gorm:"column:hide_user_profile_pictures;not null;default:false"`
	WelcomeNotificationSent bool       `gorm:"column:welcome_notification_sent;not null;default:false;index"`
	CreatedIP               *IP        `gorm:"column:created_ip"`
	RequireAltText          bool       `gorm:"column:require_alt_text;not null;default:false"`
}

func (User) TableName() string {
	return "users"
}

type Community struct {
	ID                UID       `gorm:"column:id;primaryKey"`
	UserID            UID       `gorm:"column:user_id;not null"`
	Name              string    `gorm:"column:name;type:varchar(128);not null"`
	NameLC            string    `gorm:"column:name_lc;type:varchar(128);not null;uniqueIndex"`
	NSFW              bool      `gorm:"column:nsfw;not null;default:false"`
	About             *string   `gorm:"column:about;type:text"`
	NoMembers         uint      `gorm:"column:no_members;not null;default:0"`
	CreatedAt         time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP"`
	DeletedAt         *time.Time
	DeletedBy         *UID    `gorm:"column:deleted_by"`
	ProPic            *string `gorm:"column:pro_pic;type:text"`
	BannerImage       *string `gorm:"column:banner_image;type:text"`
	ProPic2           *UID    `gorm:"column:pro_pic_2"`
	BannerImage2      *UID    `gorm:"column:banner_image_2"`
	PostsCount        int     `gorm:"column:posts_count;not null;default:0"`
	PostingRestricted bool    `gorm:"column:posting_restricted;not null;default:false"`
}

func (Community) TableName() string {
	return "communities"
}

type CommunityMember struct {
	ID          uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	CommunityID UID       `gorm:"column:community_id;not null;uniqueIndex:community_members_one_user,priority:1;index:community_members_community_created,priority:1"`
	UserID      UID       `gorm:"column:user_id;not null;uniqueIndex:community_members_one_user,priority:2"`
	IsMod       bool      `gorm:"column:is_mod;not null;default:false"`
	CreatedAt   time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP"`
}

func (CommunityMember) TableName() string {
	return "community_members"
}

type CommunityMod struct {
	ID          uint      `gorm:"column:id;primaryKey;autoIncrement"`
	CommunityID UID       `gorm:"column:community_id;not null;uniqueIndex:community_mods_one_user,priority:1"`
	UserID      UID       `gorm:"column:user_id;not null;uniqueIndex:community_mods_one_user,priority:2"`
	CreatedAt   time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP"`
	Position    int       `gorm:"column:position;not null;default:0"`
}

func (CommunityMod) TableName() string {
	return "community_mods"
}

type CommunityBanned struct {
	ID          uint       `gorm:"column:id;primaryKey;autoIncrement"`
	CommunityID UID        `gorm:"column:community_id;not null;uniqueIndex:community_banned_one_user,priority:1"`
	UserID      UID        `gorm:"column:user_id;not null;uniqueIndex:community_banned_one_user,priority:2"`
	Expires     *time.Time `gorm:"column:expires"`
	BannedBy    UID        `gorm:"column:banned_by;not null"`
	CreatedAt   time.Time  `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP"`
}

func (CommunityBanned) TableName() string {
	return "community_banned"
}

type Post struct {
	ID               UID        `gorm:"column:id;primaryKey"`
	Type             int8       `gorm:"column:type;not null;default:0"`
	PublicID         string     `gorm:"column:public_id;type:char(8);not null;uniqueIndex"`
	UserID           UID        `gorm:"column:user_id;not null"`
	UserGroup        int8       `gorm:"column:user_group;not null;default:1"`
	CommunityID      UID        `gorm:"column:community_id;not null;index:posts_comm_deleted_points_id,priority:1;index:posts_comm_deleted_id,priority:1;index:posts_deleted_comm_hotness_id,priority:2;index:posts_last_activity_community,priority:1"`
	Title            string     `gorm:"column:title;type:varchar(255);not null"`
	Body             *string    `gorm:"column:body;type:text"`
	Image            *string    `gorm:"column:image;type:text"`
	LinkInfo         *string    `gorm:"column:link_info;type:text"`
	LinkImage        *UID       `gorm:"column:link_image"`
	Locked           bool       `gorm:"column:locked;not null;default:false"`
	LockedAt         *time.Time `gorm:"column:locked_at;index"`
	LockedBy         *UID       `gorm:"column:locked_by"`
	LockedByGroup    int8       `gorm:"column:locked_by_group;not null;default:0"`
	NoComments       uint       `gorm:"column:no_comments;not null;default:0"`
	Upvotes          uint       `gorm:"column:upvotes;not null;default:0"`
	Downvotes        uint       `gorm:"column:downvotes;not null;default:0"`
	Points           int        `gorm:"column:points;not null;default:0;index:posts_deleted_points_id,priority:2;index:posts_comm_deleted_points_id,priority:3"`
	Hotness          int64      `gorm:"column:hotness;not null;default:0;index:posts_deleted_hotness_id,priority:2;index:posts_deleted_comm_hotness_id,priority:3"`
	CreatedAt        time.Time  `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP"`
	EditedAt         *time.Time `gorm:"column:edited_at"`
	Deleted          bool       `gorm:"column:deleted;not null;default:false;index:posts_deleted_points_id,priority:1;index:posts_comm_deleted_points_id,priority:2;index:posts_deleted_id,priority:1;index:posts_comm_deleted_id,priority:2;index:posts_deleted_hotness_id,priority:1;index:posts_deleted_comm_hotness_id,priority:1;index:posts_last_activity_deleted,priority:1;index:posts_last_activity_community,priority:2"`
	DeletedAt        *time.Time `gorm:"column:deleted_at;index"`
	DeletedBy        *UID       `gorm:"column:deleted_by"`
	DeletedAs        int8       `gorm:"column:deleted_as;not null;default:0"`
	DeletedContent   bool       `gorm:"column:deleted_content;not null;default:false"`
	DeletedContentAt *time.Time `gorm:"column:deleted_content_at"`
	DeletedContentBy *UID       `gorm:"column:deleted_content_by"`
	DeletedContentAs int8       `gorm:"column:deleted_content_as;not null;default:0"`
	IsPinnedSite     bool       `gorm:"column:is_pinned_site;not null;default:false"`
	LastActivityAt   time.Time  `gorm:"column:last_activity_at;not null;default:CURRENT_TIMESTAMP;index:posts_last_activity_deleted,priority:2;index:posts_last_activity_community,priority:3"`
	IsPinned         bool       `gorm:"column:is_pinned;not null;default:false"`
}

func (Post) TableName() string {
	return "posts"
}

type PostWindow struct {
	ID          uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	CommunityID UID       `gorm:"column:community_id;not null"`
	PostID      UID       `gorm:"column:post_id;not null"`
	UserID      UID       `gorm:"column:user_id;not null"`
	Points      int       `gorm:"column:points;not null;default:0"`
	CreatedAt   time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP"`
}

type PostToday struct{ PostWindow }
type PostWeek struct{ PostWindow }
type PostMonth struct{ PostWindow }
type PostYear struct{ PostWindow }

func (PostToday) TableName() string { return "posts_today" }
func (PostWeek) TableName() string  { return "posts_week" }
func (PostMonth) TableName() string { return "posts_month" }
func (PostYear) TableName() string  { return "posts_year" }

type PostVote struct {
	ID        uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	PostID    UID       `gorm:"column:post_id;not null;uniqueIndex:post_votes_post_user,priority:1"`
	UserID    UID       `gorm:"column:user_id;not null;uniqueIndex:post_votes_post_user,priority:2"`
	Up        bool      `gorm:"column:up;not null;default:true"`
	CreatedAt time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP"`
	IsUserNew bool      `gorm:"column:is_user_new;not null;default:false"`
}

func (PostVote) TableName() string {
	return "post_votes"
}

type Comment struct {
	ID              UID        `gorm:"column:id;primaryKey"`
	PostID          UID        `gorm:"column:post_id;not null;index:comments_post_depth_id,priority:1;index:comments_post_upvotes_id,priority:1;index:comments_post_created_at,priority:1"`
	PostPublicID    string     `gorm:"column:post_public_id;type:char(8);not null"`
	CommunityID     UID        `gorm:"column:community_id;not null"`
	CommunityName   string     `gorm:"column:community_name;type:varchar(128);not null"`
	UserID          UID        `gorm:"column:user_id;not null"`
	Username        string     `gorm:"column:username;type:varchar(20);not null"`
	UserDeleted     bool       `gorm:"column:user_deleted;not null;default:false"`
	UserGroup       int8       `gorm:"column:user_group;not null;default:1"`
	ParentID        *UID       `gorm:"column:parent_id"`
	Ancestors       UIDList    `gorm:"column:ancestors"`
	Depth           uint8      `gorm:"column:depth;not null;default:0;index:comments_post_depth_id,priority:2"`
	NoReplies       uint       `gorm:"column:no_replies;not null;default:0"`
	NoRepliesDirect uint       `gorm:"column:no_replies_direct;not null;default:0"`
	Body            *string    `gorm:"column:body;type:text"`
	Upvotes         uint       `gorm:"column:upvotes;not null;default:0;index:comments_post_upvotes_id,priority:2"`
	Downvotes       uint       `gorm:"column:downvotes;not null;default:0"`
	Points          int        `gorm:"column:points;not null;default:0"`
	CreatedAt       time.Time  `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP;index:comments_post_created_at,priority:2"`
	EditedAt        *time.Time `gorm:"column:edited_at"`
	DeletedAt       *time.Time `gorm:"column:deleted_at"`
	DeletedBy       *UID       `gorm:"column:deleted_by"`
	DeletedAs       int8       `gorm:"column:deleted_as;not null;default:0"`
}

func (Comment) TableName() string {
	return "comments"
}

type CommentReply struct {
	ID       uint64 `gorm:"column:id;primaryKey;autoIncrement"`
	ParentID UID    `gorm:"column:parent_id;not null;index:comment_replies_parent_reply,priority:1"`
	ReplyID  UID    `gorm:"column:reply_id;not null;index:comment_replies_parent_reply,priority:2"`
}

func (CommentReply) TableName() string {
	return "comment_replies"
}

type CommentVote struct {
	ID        uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	CommentID UID       `gorm:"column:comment_id;not null;uniqueIndex:comment_votes_comment_user,priority:1"`
	UserID    UID       `gorm:"column:user_id;not null;uniqueIndex:comment_votes_comment_user,priority:2"`
	Up        bool      `gorm:"column:up;not null;default:true"`
	CreatedAt time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP"`
	IsUserNew bool      `gorm:"column:is_user_new;not null;default:false"`
}

func (CommentVote) TableName() string {
	return "comment_votes"
}

type PostsComment struct {
	ID         uint64 `gorm:"column:id;primaryKey;autoIncrement"`
	TargetID   UID    `gorm:"column:target_id;not null;uniqueIndex:posts_comments_user_target_id,priority:2"`
	TargetType int8   `gorm:"column:target_type;not null;index:posts_comments_user_target,priority:2"`
	UserID     UID    `gorm:"column:user_id;not null;uniqueIndex:posts_comments_user_target_id,priority:1;index:posts_comments_user_target,priority:1"`
	Deleted    bool   `gorm:"column:deleted;not null;default:false"`
}

func (PostsComment) TableName() string {
	return "posts_comments"
}

type CommunityRule struct {
	ID          uint      `gorm:"column:id;primaryKey;autoIncrement"`
	Rule        string    `gorm:"column:rule;type:varchar(255);not null"`
	Description *string   `gorm:"column:description;type:varchar(1024)"`
	CommunityID UID       `gorm:"column:community_id;not null"`
	CreatedBy   UID       `gorm:"column:created_by;not null"`
	ZIndex      int       `gorm:"column:z_index;not null;default:0"`
	CreatedAt   time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP"`
}

func (CommunityRule) TableName() string {
	return "community_rules"
}

type Notification struct {
	ID        uint64     `gorm:"column:id;primaryKey;autoIncrement"`
	UserID    UID        `gorm:"column:user_id;not null;index:notifications_user_id;index:notifications_user_updated,priority:1"`
	Type      string     `gorm:"column:type;type:varchar(32);not null"`
	Notif     JSONMap    `gorm:"column:notif"`
	Seen      bool       `gorm:"column:seen;not null;default:false"`
	CreatedAt time.Time  `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP"`
	SeenAt    *time.Time `gorm:"column:seen_at"`
	UpdatedAt time.Time  `gorm:"column:updated_at;not null;default:CURRENT_TIMESTAMP;index:notifications_user_updated,priority:2"`
}

func (Notification) TableName() string {
	return "notifications"
}

type ReportReason struct {
	ID          uint      `gorm:"column:id;primaryKey;autoIncrement"`
	Title       string    `gorm:"column:title;type:varchar(255);not null"`
	Description *string   `gorm:"column:description;type:varchar(1024)"`
	CreatedAt   time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP"`
}

func (ReportReason) TableName() string {
	return "report_reasons"
}

type Report struct {
	ID          uint       `gorm:"column:id;primaryKey;autoIncrement"`
	CommunityID UID        `gorm:"column:community_id;not null"`
	PostID      *UID       `gorm:"column:post_id;index"`
	ReportType  int8       `gorm:"column:report_type;not null"`
	ReasonID    uint       `gorm:"column:reason_id;not null"`
	TargetID    UID        `gorm:"column:target_id;not null;index"`
	CreatedBy   UID        `gorm:"column:created_by;not null"`
	ActionTaken *string    `gorm:"column:action_taken;type:varchar(32);index"`
	DealtAt     *time.Time `gorm:"column:dealt_at;index"`
	DealtBy     *UID       `gorm:"column:dealt_by"`
	CreatedAt   time.Time  `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP;index"`
}

func (Report) TableName() string {
	return "reports"
}

type DefaultCommunity struct {
	ID          uint   `gorm:"column:id;primaryKey;autoIncrement"`
	NameLC      string `gorm:"column:name_lc;type:varchar(128);not null;uniqueIndex"`
	CommunityID UID    `gorm:"column:community_id;not null"`
}

func (DefaultCommunity) TableName() string {
	return "default_communities"
}

type CommunityRequest struct {
	ID              uint       `gorm:"column:id;primaryKey;autoIncrement"`
	ByUser          string     `gorm:"column:by_user;type:varchar(20);not null;index:community_requests_user_name,priority:1"`
	CommunityName   string     `gorm:"column:community_name;type:varchar(128);not null"`
	CommunityNameLC string     `gorm:"column:community_name_lc;type:varchar(128);not null;index:community_requests_user_name,priority:2"`
	Note            *string    `gorm:"column:note;type:text"`
	CreatedAt       time.Time  `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP"`
	DeletedAt       *time.Time `gorm:"column:deleted_at"`
	DeniedNote      *string    `gorm:"column:denied_note;type:text"`
	DeniedBy        *string    `gorm:"column:denied_by;type:varchar(20)"`
	DeniedAt        *time.Time `gorm:"column:denied_at"`
}

func (CommunityRequest) TableName() string {
	return "community_requests"
}

type ApplicationData struct {
	Key       string    `gorm:"column:key;primaryKey;type:varchar(255)"`
	Value     *string   `gorm:"column:value;type:text"`
	CreatedAt time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP"`
}

func (ApplicationData) TableName() string {
	return "application_data"
}

type WebPushSubscription struct {
	ID               uint64     `gorm:"column:id;primaryKey;autoIncrement"`
	SessionID        string     `gorm:"column:session_id;type:varchar(512);not null;uniqueIndex"`
	UserID           UID        `gorm:"column:user_id;not null"`
	PushSubscription JSONMap    `gorm:"column:push_subscription"`
	CreatedAt        time.Time  `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt        *time.Time `gorm:"column:updated_at"`
}

func (WebPushSubscription) TableName() string {
	return "web_push_subscriptions"
}

type Analytics struct {
	ID        uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	EventName string    `gorm:"column:event_name;type:varchar(255);not null;index:analytics_event_created,priority:1"`
	UniqueKey Bytes16   `gorm:"column:unique_key;uniqueIndex"`
	Payload   *string   `gorm:"column:payload;type:text"`
	CreatedAt time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP;index:analytics_event_created,priority:2;index"`
}

func (Analytics) TableName() string {
	return "analytics"
}

type PinnedPost struct {
	ID          uint      `gorm:"column:id;primaryKey;autoIncrement"`
	PostID      UID       `gorm:"column:post_id;not null;uniqueIndex:pinned_posts_unique_pin,priority:2;index:pinned_posts_community_post,priority:2"`
	ZIndex      int       `gorm:"column:z_index;not null;default:0"`
	CreatedAt   time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP"`
	CommunityID *UID      `gorm:"column:community_id;index:pinned_posts_community_post,priority:1"`
	IsSiteWide  bool      `gorm:"column:is_site_wide;not null;uniqueIndex:pinned_posts_unique_pin,priority:1"`
}

func (PinnedPost) TableName() string {
	return "pinned_posts"
}

type MutedCommunity struct {
	ID          uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	UserID      UID       `gorm:"column:user_id;not null;uniqueIndex:muted_communities_user_community,priority:1"`
	CommunityID UID       `gorm:"column:community_id;not null;uniqueIndex:muted_communities_user_community,priority:2"`
	CreatedAt   time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP"`
}

func (MutedCommunity) TableName() string {
	return "muted_communities"
}

type MutedUser struct {
	ID          uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	UserID      UID       `gorm:"column:user_id;not null;uniqueIndex:muted_users_user_muted,priority:1"`
	MutedUserID UID       `gorm:"column:muted_user_id;not null;uniqueIndex:muted_users_user_muted,priority:2"`
	CreatedAt   time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP"`
}

func (MutedUser) TableName() string {
	return "muted_users"
}

type Image struct {
	ID           UID        `gorm:"column:id;primaryKey"`
	StoreName    string     `gorm:"column:store_name;type:varchar(64);not null"`
	StoreMeta    JSONMap    `gorm:"column:store_metadata"`
	Format       string     `gorm:"column:format;type:varchar(16);not null"`
	Width        int        `gorm:"column:width;not null"`
	Height       int        `gorm:"column:height;not null"`
	Size         int        `gorm:"column:size;not null"`
	UploadSize   int        `gorm:"column:upload_size;not null"`
	AverageColor Bytes12    `gorm:"column:average_color"`
	CreatedAt    time.Time  `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP"`
	DeletedAt    *time.Time `gorm:"column:deleted_at"`
	AltText      *string    `gorm:"column:alt_text;type:varchar(1024)"`
}

func (Image) TableName() string {
	return "images"
}

type PostImage struct {
	ID      uint64 `gorm:"column:id;primaryKey;autoIncrement"`
	PostID  UID    `gorm:"column:post_id;not null;uniqueIndex:post_images_post_image,priority:1"`
	ImageID UID    `gorm:"column:image_id;not null;uniqueIndex:post_images_post_image,priority:2;uniqueIndex"`
	ZIndex  int    `gorm:"column:z_index;not null;default:0"`
}

func (PostImage) TableName() string {
	return "post_images"
}

type TempImage struct {
	ID        uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	UserID    UID       `gorm:"column:user_id;not null"`
	ImageID   UID       `gorm:"column:image_id;not null;uniqueIndex"`
	CreatedAt time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP;index"`
	ZIndex    int       `gorm:"column:z_index;not null;default:0"`
}

func (TempImage) TableName() string {
	return "temp_images"
}

type BadgeType struct {
	ID        uint      `gorm:"column:id;primaryKey;autoIncrement"`
	Name      string    `gorm:"column:name;type:varchar(64);not null;uniqueIndex"`
	CreatedAt time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP"`
}

func (BadgeType) TableName() string {
	return "badge_types"
}

type UserBadge struct {
	ID        uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	Type      uint      `gorm:"column:type;not null"`
	UserID    UID       `gorm:"column:user_id;not null;index"`
	CreatedAt time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP"`
}

func (UserBadge) TableName() string {
	return "user_badges"
}

type List struct {
	ID            uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	UserID        UID       `gorm:"column:user_id;not null;uniqueIndex:lists_user_name,priority:1;index:lists_user_created,priority:1"`
	Name          string    `gorm:"column:name;type:varchar(128);not null;uniqueIndex:lists_user_name,priority:2"`
	DisplayName   string    `gorm:"column:display_name;type:varchar(128);not null"`
	Public        bool      `gorm:"column:public;not null;default:false"`
	Description   *string   `gorm:"column:description;type:text"`
	NumItems      int       `gorm:"column:num_items;not null;default:0"`
	Ordering      int8      `gorm:"column:ordering;not null;default:0"`
	CreatedAt     time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP;index:lists_user_created,priority:2"`
	LastUpdatedAt time.Time `gorm:"column:last_updated_at;not null;default:CURRENT_TIMESTAMP"`
}

func (List) TableName() string {
	return "lists"
}

type ListItem struct {
	ID         uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	ListID     uint64    `gorm:"column:list_id;not null;uniqueIndex:list_items_list_target,priority:1;index:list_items_list_target_id,priority:1;index:list_items_list_created,priority:1"`
	TargetType int8      `gorm:"column:target_type;not null;uniqueIndex:list_items_list_target,priority:2"`
	TargetID   UID       `gorm:"column:target_id;not null;uniqueIndex:list_items_list_target,priority:3;index:list_items_list_target_id,priority:2"`
	CreatedAt  time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP;index:list_items_list_created,priority:2"`
}

func (ListItem) TableName() string {
	return "list_items"
}

type HiddenPost struct {
	ID        uint      `gorm:"column:id;primaryKey;autoIncrement"`
	UserID    UID       `gorm:"column:user_id;not null;uniqueIndex:hidden_posts_user_post,priority:1"`
	PostID    UID       `gorm:"column:post_id;not null;uniqueIndex:hidden_posts_user_post,priority:2"`
	CreatedAt time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP"`
}

func (HiddenPost) TableName() string {
	return "hidden_posts"
}

type AnnouncementPost struct {
	ID                uint       `gorm:"column:id;primaryKey;autoIncrement"`
	PostID            UID        `gorm:"column:post_id;not null;uniqueIndex"`
	AnnouncedBy       UID        `gorm:"column:announced_by;not null"`
	SendingStartedAt  *time.Time `gorm:"column:sending_started_at"`
	SendingFinishedAt *time.Time `gorm:"column:sending_finished_at"`
	TotalSent         int        `gorm:"column:total_sent;not null;default:0"`
}

func (AnnouncementPost) TableName() string {
	return "announcement_posts"
}

type AnnouncementNotificationSent struct {
	ID     uint      `gorm:"column:id;primaryKey;autoIncrement"`
	PostID UID       `gorm:"column:post_id;not null;uniqueIndex:announcement_notifications_sent_post_user,priority:1"`
	UserID UID       `gorm:"column:user_id;not null;uniqueIndex:announcement_notifications_sent_post_user,priority:2"`
	SentAt time.Time `gorm:"column:sent_at;not null;default:CURRENT_TIMESTAMP"`
}

func (AnnouncementNotificationSent) TableName() string {
	return "announcement_notifications_sent"
}

type PostVisit struct {
	ID             uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	PostID         UID       `gorm:"column:post_id;not null;uniqueIndex:post_visits_post_user,priority:1"`
	UserID         UID       `gorm:"column:user_id;not null;uniqueIndex:post_visits_post_user,priority:2"`
	FirstVisitedAt time.Time `gorm:"column:first_visited_at;not null;default:CURRENT_TIMESTAMP"`
	LastVisitedAt  time.Time `gorm:"column:last_visited_at;not null;default:CURRENT_TIMESTAMP"`
}

func (PostVisit) TableName() string {
	return "post_visits"
}

type IPBlock struct {
	ID              uint       `gorm:"column:id;primaryKey;autoIncrement"`
	IP              IP         `gorm:"column:ip;not null"`
	MaskedBits      uint8      `gorm:"column:masked_bits;not null;default:0"`
	CreatedAt       time.Time  `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP"`
	CreatedBy       UID        `gorm:"column:created_by;not null"`
	ExpiresAt       *time.Time `gorm:"column:expires_at"`
	CancelledAt     *time.Time `gorm:"column:cancelled_at"`
	InEffect        bool       `gorm:"column:in_effect;not null;default:true"`
	AssociatedUsers StringList `gorm:"column:associated_users"`
	Note            *string    `gorm:"column:note;type:text"`
}

func (IPBlock) TableName() string {
	return "ipblocks"
}

func AllModels() []any {
	return []any{
		&MigrationRecord{},
		&User{},
		&Community{},
		&CommunityMember{},
		&CommunityMod{},
		&CommunityBanned{},
		&Post{},
		&PostToday{},
		&PostWeek{},
		&PostMonth{},
		&PostYear{},
		&PostVote{},
		&Comment{},
		&CommentReply{},
		&CommentVote{},
		&PostsComment{},
		&CommunityRule{},
		&Notification{},
		&ReportReason{},
		&Report{},
		&DefaultCommunity{},
		&CommunityRequest{},
		&ApplicationData{},
		&WebPushSubscription{},
		&Analytics{},
		&PinnedPost{},
		&MutedCommunity{},
		&MutedUser{},
		&Image{},
		&PostImage{},
		&TempImage{},
		&BadgeType{},
		&UserBadge{},
		&List{},
		&ListItem{},
		&HiddenPost{},
		&AnnouncementPost{},
		&AnnouncementNotificationSent{},
		&PostVisit{},
		&IPBlock{},
	}
}
