// Copyright 2018 NetApp, Inc. All Rights Reserved.

package santricity

import "fmt"

var HostTypes = map[string]string{
	"linux_atto":        "LnxTPGSALUA",
	"linux_dm_mp":       "LnxDHALUA", // Updated to modern default (index 28)
	"linux_mpp_rdac":    "LNX",
	"linux_pathmanager": "LnxTPGSALUA_PM",
	"linux_sf":          "LnxTPGSALUA_SF",
	"ontap":             "ONTAP_ALUA",
	"ontap_rdac":        "ONTAP_RDAC",
	"vmware":            "VmwTPGSALUA",
	"windows":           "W2KNETNCL",
	"windows_atto":      "WinTPGSALUA",
	"windows_clustered": "W2KNETCL",
}

// Error wrapper
type Error struct {
	Code    int
	Message string
}

func (e Error) Error() string {
	return fmt.Sprintf("device API error: %s, Status code: %d", e.Message, e.Code)
}

// AboutResponse includes basic information about the system.
type AboutResponse struct {
	RunningAsProxy     bool   `json:"runningAsProxy"`     // "runningAsProxy": false,
	Version            string `json:"version"`            // "version": "03.20.9000.0004",
	SystemID           string `json:"systemId"`           // "systemId": "de679125-1c38-4356-b175-810c8b536d6a",
	ControllerPosition int    `json:"controllerPosition"` // "controllerPosition": 1,
	StartTimestamp     string `json:"startTimestamp"`     // "startTimestamp": "1573141561",
	SamlEnabled        bool   `json:"samlEnabled"`        // "samlEnabled": false
}

type StorageSystem struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Wwn             string   `json:"wwn"`
	PasswordStatus  string   `json:"passwordStatus"`
	PasswordSet     bool     `json:"passwordSet"`
	Status          string   `json:"status"`
	IP1             string   `json:"ip1"`
	IP2             string   `json:"ip2"`
	ManagementPaths []string `json:"managementPaths"`
	Controllers     []struct {
		ControllerID      string   `json:"controllerId"`
		IPAddresses       []string `json:"ipAddresses"`
		CertificateStatus string   `json:"certificateStatus"`
	} `json:"controllers"`
	DriveCount                        int         `json:"driveCount"`
	TrayCount                         int         `json:"trayCount"`
	TraceEnabled                      bool        `json:"traceEnabled"`
	Types                             string      `json:"types"`
	Model                             string      `json:"model"`
	MetaTags                          []KeyValues `json:"metaTags"`
	HotSpareSize                      string      `json:"hotSpareSize"`
	UsedPoolSpace                     string      `json:"usedPoolSpace"`
	FreePoolSpace                     string      `json:"freePoolSpace"`
	UnconfiguredSpace                 string      `json:"unconfiguredSpace"`
	DriveTypes                        []string    `json:"driveTypes"`
	HostSpareCountInStandby           int         `json:"hostSpareCountInStandby"`
	HotSpareCount                     int         `json:"hotSpareCount"`
	HostSparesUsed                    int         `json:"hostSparesUsed"`
	ResourceProvisionedVolumesEnabled bool        `json:"resourceProvisionedVolumesEnabled"`
	BootTime                          string      `json:"bootTime"`
	FwVersion                         string      `json:"fwVersion"`
	AppVersion                        string      `json:"appVersion"`
	BootVersion                       string      `json:"bootVersion"`
	NvsramVersion                     string      `json:"nvsramVersion"`
	ChassisSerialNumber               string      `json:"chassisSerialNumber"`
	AccessVolume                      struct {
		Enabled               bool   `json:"enabled"`
		VolumeHandle          int    `json:"volumeHandle"`
		Capacity              string `json:"capacity"`
		AccessVolumeRef       string `json:"accessVolumeRef"`
		Reserved1             string `json:"reserved1"`
		ObjectType            string `json:"objectType"`
		Wwn                   string `json:"wwn"`
		PreferredControllerID string `json:"preferredControllerId"`
		TotalSizeInBytes      string `json:"totalSizeInBytes"`
		ListOfMappings        []struct {
			LunMappingRef string `json:"lunMappingRef"`
			Lun           int    `json:"lun"`
			Ssid          int    `json:"ssid"`
			Perms         int    `json:"perms"`
			VolumeRef     string `json:"volumeRef"`
			Type          string `json:"type"`
			MapRef        string `json:"mapRef"`
			ID            string `json:"id"`
		} `json:"listOfMappings"`
		Mapped              bool   `json:"mapped"`
		CurrentControllerID string `json:"currentControllerId"`
		Name                string `json:"name"`
		ID                  string `json:"id"`
	} `json:"accessVolume"`
	UnconfiguredSpaceByDriveType     map[string]string `json:"unconfiguredSpaceByDriveType"`
	MediaScanPeriod                  int               `json:"mediaScanPeriod"`
	DriveChannelPortDisabled         bool              `json:"driveChannelPortDisabled"`
	RecoveryModeEnabled              bool              `json:"recoveryModeEnabled"`
	AutoLoadBalancingEnabled         bool              `json:"autoLoadBalancingEnabled"`
	HostConnectivityReportingEnabled bool              `json:"hostConnectivityReportingEnabled"`
	RemoteMirroringEnabled           bool              `json:"remoteMirroringEnabled"`
	FcRemoteMirroringState           string            `json:"fcRemoteMirroringState"`
	AsupEnabled                      bool              `json:"asupEnabled"`
	SecurityKeyEnabled               bool              `json:"securityKeyEnabled"`
	ExternalKeyEnabled               bool              `json:"externalKeyEnabled"`
	LastContacted                    string            `json:"lastContacted"`
	DefinedPartitionCount            int               `json:"definedPartitionCount"`
	SimplexModeEnabled               bool              `json:"simplexModeEnabled"`
	SupportedManagementPorts         []string          `json:"supportedManagementPorts"`
	InvalidSystemConfig              bool              `json:"invalidSystemConfig"`
	FreePoolSpaceAsString            string            `json:"freePoolSpaceAsString"`
	HotSpareSizeAsString             string            `json:"hotSpareSizeAsString"`
	UnconfiguredSpaceAsStrings       string            `json:"unconfiguredSpaceAsStrings"`
	UsedPoolSpaceAsString            string            `json:"usedPoolSpaceAsString"`
}

type KeyValues struct {
	Key       string   `json:"key"`
	ValueList []string `json:"valueList"`
}

type VolumeGroupEx struct {
	IsOffline          bool   `json:"offline"`
	WorldWideName      string `json:"worldWideName"`
	VolumeGroupRef     string `json:"volumeGroupRef"`
	Label              string `json:"label"`
	FreeSpace          string `json:"freeSpace"`         // Documentation says this is an int but really it is a string!
	DriveMediaType     string `json:"driveMediaType"`    // 'hdd', 'ssd'
	DrivePhysicalType  string `json:"drivePhysicalType"` // 'sas', 'nvme4k', etc.
	RaidLevel          string `json:"raidLevel"`
	BlkSizeSupported   []int  `json:"blkSizeSupported"`
	BlkSizeRecommended int    `json:"blkSizeRecommended"`
}

// Functions to allow sorting storage pools by free space
type ByFreeSpace []VolumeGroupEx

func (s ByFreeSpace) Len() int {
	return len(s)
}
func (s ByFreeSpace) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s ByFreeSpace) Less(i, j int) bool {
	return len(s[i].FreeSpace) < len(s[j].FreeSpace)
}

type VolumeCreateRequest struct {
	VolumeGroupRef   string      `json:"poolId"`
	Name             string      `json:"name"`
	SizeUnit         string      `json:"sizeUnit"` //bytes, b, kb, mb, gb, tb, pb, eb, zb, yb
	Size             string      `json:"size"`
	SegmentSize      int         `json:"segSize,omitempty"`
	DataAssurance    bool        `json:"dataAssuranceEnabled,omitempty"`
	OwningController string      `json:"owningControllerId,omitempty"`
	RaidLevel        string      `json:"raidLevel,omitempty"` // raidUnsupported, raidAll, raid0, raid1, raid3, raid5, raid6, raidDiskPool, raid1p
	BlockSize        int         `json:"blockSize,omitempty"`
	VolumeTags       []VolumeTag `json:"metaTags,omitempty"`
}

// VolumeUpdateRequest is used to update a volume
type VolumeUpdateRequest struct {
	Name       string      `json:"name,omitempty"`     // the new name (to rename it)
	VolumeTags []VolumeTag `json:"metaTags,omitempty"` // Key/Value pair for volume meta data
}

type VolumeTag struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func (t VolumeTag) Equals(otherTag VolumeTag) bool {
	return t.Key == otherTag.Key && t.Value == otherTag.Value
}

type VolumeEx struct {
	IsOffline      bool         `json:"offline"`
	Label          string       `json:"label"`
	VolumeSize     string       `json:"capacity"`
	SegmentSize    int          `json:"segmentSize"`
	VolumeRef      string       `json:"volumeRef"`
	WorldWideName  string       `json:"worldWideName"`
	VolumeGroupRef string       `json:"volumeGroupRef"`
	RaidLevel      string       `json:"raidLevel"` // "raidDiskPool", "raid6", etc.
	BlockSize      int          `json:"blkSize"`
	Mappings       []LUNMapping `json:"listOfMappings"`
	IsMapped       bool         `json:"mapped"`
	VolumeTags     []VolumeTag  `json:"metadata"`
	VolumeUse      string       `json:"volumeUse,omitempty"` // "standardVolume", "freeRepositoryVolume", etc.
}

type VolumeResizeRequest struct {
	ExpansionSize int    `json:"expansionSize"`
	SizeUnit      string `json:"sizeUnit"` //bytes, b, kb, mb, gb, tb, pb, eb, zb, yb
}

type VolumeResizeStatusResponse struct {
	PercentComplete  int    `json:"percentComplete"`
	TimeToCompletion int    `json:"timeToCompletion"`
	Action           string `json:"action"`
}

type HostCreateRequest struct {
	Name     string `json:"name"`
	HostType struct {
		Index int    `json:"index"`
		Code  string `json:"code,omitempty"`
	} `json:"hostType"`
	GroupID string     `json:"groupId,omitempty"`
	Ports   []HostPort `json:"ports"`
}

type HostType struct {
	Name  string `json:"name,omitempty"`
	Index int    `json:"index"`
	Code  string `json:"code,omitempty"`
}

type HostPort struct {
	Type            string `json:"type"`
	Port            string `json:"port"`
	Label           string `json:"label"`
	IscsiChapSecret string `json:"iscsiChapSecret,omitempty"`
}

type HostEx struct {
	HostRef       string            `json:"hostRef"`
	ClusterRef    string            `json:"clusterRef"`
	Label         string            `json:"label"`
	HostTypeIndex int               `json:"hostTypeIndex"`
	Initiators    []HostExInitiator `json:"initiators"`
}

type HostExInitiator struct {
	InitiatorRef      string                  `json:"initiatorRef"`
	NodeName          HostExScsiNodeName      `json:"nodeName"`
	Label             string                  `json:"label"`
	InitiatorNodeName HostExInitiatorNodeName `json:"initiatorNodeName"`
}

type HostExInitiatorNodeName struct {
	NodeName      HostExScsiNodeName `json:"nodeName"`
	InterfaceType string             `json:"interfaceType"`
}

type HostExScsiNodeName struct {
	IoInterfaceType string `json:"ioInterfaceType"`
	IscsiNodeName   string `json:"iscsiNodeName"`
	NvmeNodeName    string `json:"nvmeNodeName,omitempty"`
	RemoteNodeWWN   string `json:"remoteNodeWWN,omitempty"`
}

type HostGroupCreateRequest struct {
	Name  string   `json:"name"`
	Hosts []string `json:"hosts"`
}

type HostGroup struct {
	ClusterRef string `json:"clusterRef"`
	Label      string `json:"label"`
}

type HostPortUpdate struct {
	PortRef         string `json:"portRef,omitempty"`
	HostRef         string `json:"hostRef,omitempty"`
	Port            string `json:"port,omitempty"`
	Label           string `json:"label,omitempty"`
	IscsiChapSecret string `json:"iscsiChapSecret,omitempty"`
}

type HostUpdateRequest struct {
	Name          string           `json:"name,omitempty"`
	GroupID       string           `json:"groupId,omitempty"`
	Ports         []HostPort       `json:"ports,omitempty"`
	PortsToUpdate []HostPortUpdate `json:"portsToUpdate,omitempty"`
	PortsToRemove []string         `json:"portsToRemove,omitempty"`
	HostType      *HostType        `json:"hostType,omitempty"`
}

type VolumeMappingCreateRequest struct {
	MappableObjectID string `json:"mappableObjectId"`
	TargetID         string `json:"targetId"`
	LunNumber        int    `json:"lun,omitempty"`
}

type LUNMapping struct {
	LunMappingRef string `json:"lunMappingRef"`
	LunNumber     int    `json:"lun"`
	VolumeRef     string `json:"volumeRef"`
	MapRef        string `json:"mapRef"`
	Type          string `json:"type"`
}

// Used for errors on RESTful calls to return what went wrong
type CallResponseError struct {
	ErrorMsg     string `json:"errorMessage"`
	LocalizedMsg string `json:"localizedMessage"`
	ReturnCode   string `json:"retcode"`
	CodeType     string `json:"codeType"` //'symbol', 'webservice', 'systemerror', 'devicemgrerror'
}

type IscsiTargetSettings struct {
	TargetRef string `json:"targetRef"`
	NodeName  struct {
		IoInterfaceType string      `json:"ioInterfaceType"`
		IscsiNodeName   string      `json:"iscsiNodeName"`
		RemoteNodeWWN   interface{} `json:"remoteNodeWWN"`
	} `json:"nodeName"`
	Alias struct {
		IoInterfaceType string `json:"ioInterfaceType"`
		IscsiAlias      string `json:"iscsiAlias"`
	} `json:"alias"`
	ConfiguredAuthMethods struct {
		AuthMethodData []struct {
			AuthMethod string      `json:"authMethod"`
			ChapSecret interface{} `json:"chapSecret"`
		} `json:"authMethodData"`
	} `json:"configuredAuthMethods"`
	Portals []struct {
		GroupTag  int `json:"groupTag"`
		IPAddress struct {
			AddressType string      `json:"addressType"`
			Ipv4Address string      `json:"ipv4Address"`
			Ipv6Address interface{} `json:"ipv6Address"`
		} `json:"ipAddress"`
		TCPListenPort int `json:"tcpListenPort"`
	} `json:"portals"`
}

type NvmeofTargetSettings struct {
	TargetRef string `json:"targetRef"`
	NodeName  struct {
		IoInterfaceType string  `json:"ioInterfaceType"`
		NvmeNodeName    string  `json:"nvmeNodeName"`
		IscsiNodeName   *string `json:"iscsiNodeName"`
		RemoteNodeWWN   *string `json:"remoteNodeWWN"`
	} `json:"nodeName"`
	Alias struct {
		IoInterfaceType string  `json:"ioInterfaceType"`
		IscsiAlias      *string `json:"iscsiAlias"`
	} `json:"alias"`
	ConfiguredAuthMethods struct {
		AuthMethodData []interface{} `json:"authMethodData"`
	} `json:"configuredAuthMethods"`
	Portals []interface{} `json:"portals"`
}

type VolumeExpansionRequest struct {
	ExpansionSize string `json:"expansionSize"` // String format int64: "1073741824"
	SizeUnit      string `json:"sizeUnit"`      // "bytes"
}

// SnapshotGroup represents a PiT Group (Snapshot Group)
type SnapshotGroup struct {
	PitGroupRef       string `json:"pitGroupRef"`
	BaseVolume        string `json:"baseVolume"`
	Label             string `json:"label"`
	Status            string `json:"status"`
	SnapshotCount     int    `json:"snapshotCount"`
	FullWarnThreshold int    `json:"fullWarnThreshold"`
	AutoDeleteLimit   int    `json:"autoDeleteLimit"`
	RepFullPolicy     string `json:"repFullPolicy"`
	RollbackPriority  string `json:"rollbackPriority"`
}

// SnapshotImage represents a PiT (Point-in-Time Image/Snapshot)
// API definition name: "Snapshot"
type SnapshotImage struct {
	PitRef            string `json:"pitRef"`
	PitGroupRef       string `json:"pitGroupRef"`
	Status            string `json:"status"`
	PitTimestamp      string `json:"pitTimestamp"`
	PitSequenceNumber string `json:"pitSequenceNumber"`
	BaseVol           string `json:"baseVol"`
}

// SnapshotVolume represents a Linked Clone (Snapshot Volume)
// API definition name: "PitViewEx"
type SnapshotVolume struct {
	SnapshotRef string `json:"viewRef"` // The ID of the Snapshot Volume (linked clone)
	BaseVolume  string `json:"baseVol"` // The Base Volume it was created from
	BasePIT     string `json:"basePIT"` // The Snapshot Image (PiT) this volume accesses
	Label       string `json:"label"`
	Status      string `json:"status"`
}

// ConsistencyGroup represents a Consistency Group for snapshots (Snapshot Group container)
// API definition name: "PITConsistencyGroup"
type ConsistencyGroup struct {
	ConsistencyGroupRef string `json:"cgRef"`
	Label               string `json:"label"`
	RepFullPolicy       string `json:"repFullPolicy"`
	FullWarnThreshold   int    `json:"fullWarnThreshold"`
	AutoDeleteLimit     int    `json:"autoDeleteLimit"`
	RollbackPriority    string `json:"rollbackPriority,omitempty"`
}

// ConsistencyGroupMember represents a volume member of a Consistency Group
// API definition name: "PITCGMember"
type ConsistencyGroupMember struct {
	ConsistencyGroupId      string `json:"consistencyGroupId"`
	VolumeId                string `json:"volumeId"`
	VolumeWwn               string `json:"volumeWwn"`
	BaseVolumeName          string `json:"baseVolumeName"`
	RepositoryVolume        string `json:"repositoryVolume"`
	TotalRepositoryCapacity string `json:"totalRepositoryCapacity"`
	UsedRepositoryCapacity  string `json:"usedRepositoryCapacity"`
	AutoDeleteLimit         int    `json:"autoDeleteLimit"`
	FullWarnThreshold       int    `json:"fullWarnThreshold"`
}

// ConsistencyGroupView represents a Linked Clone (View) of a Consistency Group Snapshot
// API definition name: "PITConsistencyGroupView"
type ConsistencyGroupView struct {
	ConsistencyGroupViewRef string `json:"cgViewRef"`
	GroupRef                string `json:"groupRef"`
	Label                   string `json:"label"`
	ViewTime                string `json:"viewTime"`
	ViewSequenceNumber      string `json:"viewSequenceNumber"`
	Name                    string `json:"name"`
	Id                      string `json:"id"`
}

type Host struct {
	ClusterRef     string `json:"clusterRef"`
	HostRef        string `json:"hostRef"`
	Name           string `json:"name"`
	Label          string `json:"label"`
	HostTypeIndex  int    `json:"hostTypeIndex"`
	IsSAControlled bool   `json:"isSAControlled"`
}

type SnapshotRollbackRequest struct {
	PitRef []string `json:"pitRef"`
}

// ConcatRepositoryVolume represents a concatenated repository volume used by Snapshots and R/W Linked Clones
// API definition name: "ConcatRepositoryVolume"
type ConcatRepositoryVolume struct {
	ConcatVolRef      string   `json:"concatVolRef"`
	Status            string   `json:"status"`
	MemberCount       int      `json:"memberCount"`
	AggregateCapacity string   `json:"aggregateCapacity"`
	MemberRefs        []string `json:"memberRefs"`
	BaseObjectType    string   `json:"baseObjectType"`
	BaseObjectId      string   `json:"baseObjectId"`
	Name              string   `json:"name,omitempty"`
	MemberNames       string   `json:"memberNames,omitempty"`
}
