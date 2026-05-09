package grappadcim

type ServerItem struct {
	ID                    int     `json:"id"`
	Kind                  string  `json:"kind"`
	Name                  *string `json:"name,omitempty"`
	CustomerID            *int    `json:"customerId,omitempty"`
	Contact               *string `json:"contact,omitempty"`
	Status                *string `json:"status,omitempty"`
	OperatingSystem       *string `json:"operatingSystem,omitempty"`
	Architecture          *string `json:"architecture,omitempty"`
	Hostname              *string `json:"hostname,omitempty"`
	RackID                *int    `json:"rackId,omitempty"`
	RackName              *string `json:"rackName,omitempty"`
	Unit                  *int    `json:"unit,omitempty"`
	UnitPosition          *int    `json:"unitPosition,omitempty"`
	Slot                  *string `json:"slot,omitempty"`
	VirtualizationType    *string `json:"virtualizationType,omitempty"`
	VirtualizationCluster *string `json:"virtualizationCluster,omitempty"`
	Model                 *string `json:"model,omitempty"`
	Serial                *string `json:"serial,omitempty"`
	CPUSockets            *int    `json:"cpuSockets,omitempty"`
	CPU                   *string `json:"cpu,omitempty"`
	CoreCount             *int    `json:"coreCount,omitempty"`
	RAM                   *int    `json:"ram,omitempty"`
	RAMBanks              *string `json:"ramBanks,omitempty"`
	Disks                 *string `json:"disks,omitempty"`
	RaidLevel             *string `json:"raidLevel,omitempty"`
	Hotspare              *int    `json:"hotspare,omitempty"`
	IloAddress            *string `json:"iloAddress,omitempty"`
	PatchingManagement    *string `json:"patchingManagement,omitempty"`
	CustomerRootAccess    *string `json:"customerRootAccess,omitempty"`
	CustomerUsername      *string `json:"customerUsername,omitempty"`
	CdlanUsername         *string `json:"cdlanUsername,omitempty"`
	SyslogServer          *string `json:"syslogServer,omitempty"`
	SyslogServices        *string `json:"syslogServices,omitempty"`
	BackupHostname        *string `json:"backupHostname,omitempty"`
	BackupType            *string `json:"backupType,omitempty"`
	BackupNasServer       *string `json:"backupNasServer,omitempty"`
	BackupSchedule        *string `json:"backupSchedule,omitempty"`
	BackupQuotaGB         *int    `json:"backupQuotaGb,omitempty"`
	ActivatedAt           *string `json:"activatedAt,omitempty"`
	CeasedAt              *string `json:"ceasedAt,omitempty"`
	Note                  *string `json:"note,omitempty"`
	ManagementIP          *string `json:"managementIp,omitempty"`
	BackupNote            *string `json:"backupNote,omitempty"`
	ManagementNote        *string `json:"managementNote,omitempty"`
	EquipmentID           *int    `json:"equipmentId,omitempty"`
	EquipmentName         *string `json:"equipmentName,omitempty"`
	OrderCode             *string `json:"orderCode,omitempty"`
	SerialNumber          *string `json:"serialNumber,omitempty"`
	PortCount             *int    `json:"portCount,omitempty"`
}

type ServerInput struct {
	Kind                  string  `json:"kind"`
	Name                  *string `json:"name,omitempty"`
	CustomerID            *int    `json:"customerId,omitempty"`
	Contact               *string `json:"contact,omitempty"`
	Status                *string `json:"status,omitempty"`
	OperatingSystem       *string `json:"operatingSystem,omitempty"`
	Architecture          *string `json:"architecture,omitempty"`
	Hostname              *string `json:"hostname,omitempty"`
	RackID                *int    `json:"rackId,omitempty"`
	Unit                  *int    `json:"unit,omitempty"`
	UnitPosition          *int    `json:"unitPosition,omitempty"`
	Slot                  *string `json:"slot,omitempty"`
	VirtualizationType    *string `json:"virtualizationType,omitempty"`
	VirtualizationCluster *string `json:"virtualizationCluster,omitempty"`
	Model                 *string `json:"model,omitempty"`
	Serial                *string `json:"serial,omitempty"`
	CPUSockets            *int    `json:"cpuSockets,omitempty"`
	CPU                   *string `json:"cpu,omitempty"`
	CoreCount             *int    `json:"coreCount,omitempty"`
	RAM                   *int    `json:"ram,omitempty"`
	RAMBanks              *string `json:"ramBanks,omitempty"`
	Disks                 *string `json:"disks,omitempty"`
	RaidLevel             *string `json:"raidLevel,omitempty"`
	Hotspare              *int    `json:"hotspare,omitempty"`
	IloAddress            *string `json:"iloAddress,omitempty"`
	PatchingManagement    *string `json:"patchingManagement,omitempty"`
	CustomerRootAccess    *string `json:"customerRootAccess,omitempty"`
	CustomerUsername      *string `json:"customerUsername,omitempty"`
	CdlanUsername         *string `json:"cdlanUsername,omitempty"`
	SyslogServer          *string `json:"syslogServer,omitempty"`
	SyslogServices        *string `json:"syslogServices,omitempty"`
	BackupHostname        *string `json:"backupHostname,omitempty"`
	BackupType            *string `json:"backupType,omitempty"`
	BackupNasServer       *string `json:"backupNasServer,omitempty"`
	BackupSchedule        *string `json:"backupSchedule,omitempty"`
	BackupQuotaGB         *int    `json:"backupQuotaGb,omitempty"`
	ActivatedAt           *string `json:"activatedAt,omitempty"`
	Note                  *string `json:"note,omitempty"`
	ManagementIP          *string `json:"managementIp,omitempty"`
	BackupNote            *string `json:"backupNote,omitempty"`
	ManagementNote        *string `json:"managementNote,omitempty"`
	EquipmentID           *int    `json:"equipmentId,omitempty"`
	OrderCode             *string `json:"orderCode,omitempty"`
	SerialNumber          *string `json:"serialNumber,omitempty"`
	PortCount             *int    `json:"portCount,omitempty"`
}

type ServerPatch struct {
	Kind                  *string `json:"kind,omitempty"`
	Name                  *string `json:"name,omitempty"`
	CustomerID            *int    `json:"customerId,omitempty"`
	Contact               *string `json:"contact,omitempty"`
	Status                *string `json:"status,omitempty"`
	OperatingSystem       *string `json:"operatingSystem,omitempty"`
	Architecture          *string `json:"architecture,omitempty"`
	Hostname              *string `json:"hostname,omitempty"`
	RackID                *int    `json:"rackId,omitempty"`
	Unit                  *int    `json:"unit,omitempty"`
	UnitPosition          *int    `json:"unitPosition,omitempty"`
	Slot                  *string `json:"slot,omitempty"`
	VirtualizationType    *string `json:"virtualizationType,omitempty"`
	VirtualizationCluster *string `json:"virtualizationCluster,omitempty"`
	Model                 *string `json:"model,omitempty"`
	Serial                *string `json:"serial,omitempty"`
	CPUSockets            *int    `json:"cpuSockets,omitempty"`
	CPU                   *string `json:"cpu,omitempty"`
	CoreCount             *int    `json:"coreCount,omitempty"`
	RAM                   *int    `json:"ram,omitempty"`
	RAMBanks              *string `json:"ramBanks,omitempty"`
	Disks                 *string `json:"disks,omitempty"`
	RaidLevel             *string `json:"raidLevel,omitempty"`
	Hotspare              *int    `json:"hotspare,omitempty"`
	IloAddress            *string `json:"iloAddress,omitempty"`
	PatchingManagement    *string `json:"patchingManagement,omitempty"`
	CustomerRootAccess    *string `json:"customerRootAccess,omitempty"`
	CustomerUsername      *string `json:"customerUsername,omitempty"`
	CdlanUsername         *string `json:"cdlanUsername,omitempty"`
	SyslogServer          *string `json:"syslogServer,omitempty"`
	SyslogServices        *string `json:"syslogServices,omitempty"`
	BackupHostname        *string `json:"backupHostname,omitempty"`
	BackupType            *string `json:"backupType,omitempty"`
	BackupNasServer       *string `json:"backupNasServer,omitempty"`
	BackupSchedule        *string `json:"backupSchedule,omitempty"`
	BackupQuotaGB         *int    `json:"backupQuotaGb,omitempty"`
	ActivatedAt           *string `json:"activatedAt,omitempty"`
	Note                  *string `json:"note,omitempty"`
	ManagementIP          *string `json:"managementIp,omitempty"`
	BackupNote            *string `json:"backupNote,omitempty"`
	ManagementNote        *string `json:"managementNote,omitempty"`
	EquipmentID           *int    `json:"equipmentId,omitempty"`
	OrderCode             *string `json:"orderCode,omitempty"`
	SerialNumber          *string `json:"serialNumber,omitempty"`
	PortCount             *int    `json:"portCount,omitempty"`
}

type ServerChildren struct {
	Cards        []ServerCard        `json:"cards"`
	Applications []ServerApplication `json:"applications"`
	Services     []ServerService     `json:"services"`
	Ports        []ServerPort        `json:"ports"`
}

type ServerCard struct {
	ID           int     `json:"id"`
	PhysicalName *string `json:"physicalName,omitempty"`
	OSName       *string `json:"osName,omitempty"`
	IP           *string `json:"ip,omitempty"`
	SubnetmaskID *int    `json:"subnetmaskId,omitempty"`
	Note         *string `json:"note,omitempty"`
}

type ServerApplication struct {
	ID             int     `json:"id"`
	Name           *string `json:"name,omitempty"`
	ManagedByCdlan *string `json:"managedByCdlan,omitempty"`
}

type ServerService struct {
	ID   int     `json:"id"`
	Name *string `json:"name,omitempty"`
}

type ServerPort struct {
	ID                   int     `json:"id"`
	InterfaceName        *string `json:"interfaceName,omitempty"`
	DestinationInterface *string `json:"destinationInterface,omitempty"`
	PortType             *string `json:"portType,omitempty"`
}

type ServerCredentials struct {
	ServerID                   int     `json:"serverId"`
	IloAddress                 *string `json:"iloAddress,omitempty"`
	IloUsername                *string `json:"iloUsername,omitempty"`
	CustomerRootAccess         *string `json:"customerRootAccess,omitempty"`
	CustomerUsername           *string `json:"customerUsername,omitempty"`
	CdlanUsername              *string `json:"cdlanUsername,omitempty"`
	IloPasswordStored          bool    `json:"iloPasswordStored"`
	RootAdministratorStored    bool    `json:"rootAdministratorStored"`
	CustomerPasswordStored     bool    `json:"customerPasswordStored"`
	CdlanPasswordStored        bool    `json:"cdlanPasswordStored"`
	PasswordValueAccessEnabled bool    `json:"passwordValueAccessEnabled"`
	PasswordWriteAccessEnabled bool    `json:"passwordWriteAccessEnabled"`
}

type ServerCredentialsPatch struct {
	IloAddress         *string `json:"iloAddress,omitempty"`
	IloUsername        *string `json:"iloUsername,omitempty"`
	CustomerRootAccess *string `json:"customerRootAccess,omitempty"`
	CustomerUsername   *string `json:"customerUsername,omitempty"`
	CdlanUsername      *string `json:"cdlanUsername,omitempty"`
}
