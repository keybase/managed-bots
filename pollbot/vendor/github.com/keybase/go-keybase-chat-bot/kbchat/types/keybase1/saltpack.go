// Auto-generated to Go types using avdl-compiler v1.4.6 (https://github.com/keybase/node-avdl-compiler)
//   Input file: ../client/protocol/avdl/keybase1/saltpack.avdl

package keybase1

import (
	"fmt"
)

type AuthenticityType int

const (
	AuthenticityType_SIGNED     AuthenticityType = 0
	AuthenticityType_REPUDIABLE AuthenticityType = 1
	AuthenticityType_ANONYMOUS  AuthenticityType = 2
)

func (o AuthenticityType) DeepCopy() AuthenticityType { return o }

var AuthenticityTypeMap = map[string]AuthenticityType{
	"SIGNED":     0,
	"REPUDIABLE": 1,
	"ANONYMOUS":  2,
}

var AuthenticityTypeRevMap = map[AuthenticityType]string{
	0: "SIGNED",
	1: "REPUDIABLE",
	2: "ANONYMOUS",
}

func (e AuthenticityType) String() string {
	if v, ok := AuthenticityTypeRevMap[e]; ok {
		return v
	}
	return fmt.Sprintf("%v", int(e))
}

type SaltpackEncryptOptions struct {
	Recipients                []string         `codec:"recipients" json:"recipients"`
	TeamRecipients            []string         `codec:"teamRecipients" json:"teamRecipients"`
	AuthenticityType          AuthenticityType `codec:"authenticityType" json:"authenticityType"`
	UseEntityKeys             bool             `codec:"useEntityKeys" json:"useEntityKeys"`
	UseDeviceKeys             bool             `codec:"useDeviceKeys" json:"useDeviceKeys"`
	UsePaperKeys              bool             `codec:"usePaperKeys" json:"usePaperKeys"`
	NoSelfEncrypt             bool             `codec:"noSelfEncrypt" json:"noSelfEncrypt"`
	Binary                    bool             `codec:"binary" json:"binary"`
	SaltpackVersion           int              `codec:"saltpackVersion" json:"saltpackVersion"`
	UseKBFSKeysOnlyForTesting bool             `codec:"useKBFSKeysOnlyForTesting" json:"useKBFSKeysOnlyForTesting"`
}

func (o SaltpackEncryptOptions) DeepCopy() SaltpackEncryptOptions {
	return SaltpackEncryptOptions{
		Recipients: (func(x []string) []string {
			if x == nil {
				return nil
			}
			ret := make([]string, len(x))
			for i, v := range x {
				vCopy := v
				ret[i] = vCopy
			}
			return ret
		})(o.Recipients),
		TeamRecipients: (func(x []string) []string {
			if x == nil {
				return nil
			}
			ret := make([]string, len(x))
			for i, v := range x {
				vCopy := v
				ret[i] = vCopy
			}
			return ret
		})(o.TeamRecipients),
		AuthenticityType:          o.AuthenticityType.DeepCopy(),
		UseEntityKeys:             o.UseEntityKeys,
		UseDeviceKeys:             o.UseDeviceKeys,
		UsePaperKeys:              o.UsePaperKeys,
		NoSelfEncrypt:             o.NoSelfEncrypt,
		Binary:                    o.Binary,
		SaltpackVersion:           o.SaltpackVersion,
		UseKBFSKeysOnlyForTesting: o.UseKBFSKeysOnlyForTesting,
	}
}

type SaltpackDecryptOptions struct {
	Interactive      bool `codec:"interactive" json:"interactive"`
	ForceRemoteCheck bool `codec:"forceRemoteCheck" json:"forceRemoteCheck"`
	UsePaperKey      bool `codec:"usePaperKey" json:"usePaperKey"`
}

func (o SaltpackDecryptOptions) DeepCopy() SaltpackDecryptOptions {
	return SaltpackDecryptOptions{
		Interactive:      o.Interactive,
		ForceRemoteCheck: o.ForceRemoteCheck,
		UsePaperKey:      o.UsePaperKey,
	}
}

type SaltpackSignOptions struct {
	Detached        bool `codec:"detached" json:"detached"`
	Binary          bool `codec:"binary" json:"binary"`
	SaltpackVersion int  `codec:"saltpackVersion" json:"saltpackVersion"`
}

func (o SaltpackSignOptions) DeepCopy() SaltpackSignOptions {
	return SaltpackSignOptions{
		Detached:        o.Detached,
		Binary:          o.Binary,
		SaltpackVersion: o.SaltpackVersion,
	}
}

type SaltpackVerifyOptions struct {
	SignedBy  string `codec:"signedBy" json:"signedBy"`
	Signature []byte `codec:"signature" json:"signature"`
}

func (o SaltpackVerifyOptions) DeepCopy() SaltpackVerifyOptions {
	return SaltpackVerifyOptions{
		SignedBy: o.SignedBy,
		Signature: (func(x []byte) []byte {
			if x == nil {
				return nil
			}
			return append([]byte{}, x...)
		})(o.Signature),
	}
}

type SaltpackEncryptedMessageInfo struct {
	Devices          []Device       `codec:"devices" json:"devices"`
	NumAnonReceivers int            `codec:"numAnonReceivers" json:"numAnonReceivers"`
	ReceiverIsAnon   bool           `codec:"receiverIsAnon" json:"receiverIsAnon"`
	Sender           SaltpackSender `codec:"sender" json:"sender"`
}

func (o SaltpackEncryptedMessageInfo) DeepCopy() SaltpackEncryptedMessageInfo {
	return SaltpackEncryptedMessageInfo{
		Devices: (func(x []Device) []Device {
			if x == nil {
				return nil
			}
			ret := make([]Device, len(x))
			for i, v := range x {
				vCopy := v.DeepCopy()
				ret[i] = vCopy
			}
			return ret
		})(o.Devices),
		NumAnonReceivers: o.NumAnonReceivers,
		ReceiverIsAnon:   o.ReceiverIsAnon,
		Sender:           o.Sender.DeepCopy(),
	}
}
