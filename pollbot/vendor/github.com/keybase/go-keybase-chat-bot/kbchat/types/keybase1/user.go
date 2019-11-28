// Auto-generated to Go types using avdl-compiler v1.4.6 (https://github.com/keybase/node-avdl-compiler)
//   Input file: ../client/protocol/avdl/keybase1/user.avdl

package keybase1

import (
	"fmt"
)

type TrackProof struct {
	ProofType string `codec:"proofType" json:"proofType"`
	ProofName string `codec:"proofName" json:"proofName"`
	IdString  string `codec:"idString" json:"idString"`
}

func (o TrackProof) DeepCopy() TrackProof {
	return TrackProof{
		ProofType: o.ProofType,
		ProofName: o.ProofName,
		IdString:  o.IdString,
	}
}

type WebProof struct {
	Hostname  string   `codec:"hostname" json:"hostname"`
	Protocols []string `codec:"protocols" json:"protocols"`
}

func (o WebProof) DeepCopy() WebProof {
	return WebProof{
		Hostname: o.Hostname,
		Protocols: (func(x []string) []string {
			if x == nil {
				return nil
			}
			ret := make([]string, len(x))
			for i, v := range x {
				vCopy := v
				ret[i] = vCopy
			}
			return ret
		})(o.Protocols),
	}
}

type Proofs struct {
	Social     []TrackProof `codec:"social" json:"social"`
	Web        []WebProof   `codec:"web" json:"web"`
	PublicKeys []PublicKey  `codec:"publicKeys" json:"publicKeys"`
}

func (o Proofs) DeepCopy() Proofs {
	return Proofs{
		Social: (func(x []TrackProof) []TrackProof {
			if x == nil {
				return nil
			}
			ret := make([]TrackProof, len(x))
			for i, v := range x {
				vCopy := v.DeepCopy()
				ret[i] = vCopy
			}
			return ret
		})(o.Social),
		Web: (func(x []WebProof) []WebProof {
			if x == nil {
				return nil
			}
			ret := make([]WebProof, len(x))
			for i, v := range x {
				vCopy := v.DeepCopy()
				ret[i] = vCopy
			}
			return ret
		})(o.Web),
		PublicKeys: (func(x []PublicKey) []PublicKey {
			if x == nil {
				return nil
			}
			ret := make([]PublicKey, len(x))
			for i, v := range x {
				vCopy := v.DeepCopy()
				ret[i] = vCopy
			}
			return ret
		})(o.PublicKeys),
	}
}

type UserSummary struct {
	Uid          UID    `codec:"uid" json:"uid"`
	Username     string `codec:"username" json:"username"`
	Thumbnail    string `codec:"thumbnail" json:"thumbnail"`
	IdVersion    int    `codec:"idVersion" json:"idVersion"`
	FullName     string `codec:"fullName" json:"fullName"`
	Bio          string `codec:"bio" json:"bio"`
	Proofs       Proofs `codec:"proofs" json:"proofs"`
	SigIDDisplay string `codec:"sigIDDisplay" json:"sigIDDisplay"`
	TrackTime    Time   `codec:"trackTime" json:"trackTime"`
}

func (o UserSummary) DeepCopy() UserSummary {
	return UserSummary{
		Uid:          o.Uid.DeepCopy(),
		Username:     o.Username,
		Thumbnail:    o.Thumbnail,
		IdVersion:    o.IdVersion,
		FullName:     o.FullName,
		Bio:          o.Bio,
		Proofs:       o.Proofs.DeepCopy(),
		SigIDDisplay: o.SigIDDisplay,
		TrackTime:    o.TrackTime.DeepCopy(),
	}
}

type EmailAddress string

func (o EmailAddress) DeepCopy() EmailAddress {
	return o
}

type Email struct {
	Email               EmailAddress       `codec:"email" json:"email"`
	IsVerified          bool               `codec:"isVerified" json:"isVerified"`
	IsPrimary           bool               `codec:"isPrimary" json:"isPrimary"`
	Visibility          IdentityVisibility `codec:"visibility" json:"visibility"`
	LastVerifyEmailDate UnixTime           `codec:"lastVerifyEmailDate" json:"lastVerifyEmailDate"`
}

func (o Email) DeepCopy() Email {
	return Email{
		Email:               o.Email.DeepCopy(),
		IsVerified:          o.IsVerified,
		IsPrimary:           o.IsPrimary,
		Visibility:          o.Visibility.DeepCopy(),
		LastVerifyEmailDate: o.LastVerifyEmailDate.DeepCopy(),
	}
}

type UserSettings struct {
	Emails       []Email           `codec:"emails" json:"emails"`
	PhoneNumbers []UserPhoneNumber `codec:"phoneNumbers" json:"phoneNumbers"`
}

func (o UserSettings) DeepCopy() UserSettings {
	return UserSettings{
		Emails: (func(x []Email) []Email {
			if x == nil {
				return nil
			}
			ret := make([]Email, len(x))
			for i, v := range x {
				vCopy := v.DeepCopy()
				ret[i] = vCopy
			}
			return ret
		})(o.Emails),
		PhoneNumbers: (func(x []UserPhoneNumber) []UserPhoneNumber {
			if x == nil {
				return nil
			}
			ret := make([]UserPhoneNumber, len(x))
			for i, v := range x {
				vCopy := v.DeepCopy()
				ret[i] = vCopy
			}
			return ret
		})(o.PhoneNumbers),
	}
}

type UserSummary2 struct {
	Uid        UID    `codec:"uid" json:"uid"`
	Username   string `codec:"username" json:"username"`
	Thumbnail  string `codec:"thumbnail" json:"thumbnail"`
	FullName   string `codec:"fullName" json:"fullName"`
	IsFollower bool   `codec:"isFollower" json:"isFollower"`
	IsFollowee bool   `codec:"isFollowee" json:"isFollowee"`
}

func (o UserSummary2) DeepCopy() UserSummary2 {
	return UserSummary2{
		Uid:        o.Uid.DeepCopy(),
		Username:   o.Username,
		Thumbnail:  o.Thumbnail,
		FullName:   o.FullName,
		IsFollower: o.IsFollower,
		IsFollowee: o.IsFollowee,
	}
}

type UserSummary2Set struct {
	Users   []UserSummary2 `codec:"users" json:"users"`
	Time    Time           `codec:"time" json:"time"`
	Version int            `codec:"version" json:"version"`
}

func (o UserSummary2Set) DeepCopy() UserSummary2Set {
	return UserSummary2Set{
		Users: (func(x []UserSummary2) []UserSummary2 {
			if x == nil {
				return nil
			}
			ret := make([]UserSummary2, len(x))
			for i, v := range x {
				vCopy := v.DeepCopy()
				ret[i] = vCopy
			}
			return ret
		})(o.Users),
		Time:    o.Time.DeepCopy(),
		Version: o.Version,
	}
}

type InterestingPerson struct {
	Uid        UID               `codec:"uid" json:"uid"`
	Username   string            `codec:"username" json:"username"`
	Fullname   string            `codec:"fullname" json:"fullname"`
	ServiceMap map[string]string `codec:"serviceMap" json:"serviceMap"`
}

func (o InterestingPerson) DeepCopy() InterestingPerson {
	return InterestingPerson{
		Uid:      o.Uid.DeepCopy(),
		Username: o.Username,
		Fullname: o.Fullname,
		ServiceMap: (func(x map[string]string) map[string]string {
			if x == nil {
				return nil
			}
			ret := make(map[string]string, len(x))
			for k, v := range x {
				kCopy := k
				vCopy := v
				ret[kCopy] = vCopy
			}
			return ret
		})(o.ServiceMap),
	}
}

type ProofSuggestionsRes struct {
	Suggestions []ProofSuggestion `codec:"suggestions" json:"suggestions"`
	ShowMore    bool              `codec:"showMore" json:"showMore"`
}

func (o ProofSuggestionsRes) DeepCopy() ProofSuggestionsRes {
	return ProofSuggestionsRes{
		Suggestions: (func(x []ProofSuggestion) []ProofSuggestion {
			if x == nil {
				return nil
			}
			ret := make([]ProofSuggestion, len(x))
			for i, v := range x {
				vCopy := v.DeepCopy()
				ret[i] = vCopy
			}
			return ret
		})(o.Suggestions),
		ShowMore: o.ShowMore,
	}
}

type ProofSuggestion struct {
	Key              string             `codec:"key" json:"key"`
	BelowFold        bool               `codec:"belowFold" json:"belowFold"`
	ProfileText      string             `codec:"profileText" json:"profileText"`
	ProfileIcon      []SizedImage       `codec:"profileIcon" json:"profileIcon"`
	ProfileIconWhite []SizedImage       `codec:"profileIconWhite" json:"profileIconWhite"`
	PickerText       string             `codec:"pickerText" json:"pickerText"`
	PickerSubtext    string             `codec:"pickerSubtext" json:"pickerSubtext"`
	PickerIcon       []SizedImage       `codec:"pickerIcon" json:"pickerIcon"`
	Metas            []Identify3RowMeta `codec:"metas" json:"metas"`
}

func (o ProofSuggestion) DeepCopy() ProofSuggestion {
	return ProofSuggestion{
		Key:         o.Key,
		BelowFold:   o.BelowFold,
		ProfileText: o.ProfileText,
		ProfileIcon: (func(x []SizedImage) []SizedImage {
			if x == nil {
				return nil
			}
			ret := make([]SizedImage, len(x))
			for i, v := range x {
				vCopy := v.DeepCopy()
				ret[i] = vCopy
			}
			return ret
		})(o.ProfileIcon),
		ProfileIconWhite: (func(x []SizedImage) []SizedImage {
			if x == nil {
				return nil
			}
			ret := make([]SizedImage, len(x))
			for i, v := range x {
				vCopy := v.DeepCopy()
				ret[i] = vCopy
			}
			return ret
		})(o.ProfileIconWhite),
		PickerText:    o.PickerText,
		PickerSubtext: o.PickerSubtext,
		PickerIcon: (func(x []SizedImage) []SizedImage {
			if x == nil {
				return nil
			}
			ret := make([]SizedImage, len(x))
			for i, v := range x {
				vCopy := v.DeepCopy()
				ret[i] = vCopy
			}
			return ret
		})(o.PickerIcon),
		Metas: (func(x []Identify3RowMeta) []Identify3RowMeta {
			if x == nil {
				return nil
			}
			ret := make([]Identify3RowMeta, len(x))
			for i, v := range x {
				vCopy := v.DeepCopy()
				ret[i] = vCopy
			}
			return ret
		})(o.Metas),
	}
}

type NextMerkleRootRes struct {
	Res *MerkleRootV2 `codec:"res,omitempty" json:"res,omitempty"`
}

func (o NextMerkleRootRes) DeepCopy() NextMerkleRootRes {
	return NextMerkleRootRes{
		Res: (func(x *MerkleRootV2) *MerkleRootV2 {
			if x == nil {
				return nil
			}
			tmp := (*x).DeepCopy()
			return &tmp
		})(o.Res),
	}
}

// PassphraseState values are used in .config.json, so should not be changed without a migration strategy
type PassphraseState int

const (
	PassphraseState_KNOWN  PassphraseState = 0
	PassphraseState_RANDOM PassphraseState = 1
)

func (o PassphraseState) DeepCopy() PassphraseState { return o }

var PassphraseStateMap = map[string]PassphraseState{
	"KNOWN":  0,
	"RANDOM": 1,
}

var PassphraseStateRevMap = map[PassphraseState]string{
	0: "KNOWN",
	1: "RANDOM",
}

func (e PassphraseState) String() string {
	if v, ok := PassphraseStateRevMap[e]; ok {
		return v
	}
	return fmt.Sprintf("%v", int(e))
}

type CanLogoutRes struct {
	CanLogout       bool            `codec:"canLogout" json:"canLogout"`
	Reason          string          `codec:"reason" json:"reason"`
	PassphraseState PassphraseState `codec:"passphraseState" json:"passphraseState"`
}

func (o CanLogoutRes) DeepCopy() CanLogoutRes {
	return CanLogoutRes{
		CanLogout:       o.CanLogout,
		Reason:          o.Reason,
		PassphraseState: o.PassphraseState.DeepCopy(),
	}
}

type UserPassphraseStateMsg struct {
	PassphraseState PassphraseState `codec:"passphraseState" json:"state"`
}

func (o UserPassphraseStateMsg) DeepCopy() UserPassphraseStateMsg {
	return UserPassphraseStateMsg{
		PassphraseState: o.PassphraseState.DeepCopy(),
	}
}
