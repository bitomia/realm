package cloudinit

type CloudInit struct {
	MetaData *MetaData `json:"meta_data,omitempty" yaml:"meta_data,omitempty"`
	UserData *UserData `json:"user_data,omitempty" yaml:"user_data,omitempty"`
}

type MetaData struct {
	InstanceID       string            `json:"instance-id" yaml:"instance-id"`
	LocalHostname    *string           `json:"local-hostname,omitempty" yaml:"local-hostname,omitempty"`
	Hostname         *string           `json:"hostname,omitempty" yaml:"hostname,omitempty"`
	FQDN             *string           `json:"fqdn,omitempty" yaml:"fqdn,omitempty"`
	AvailabilityZone *string           `json:"availability-zone,omitempty" yaml:"availability-zone,omitempty"`
	Region           *string           `json:"region,omitempty" yaml:"region,omitempty"`
	PlacementRegion  *string           `json:"placement-region,omitempty" yaml:"placement-region,omitempty"`
	CloudName        *string           `json:"cloud-name,omitempty" yaml:"cloud-name,omitempty"`
	CloudID          *string           `json:"cloud-id,omitempty" yaml:"cloud-id,omitempty"`
	Platform         *string           `json:"platform,omitempty" yaml:"platform,omitempty"`
	SubPlatform      *string           `json:"subplatform,omitempty" yaml:"subplatform,omitempty"`
	PublicKeys       map[string]string `json:"public-keys,omitempty" yaml:"public-keys,omitempty"`
	Network          *NetworkMetaData  `json:"network,omitempty" yaml:"network,omitempty"`
	Tags             map[string]string `json:"tags,omitempty" yaml:"tags,omitempty"`
}

type NetworkMetaData struct {
	Version *int                     `json:"version,omitempty" yaml:"version,omitempty"`
	Config  []map[string]interface{} `json:"config,omitempty" yaml:"config,omitempty"`
}

type UserData struct {
	Hostname                *string              `json:"hostname,omitempty" yaml:"hostname,omitempty"`
	FQDN                    *string              `json:"fqdn,omitempty" yaml:"fqdn,omitempty"`
	Locale                  *string              `json:"locale,omitempty" yaml:"locale,omitempty"`
	Timezone                *string              `json:"timezone,omitempty" yaml:"timezone,omitempty"`
	Users                   []User               `json:"users,omitempty" yaml:"users,omitempty"`
	Groups                  []Group              `json:"groups,omitempty" yaml:"groups,omitempty"`
	SSHAuthorizedKeys       []string             `json:"ssh_authorized_keys,omitempty" yaml:"ssh_authorized_keys,omitempty"`
	SSHKeys                 *SSHKeys             `json:"ssh_keys,omitempty" yaml:"ssh_keys,omitempty"`
	DisableRoot             *bool                `json:"disable_root,omitempty" yaml:"disable_root,omitempty"`
	SSHPwAuth               *bool                `json:"ssh_pwauth,omitempty" yaml:"ssh_pwauth,omitempty"`
	PackageUpdate           *bool                `json:"package_update,omitempty" yaml:"package_update,omitempty"`
	PackageUpgrade          *bool                `json:"package_upgrade,omitempty" yaml:"package_upgrade,omitempty"`
	PackageRebootIfRequired *bool                `json:"package_reboot_if_required,omitempty" yaml:"package_reboot_if_required,omitempty"`
	Packages                []string             `json:"packages,omitempty" yaml:"packages,omitempty"`
	WriteFiles              []WriteFile          `json:"write_files,omitempty" yaml:"write_files,omitempty"`
	BootCMD                 []interface{}        `json:"bootcmd,omitempty" yaml:"bootcmd,omitempty"`
	RunCMD                  []interface{}        `json:"runcmd,omitempty" yaml:"runcmd,omitempty"`
	FinalMessage            *string              `json:"final_message,omitempty" yaml:"final_message,omitempty"`
	Mounts                  [][]string           `json:"mounts,omitempty" yaml:"mounts,omitempty"`
	DiskSetup               map[string]DiskSetup `json:"disk_setup,omitempty" yaml:"disk_setup,omitempty"`
	FSSetup                 []FSSetup            `json:"fs_setup,omitempty" yaml:"fs_setup,omitempty"`
	ManageEtcHosts          *bool                `json:"manage_etc_hosts,omitempty" yaml:"manage_etc_hosts,omitempty"`
	PowerState              *PowerState          `json:"power_state,omitempty" yaml:"power_state,omitempty"`
	Chef                    *Chef                `json:"chef,omitempty" yaml:"chef,omitempty"`
	Puppet                  *Puppet              `json:"puppet,omitempty" yaml:"puppet,omitempty"`
	Chpasswd                *Chpasswd            `json:"chpasswd,omitempty" yaml:"chpasswd,omitempty"`
}

type Chpasswd struct {
	Expire *bool          `json:"expire,omitempty" yaml:"expire,omitempty"`
	Users  []ChpasswdUser `json:"users,omitempty" yaml:"users,omitempty"`
	List   interface{}    `json:"list,omitempty" yaml:"list,omitempty"`
}

type ChpasswdUser struct {
	Name     string  `json:"name" yaml:"name"`
	Password string  `json:"password" yaml:"password"`
	Type     *string `json:"type,omitempty" yaml:"type,omitempty"`
}

type User struct {
	Name              string   `json:"name" yaml:"name"`
	Gecos             *string  `json:"gecos,omitempty" yaml:"gecos,omitempty"`
	Groups            []string `json:"groups,omitempty" yaml:"groups,omitempty"`
	HomeDir           *string  `json:"homedir,omitempty" yaml:"homedir,omitempty"`
	Shell             *string  `json:"shell,omitempty" yaml:"shell,omitempty"`
	Sudo              *string  `json:"sudo,omitempty" yaml:"sudo,omitempty"`
	LockPasswd        *bool    `json:"lock_passwd,omitempty" yaml:"lock_passwd,omitempty"`
	Passwd            *string  `json:"passwd,omitempty" yaml:"passwd,omitempty"`
	SSHAuthorizedKeys []string `json:"ssh_authorized_keys,omitempty" yaml:"ssh_authorized_keys,omitempty"`
	SSHImportID       []string `json:"ssh_import_id,omitempty" yaml:"ssh_import_id,omitempty"`
	System            *bool    `json:"system,omitempty" yaml:"system,omitempty"`
	NoCreateHome      *bool    `json:"no_create_home,omitempty" yaml:"no_create_home,omitempty"`
	NoUserGroup       *bool    `json:"no_user_group,omitempty" yaml:"no_user_group,omitempty"`
	Inactive          *bool    `json:"inactive,omitempty" yaml:"inactive,omitempty"`
	Expiredate        *string  `json:"expiredate,omitempty" yaml:"expiredate,omitempty"`
	PrimaryGroup      *string  `json:"primary_group,omitempty" yaml:"primary_group,omitempty"`
	UID               *int     `json:"uid,omitempty" yaml:"uid,omitempty"`
}

type Group struct {
	Name    string   `json:"name" yaml:"name"`
	Members []string `json:"members,omitempty" yaml:"members,omitempty"`
}

type SSHKeys struct {
	RSAPrivate     *string `json:"rsa_private,omitempty" yaml:"rsa_private,omitempty"`
	RSAPublic      *string `json:"rsa_public,omitempty" yaml:"rsa_public,omitempty"`
	Ed25519Private *string `json:"ed25519_private,omitempty" yaml:"ed25519_private,omitempty"`
	Ed25519Public  *string `json:"ed25519_public,omitempty" yaml:"ed25519_public,omitempty"`
}

type WriteFile struct {
	Path        string  `json:"path" yaml:"path"`
	Content     *string `json:"content,omitempty" yaml:"content,omitempty"`
	Owner       *string `json:"owner,omitempty" yaml:"owner,omitempty"`
	Permissions *string `json:"permissions,omitempty" yaml:"permissions,omitempty"`
	Encoding    *string `json:"encoding,omitempty" yaml:"encoding,omitempty"`
	Append      *bool   `json:"append,omitempty" yaml:"append,omitempty"`
	Defer       *bool   `json:"defer,omitempty" yaml:"defer,omitempty"`
}

type DiskSetup struct {
	TableType *string     `json:"table_type,omitempty" yaml:"table_type,omitempty"`
	Layout    interface{} `json:"layout,omitempty" yaml:"layout,omitempty"` // bool or []int
	Overwrite *bool       `json:"overwrite,omitempty" yaml:"overwrite,omitempty"`
}
type FSSetup struct {
	Label      *string `json:"label,omitempty" yaml:"label,omitempty"`
	Filesystem *string `json:"filesystem,omitempty" yaml:"filesystem,omitempty"`
	Device     string  `json:"device" yaml:"device"`
	Partition  *string `json:"partition,omitempty" yaml:"partition,omitempty"`
	Overwrite  *bool   `json:"overwrite,omitempty" yaml:"overwrite,omitempty"`
	ReplaceFS  *string `json:"replace_fs,omitempty" yaml:"replace_fs,omitempty"`
}

type PowerState struct {
	Delay     *string `json:"delay,omitempty" yaml:"delay,omitempty"`
	Mode      *string `json:"mode,omitempty" yaml:"mode,omitempty"` // "reboot", "halt", "poweroff"
	Message   *string `json:"message,omitempty" yaml:"message,omitempty"`
	Timeout   *int    `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	Condition *string `json:"condition,omitempty" yaml:"condition,omitempty"`
}

type Chef struct {
	InstallType    *string  `json:"install_type,omitempty" yaml:"install_type,omitempty"`
	ForceInstall   *bool    `json:"force_install,omitempty" yaml:"force_install,omitempty"`
	ServerURL      *string  `json:"server_url,omitempty" yaml:"server_url,omitempty"`
	NodeName       *string  `json:"node_name,omitempty" yaml:"node_name,omitempty"`
	Environment    *string  `json:"environment,omitempty" yaml:"environment,omitempty"`
	ValidationName *string  `json:"validation_name,omitempty" yaml:"validation_name,omitempty"`
	ValidationKey  *string  `json:"validation_key,omitempty" yaml:"validation_key,omitempty"`
	RunList        []string `json:"run_list,omitempty" yaml:"run_list,omitempty"`
}

type Puppet struct {
	Install        *bool   `json:"install,omitempty" yaml:"install,omitempty"`
	Version        *string `json:"version,omitempty" yaml:"version,omitempty"`
	CollectionName *string `json:"collection,omitempty" yaml:"collection,omitempty"`
	AIOInstallURL  *string `json:"aio_install_url,omitempty" yaml:"aio_install_url,omitempty"`
	Cleanup        *bool   `json:"cleanup,omitempty" yaml:"cleanup,omitempty"`
	StartService   *bool   `json:"start_service,omitempty" yaml:"start_service,omitempty"`
	ServerName     *string `json:"server,omitempty" yaml:"server,omitempty"`
}
