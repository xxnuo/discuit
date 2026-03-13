package core

import (
	"context"
	"errors"
	"slices"
	"sort"
	"strconv"
	"time"

	idb "github.com/discuitnet/discuit/internal/db"
	"github.com/discuitnet/discuit/internal/uid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type MuteType string

func (t MuteType) Valid() bool {
	return slices.Contains([]MuteType{"", MuteTypeUser, MuteTypeCommunity}, t)
}

const (
	MuteTypeUser      = MuteType("user")
	MuteTypeCommunity = MuteType("community")
)

type Mute struct {
	ID               int       `json:"-"`
	PublicID         string    `json:"id"` // augmented id based on Type
	User             uid.ID    `json:"-"`
	Type             MuteType  `json:"type"`
	MutedUserID      *uid.ID   `json:"mutedUserId,omitempty"`      // may be empty and omitted base on Type
	MutedCommunityID *uid.ID   `json:"mutedCommunityId,omitempty"` // may be empty and omitted base on Type
	CreatedAt        time.Time `json:"createdAt"`

	MutedUser      *User      `json:"mutedUser,omitempty"`
	MutedCommunity *Community `json:"mutedCommunity,omitempty"`
}

func (m *Mute) setPrintID() {
	s := strconv.Itoa(m.ID)
	switch m.Type {
	case MuteTypeUser:
		s = "u_" + s
	case MuteTypeCommunity:
		s = "c_" + s
	default:
		panic("unknown mute type")
	}
	m.PublicID = s
}

func extractMuteID(s string) (t MuteType, id int, err error) {
	var errMuteID = errors.New("invalid mute id")
	if len(s) <= 2 {
		err = errMuteID
		return
	}

	prefix := s[:2]
	switch prefix {
	case "u_":
		t = MuteTypeUser
	case "c_":
		t = MuteTypeCommunity
	default:
		err = errMuteID
		return
	}

	id, err = strconv.Atoi(s[2:])
	if err != nil {
		err = errMuteID
	}
	return
}

func GetMutes(ctx context.Context, db *gorm.DB, user uid.ID) ([]*Mute, error) {
	communityMutes, err := GetMutedCommunities(ctx, db, user, true)
	if err != nil {
		return nil, err
	}
	userMutes, err := GetMutedUsers(ctx, db, user, true)
	if err != nil {
		return nil, err
	}

	all := append(communityMutes, userMutes...)
	sort.Slice(all, func(i, j int) bool {
		return all[i].CreatedAt.Before(all[j].CreatedAt)
	})
	if all == nil {
		all = []*Mute{} // for the json "[]" output
	}
	return all, nil
}

func GetMutedCommunities(ctx context.Context, db *gorm.DB, user uid.ID, fetchCommunities bool) ([]*Mute, error) {
	var rows []idb.MutedCommunity
	err := db.WithContext(ctx).Order("id").Where("user_id = ?", user).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	var mutes []*Mute
	for _, row := range rows {
		communityID := row.CommunityID.Raw()
		mutes = append(mutes, &Mute{
			ID:               int(row.ID),
			User:             user,
			Type:             MuteTypeCommunity,
			MutedCommunityID: &communityID,
			CreatedAt:        row.CreatedAt,
		})
	}

	var ids []uid.ID
	for i := range mutes {
		mutes[i].setPrintID()
		ids = append(ids, *mutes[i].MutedCommunityID)
	}

	if fetchCommunities {
		comms, err := GetCommunitiesByIDs(ctx, db, ids, nil)
		if err != nil {
			return nil, err
		}
		for _, comm := range comms {
			for _, mute := range mutes {
				if *mute.MutedCommunityID == comm.ID {
					mute.MutedCommunity = comm
					break
				}
			}
			comm.MutedByViewer = true
		}
	}
	return mutes, nil
}

func GetMutedUsers(ctx context.Context, db *gorm.DB, user uid.ID, fillUsers bool) ([]*Mute, error) {
	var rows []idb.MutedUser
	err := db.WithContext(ctx).Order("id").Where("user_id = ?", user).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	var mutes []*Mute
	for _, row := range rows {
		mutedUserID := row.MutedUserID.Raw()
		mutes = append(mutes, &Mute{
			ID:          int(row.ID),
			User:        user,
			Type:        MuteTypeUser,
			MutedUserID: &mutedUserID,
			CreatedAt:   row.CreatedAt,
		})
	}

	var ids []uid.ID
	for i := range mutes {
		mutes[i].setPrintID()
		ids = append(ids, *mutes[i].MutedUserID)
	}

	if fillUsers {
		users, err := GetUsersByIDs(ctx, db, ids, nil)
		if err != nil {
			return nil, err
		}
		for _, user := range users {
			for _, mute := range mutes {
				if user.ID == *mute.MutedUserID {
					mute.MutedUser = user
					break
				}
			}
		}
	}
	return mutes, nil
}

func Unmute(ctx context.Context, db *gorm.DB, user uid.ID, id string) error {
	mType, idInt, err := extractMuteID(id)
	if err != nil {
		return err
	}

	switch mType {
	case MuteTypeCommunity:
		err = db.WithContext(ctx).Where("id = ? AND user_id = ?", idInt, user).Delete(&idb.MutedCommunity{}).Error
	case MuteTypeUser:
		err = db.WithContext(ctx).Where("id = ? AND user_id = ?", idInt, user).Delete(&idb.MutedUser{}).Error
	}
	return err
}

// ClearMutes clears all mutes of user if t is empty, otherwise it clears either
// the community or the user mutes.
func ClearMutes(ctx context.Context, db *gorm.DB, user uid.ID, t MuteType) (err error) {
	if t == "" || t == MuteTypeCommunity {
		err = db.WithContext(ctx).Where("user_id = ?", user).Delete(&idb.MutedCommunity{}).Error
		if err != nil {
			return
		}
	}
	if t == "" || t == MuteTypeUser {
		err = db.WithContext(ctx).Where("user_id = ?", user).Delete(&idb.MutedUser{}).Error
	}
	return
}

func MuteCommunity(ctx context.Context, db *gorm.DB, user, community uid.ID) error {
	return db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&idb.MutedCommunity{
			UserID:      idb.UIDFrom(user),
			CommunityID: idb.UIDFrom(community),
		}).
		Error
}

func UnmuteCommunity(ctx context.Context, db *gorm.DB, user, community uid.ID) error {
	return db.WithContext(ctx).
		Where("user_id = ? AND community_id = ?", user, community).
		Delete(&idb.MutedCommunity{}).
		Error
}

func MuteUser(ctx context.Context, db *gorm.DB, user, mutedUser uid.ID) error {
	if is, err := UserDeleted(db, mutedUser); err != nil {
		return err
	} else if is {
		return ErrUserDeleted
	}

	return db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&idb.MutedUser{
			UserID:      idb.UIDFrom(user),
			MutedUserID: idb.UIDFrom(mutedUser),
		}).
		Error
}

func UnmuteUser(ctx context.Context, db *gorm.DB, user, mutedUser uid.ID) error {
	return db.WithContext(ctx).
		Where("user_id = ? AND muted_user_id = ?", user, mutedUser).
		Delete(&idb.MutedUser{}).
		Error
}
