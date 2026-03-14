package core

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	idb "github.com/discuitnet/discuit/internal/db"
	"github.com/discuitnet/discuit/internal/httperr"
	msql "github.com/discuitnet/discuit/internal/sql"
	"github.com/discuitnet/discuit/internal/uid"
	"gorm.io/gorm"
)

// ReportType represents the type of user submitted report.
type ReportType int

const (
	ReportTypeAll = ReportType(iota - 1) // pseudo type
	ReportTypePost
	ReportTypeComment
)

// MarshalText implements the encoding.TextMarshaler interface.
func (r ReportType) MarshalText() ([]byte, error) {
	s := ""
	switch r {
	case ReportTypePost:
		s = "post"
	case ReportTypeComment:
		s = "comment"
	default:
		return nil, errors.New("unsupported report type")
	}
	return []byte(s), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (r *ReportType) UnmarshalText(text []byte) error {
	switch string(text) {
	case "post":
		*r = ReportTypePost
	case "comment":
		*r = ReportTypeComment
	default:
		return errors.New("unsupported report type")
	}
	return nil
}

// Report is a user submitted report.
type Report struct {
	ID          int             `json:"id"`
	CommunityID uid.ID          `json:"communityId"`
	PostID      uid.NullID      `json:"postId"`
	Reason      string          `json:"reason"`
	Description msql.NullString `json:"description"`
	ReasonID    int             `json:"reasonId"`
	Type        ReportType      `json:"type"` // post or comment
	TargetID    uid.ID          `json:"targetId"`
	CreatedBy   uid.ID          `json:"-"`
	ActionTaken msql.NullString `json:"actionTaken"`
	DealtAt     msql.NullTime   `json:"dealtAt"`
	DealtBy     uid.NullID      `json:"dealtBy"`
	CreatedAt   time.Time       `json:"createdAt"`

	Target interface{} `json:"target"`
}

var selectReportCols = []string{
	"reports.id",
	"reports.community_id",
	"reports.post_id",
	"reports.reason_id",
	"reports.report_type",
	"reports.target_id",
	"reports.created_by",
	"reports.action_taken",
	"reports.dealt_at",
	"reports.dealt_by",
	"reports.created_at",
	"report_reasons.title",
	"report_reasons.description",
}

var selectReportJoins = []string{
	"INNER JOIN report_reasons ON reports.reason_id = report_reasons.id",
}

// NewReport creates a new report on target.
func NewReport(ctx context.Context, db *gorm.DB, community uid.ID, post uid.NullID, t ReportType, reason int, target, createdBy uid.ID) (*Report, error) {
	if is, err := IsUserBannedFromCommunity(ctx, db, community, createdBy); err != nil {
		return nil, err
	} else if is {
		return nil, errUserBannedFromCommunity
	}

	has, err := hasUserMadeReport(ctx, db, createdBy, target, t, reason)
	if err != nil {
		return nil, err
	}
	if has {
		return nil, &httperr.Error{HTTPStatus: http.StatusConflict, Code: "already-voted", Message: "User has already voted."}
	}

	record := idb.Report{
		CommunityID: idb.UIDFrom(community),
		ReasonID:    uint(reason),
		ReportType:  int8(t),
		TargetID:    idb.UIDFrom(target),
		CreatedBy:   idb.UIDFrom(createdBy),
	}
	if post.Valid {
		postID := idb.UIDFrom(post.ID)
		record.PostID = &postID
	}

	if err := db.WithContext(ctx).Create(&record).Error; err != nil {
		return nil, err
	}
	return GetReport(ctx, db, int(record.ID))
}

// NewPostReport creates a report on post.
func NewPostReport(ctx context.Context, db *gorm.DB, post uid.ID, reason int, createdBy uid.ID) (*Report, error) {
	p, err := GetPost(ctx, db, &post, "", nil, true)
	if err != nil {
		return nil, err
	}
	ni := uid.NullID{ID: p.ID, Valid: true}
	return NewReport(ctx, db, p.CommunityID, ni, ReportTypePost, reason, p.ID, createdBy)
}

// NewCommentReport creates a report on comment.
func NewCommentReport(ctx context.Context, db *gorm.DB, comment uid.ID, reason int, createdBy uid.ID) (*Report, error) {
	c, err := GetComment(ctx, db, comment, nil)
	if err != nil {
		return nil, err
	}
	ni := uid.NullID{ID: c.PostID, Valid: true}
	return NewReport(ctx, db, c.CommunityID, ni, ReportTypeComment, reason, c.ID, createdBy)
}

func hasUserMadeReport(ctx context.Context, db *gorm.DB, userID, targetID uid.ID, t ReportType, reasonID int) (bool, error) {
	var report idb.Report
	err := db.WithContext(ctx).
		Select("id").
		Where("created_by = ? AND target_id = ? AND report_type = ? AND reason_id = ?", userID, targetID, t, reasonID).
		Take(&report).
		Error
	if err != nil {
		if idb.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func scanReports(db *gorm.DB, rows *sql.Rows) ([]*Report, error) {
	defer rows.Close()

	var reports []*Report
	for rows.Next() {
		r := &Report{}
		err := rows.Scan(
			&r.ID,
			&r.CommunityID,
			&r.PostID,
			&r.ReasonID,
			&r.Type,
			&r.TargetID,
			&r.CreatedBy,
			&r.ActionTaken,
			&r.DealtAt,
			&r.DealtBy,
			&r.CreatedAt,
			&r.Reason,
			&r.Description)
		if err != nil {
			return nil, err
		}
		reports = append(reports, r)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(reports) == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	return reports, nil
}

// GetReport returns an error if the report is not found.
func GetReport(ctx context.Context, db *gorm.DB, reportID int) (*Report, error) {
	rows, err := idb.Select(ctx, db, "reports", selectReportCols, idb.NewJoin(selectReportJoins[0])).
		Where("reports.id = ?", reportID).
		Rows()
	if err != nil {
		return nil, err
	}

	reports, err := scanReports(db, rows)
	if err != nil {
		return nil, err
	}
	return reports[0], nil
}

// FetchTarget populates r.Target with the target object on which the report was
// made.
func (r *Report) FetchTarget(ctx context.Context, db *gorm.DB) error {
	if r.Type == ReportTypePost {
		post, err := GetPost(ctx, db, &r.TargetID, "", nil, true)
		if err != nil {
			return err
		}
		r.Target = post
	} else if r.Type == ReportTypeComment {
		comment, err := GetComment(ctx, db, r.TargetID, nil)
		if err != nil {
			return err
		}
		if err = comment.loadPostDeleted(ctx, db); err != nil {
			return err
		}
		r.Target = comment
	}
	return nil
}

// Delete deletes the report permanently.
func (r *Report) Delete(ctx context.Context, db *gorm.DB, mod uid.ID) error {
	return db.WithContext(ctx).Where("id = ?", r.ID).Delete(&idb.Report{}).Error
}

// GetReports retrives user submitted reports in community. The results are paginated.
func GetReports(ctx context.Context, db *gorm.DB, community uid.ID, t ReportType, limit, page int) ([]*Report, error) {
	offset := limit * (page - 1)
	query := idb.Select(ctx, db, "reports", selectReportCols, idb.NewJoin(selectReportJoins[0])).
		Where("reports.community_id = ?", community).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset)
	if t != ReportTypeAll {
		query = query.Where("report_type = ?", t)
	}

	rows, err := query.Rows()
	if err != nil {
		return nil, err
	}

	reports, err := scanReports(db, rows)
	if err != nil {
		return nil, err
	}
	for _, r := range reports {
		if err = r.FetchTarget(ctx, db); err != nil {
			return nil, errors.New("couldn't fetch target: " + err.Error())
		}
	}
	return reports, nil
}

type ReportReason struct {
	ID          int             `json:"id"`
	Title       string          `json:"title"`
	Description msql.NullString `json:"description"`
	CreatedAt   time.Time       `json:"-"`
}

// GetReportReasons returns all report reasons. These are supposed to be about a
// dozen at most.
func GetReportReasons(ctx context.Context, db *gorm.DB) ([]ReportReason, error) {
	var all []ReportReason
	rows, err := idb.Select(ctx, db, "report_reasons", []string{"id", "title", "description", "created_at"}).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		r := ReportReason{}
		if err = rows.Scan(&r.ID, &r.Title, &r.Description, &r.CreatedAt); err != nil {
			return nil, err
		}
		all = append(all, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return all, nil
}

func RemoveAllReportsOfCommunity(ctx context.Context, db *gorm.DB, community uid.ID) error {
	return db.WithContext(ctx).Where("community_id = ?", community).Delete(&idb.Report{}).Error
}

func RemoveAllReportsOfPost(ctx context.Context, db *gorm.DB, post uid.ID) error {
	return db.WithContext(ctx).Where("post_id = ?", post).Delete(&idb.Report{}).Error
}

func RemoveAllReportsOfComment(ctx context.Context, db *gorm.DB, comment uid.ID) error {
	return db.WithContext(ctx).
		Where("target_id = ? AND report_type = ?", comment, ReportTypeComment).
		Delete(&idb.Report{}).
		Error
}
